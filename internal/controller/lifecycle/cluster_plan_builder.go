/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO: cluster plan builder can be abstracted as a common flow

// ClusterTransformContext a graph.TransformContext implementation for Cluster reconciliation
type ClusterTransformContext struct {
	context.Context
	Client roclient.ReadonlyClient
	record.EventRecorder
	logr.Logger
	Cluster     *appsv1alpha1.Cluster
	OrigCluster *appsv1alpha1.Cluster
	ClusterDef  *appsv1alpha1.ClusterDefinition
	ClusterVer  *appsv1alpha1.ClusterVersion
}

// clusterPlanBuilder a graph.PlanBuilder implementation for Cluster reconciliation
type clusterPlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *ClusterTransformContext
	transformers graph.TransformerChain
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *ClusterTransformContext
}

var _ graph.TransformContext = &ClusterTransformContext{}
var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

// TransformContext implementation

func (c *ClusterTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *ClusterTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *ClusterTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *ClusterTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

// PlanBuilder implementation

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.transCtx.Context, c.req.NamespacedName, cluster); err != nil {
		return err
	}

	c.transCtx.Cluster = cluster
	c.transCtx.OrigCluster = cluster.DeepCopy()
	c.transformers = append(c.transformers, &initTransformer{
		cluster:       c.transCtx.Cluster,
		originCluster: c.transCtx.OrigCluster,
	})
	return nil
}

func (c *clusterPlanBuilder) AddTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, transformer...)
	return c
}

func (c *clusterPlanBuilder) AddParallelTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, &ParallelTransformers{transformers: transformer})
	return c
}

// Build runs all transformers to generate a plan
func (c *clusterPlanBuilder) Build() (graph.Plan, error) {
	var err error
	defer func() {
		// set apply resource condition
		// if cluster is being deleted, no need to set apply resource condition
		if isClusterDeleting(*c.transCtx.Cluster) {
			return
		}
		preCheckCondition := meta.FindStatusCondition(c.transCtx.Cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
		if preCheckCondition == nil {
			// this should not happen
			return
		}
		// if pre-check failed, this is a fast return, no need to set apply resource condition
		if preCheckCondition.Status != metav1.ConditionTrue {
			sendWaringEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonPreCheckFailed, err)
			return
		}
		setApplyResourceCondition(&c.transCtx.Cluster.Status.Conditions, c.transCtx.Cluster.Generation, err)
		sendWaringEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonApplyResourcesFailed, err)
	}()

	// new a DAG and apply chain on it
	dag := graph.NewDAG()
	err = c.transformers.ApplyTo(c.transCtx, dag)
	c.transCtx.Logger.V(1).Info(fmt.Sprintf("DAG: %s", dag))

	// construct execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFunc,
		cli:      c.cli,
		transCtx: c.transCtx,
	}
	return plan, err
}

// Plan implementation

func (p *clusterPlan) Execute() error {
	err := p.dag.WalkReverseTopoOrder(p.walkFunc)
	if err != nil {
		if hErr := p.handlePlanExecutionError(err); hErr != nil {
			return hErr
		}
	}
	return err
}

func (p *clusterPlan) handlePlanExecutionError(err error) error {
	clusterCopy := p.transCtx.OrigCluster.DeepCopy()
	condition := newFailedApplyResourcesCondition(err)
	meta.SetStatusCondition(&clusterCopy.Status.Conditions, condition)
	sendWaringEventWithError(p.transCtx.GetRecorder(), clusterCopy, ReasonApplyResourcesFailed, err)
	return p.cli.Status().Patch(p.transCtx.Context, clusterCopy, client.MergeFrom(p.transCtx.OrigCluster))
}

// Do the real works

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &clusterPlanBuilder{
		req: req,
		cli: cli,
		transCtx: &ClusterTransformContext{
			Context:       ctx.Ctx,
			Client:        cli,
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

// TODO: retry strategy on error
func (c *clusterPlanBuilder) defaultWalkFunc(vertex graph.Vertex) error {
	node, ok := vertex.(*lifecycleVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", vertex)
	}
	if node.action == nil {
		return errors.New("node action can't be nil")
	}
	// cluster object has more business to do, handle them here
	if _, ok := node.obj.(*appsv1alpha1.Cluster); ok {
		cluster := node.obj.(*appsv1alpha1.Cluster).DeepCopy()
		origCluster := node.oriObj.(*appsv1alpha1.Cluster)
		switch *node.action {
		// cluster.meta and cluster.spec might change
		case STATUS:
			if !reflect.DeepEqual(cluster.ObjectMeta, origCluster.ObjectMeta) ||
				!reflect.DeepEqual(cluster.Spec, origCluster.Spec) {
				// TODO: we should Update instead of Patch cluster object,
				// TODO: but Update failure happens too frequently as other controllers are updating cluster object too.
				// TODO: use Patch here, revert to Update after refactoring done
				// if err := c.cli.Update(c.ctx.Ctx, cluster); err != nil {
				//	tmpCluster := &appsv1alpha1.Cluster{}
				//	err = c.cli.Get(c.ctx.Ctx,client.ObjectKeyFromObject(origCluster), tmpCluster)
				//	c.ctx.Log.Error(err, fmt.Sprintf("update %T error, orig: %v, curr: %v, api-server: %v", origCluster, origCluster, cluster, tmpCluster))
				//	return err
				// }
				patch := client.MergeFrom(origCluster.DeepCopy())
				if err := c.cli.Patch(c.transCtx.Context, cluster, patch); err != nil {
					// log for debug
					// TODO:(free6om) make error message smaller when refactor done.
					c.transCtx.Logger.Error(err, fmt.Sprintf("patch %T error, orig: %v, curr: %v", origCluster, origCluster, cluster))
					return err
				}
			}
		case CREATE, UPDATE:
			return fmt.Errorf("cluster can't be created or updated: %s", cluster.Name)
		}
	}
	switch *node.action {
	case CREATE:
		err := c.cli.Create(c.transCtx.Context, node.obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case UPDATE:
		if node.immutable {
			return nil
		}
		o, err := c.buildUpdateObj(node)
		if err != nil {
			return err
		}
		err = c.cli.Update(c.transCtx.Context, o)
		if err != nil && !apierrors.IsNotFound(err) {
			c.transCtx.Logger.Error(err, fmt.Sprintf("update %T error: %s", o, node.oriObj.GetName()))
			return err
		}
	case DELETE:
		if controllerutil.RemoveFinalizer(node.obj, dbClusterFinalizerName) {
			err := c.cli.Update(c.transCtx.Context, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				c.transCtx.Logger.Error(err, fmt.Sprintf("delete %T error: %s", node.obj, node.obj.GetName()))
				return err
			}
		}
		// delete secondary objects
		if _, ok := node.obj.(*appsv1alpha1.Cluster); !ok {
			// retain backup for data protection even if the cluster is wiped out.
			if strings.EqualFold(node.obj.GetLabels()[constant.BackupProtectionLabelKey], constant.BackupRetain) {
				return nil
			}
			err := intctrlutil.BackgroundDeleteObject(c.cli, c.transCtx.Context, node.obj)
			// err := c.cli.Delete(c.transCtx.Context, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case STATUS:
		patch := client.MergeFrom(node.oriObj)
		if err := c.cli.Status().Patch(c.transCtx.Context, node.obj, patch); err != nil {
			return err
		}
		// handle condition and phase changing triggered events
		if newCluster, ok := node.obj.(*appsv1alpha1.Cluster); ok {
			oldCluster, _ := node.oriObj.(*appsv1alpha1.Cluster)
			c.emitConditionUpdatingEvent(oldCluster.Status.Conditions, newCluster.Status.Conditions)
			c.emitPhaseUpdatingEvent(oldCluster.Status.Phase, newCluster.Status.Phase)
		}
	}
	return nil
}

func (c *clusterPlanBuilder) buildUpdateObj(node *lifecycleVertex) (client.Object, error) {
	handleSts := func(origObj, stsProto *appsv1.StatefulSet) (client.Object, error) {
		stsObj := origObj.DeepCopy()
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
			c.transCtx.EventRecorder.Eventf(c.transCtx.Cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"Start horizontal scale component %s from %d to %d",
				componentName,
				*stsObj.Spec.Replicas,
				*stsProto.Spec.Replicas)
		}
		// keep the original template annotations.
		// if annotations exist and are replaced, the statefulSet will be updated.
		mergeAnnotations(stsObj.Spec.Template.Annotations,
			&stsProto.Spec.Template.Annotations)
		stsObj.Spec.Template = stsProto.Spec.Template
		stsObj.Spec.Replicas = stsProto.Spec.Replicas
		stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
		return stsObj, nil
	}

	handleDeploy := func(origObj, deployProto *appsv1.Deployment) (client.Object, error) {
		deployObj := origObj.DeepCopy()
		mergeAnnotations(deployObj.Spec.Template.Annotations,
			&deployProto.Spec.Template.Annotations)
		deployObj.Spec = deployProto.Spec
		return deployObj, nil
	}

	handleSvc := func(origObj, svcProto *corev1.Service) (client.Object, error) {
		svcObj := origObj.DeepCopy()
		svcObj.Spec = svcProto.Spec
		svcObj.Annotations = mergeServiceAnnotations(svcObj.Annotations, svcProto.Annotations)
		return svcObj, nil
	}

	handlePVC := func(origObj, pvcProto *corev1.PersistentVolumeClaim) (client.Object, error) {
		pvcObj := origObj.DeepCopy()
		if pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] == pvcProto.Spec.Resources.Requests[corev1.ResourceStorage] {
			return pvcObj, nil
		}
		pvcObj.Spec.Resources.Requests[corev1.ResourceStorage] = pvcProto.Spec.Resources.Requests[corev1.ResourceStorage]
		return pvcObj, nil
	}

	switch v := node.obj.(type) {
	case *appsv1.StatefulSet:
		return handleSts(node.oriObj.(*appsv1.StatefulSet), v)
	case *appsv1.Deployment:
		return handleDeploy(node.oriObj.(*appsv1.Deployment), v)
	case *corev1.Service:
		return handleSvc(node.oriObj.(*corev1.Service), v)
	case *corev1.PersistentVolumeClaim:
		return handlePVC(node.oriObj.(*corev1.PersistentVolumeClaim), v)
	case *corev1.Secret, *corev1.ConfigMap:
		return v, nil
	}

	return node.obj, nil
}

func (c *clusterPlanBuilder) emitConditionUpdatingEvent(oldConditions, newConditions []metav1.Condition) {
	for _, newCondition := range newConditions {
		oldCondition := meta.FindStatusCondition(oldConditions, newCondition.Type)
		// filtered in cluster creation
		if oldCondition == nil && newCondition.Status == metav1.ConditionFalse {
			return
		}
		if !reflect.DeepEqual(oldCondition, &newCondition) {
			eType := corev1.EventTypeNormal
			if newCondition.Status == metav1.ConditionFalse {
				eType = corev1.EventTypeWarning
			}
			c.transCtx.EventRecorder.Event(c.transCtx.Cluster, eType, newCondition.Reason, newCondition.Message)
		}
	}
}

func (c *clusterPlanBuilder) emitPhaseUpdatingEvent(oldPhase, newPhase appsv1alpha1.ClusterPhase) {
	if oldPhase == newPhase {
		return
	}

	cluster := c.transCtx.Cluster
	eType := corev1.EventTypeNormal
	message := ""
	switch newPhase {
	case appsv1alpha1.RunningClusterPhase:
		message = fmt.Sprintf("Cluster: %s is ready, current phase is %s", cluster.Name, newPhase)
	case appsv1alpha1.StoppedClusterPhase:
		message = fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
	case appsv1alpha1.FailedClusterPhase, appsv1alpha1.AbnormalClusterPhase:
		message = fmt.Sprintf("Cluster: %s is %s, check according to the components message", cluster.Name, newPhase)
		eType = corev1.EventTypeWarning
	}
	if len(message) > 0 {
		c.transCtx.EventRecorder.Event(cluster, eType, string(newPhase), message)
		_ = opsutil.MarkRunningOpsRequestAnnotation(c.transCtx.Context, c.cli, cluster)
	}
}
