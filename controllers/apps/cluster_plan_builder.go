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

package apps

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
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
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	roclient "github.com/apecloud/kubeblocks/pkg/controller/client"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	defaultWeight int = iota
	workloadWeight
	clusterWeight
)

// clusterTransformContext a graph.TransformContext implementation for Cluster reconciliation
type clusterTransformContext struct {
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
	transCtx     *clusterTransformContext
	transformers graph.TransformerChain
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *clusterTransformContext
}

var _ graph.TransformContext = &clusterTransformContext{}
var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

// TransformContext implementation

func (c *clusterTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *clusterTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *clusterTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *clusterTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

func init() {
	model.AddScheme(appsv1alpha1.AddToScheme)
	model.AddScheme(dpv1alpha1.AddToScheme)
	model.AddScheme(snapshotv1.AddToScheme)
	model.AddScheme(snapshotv1beta1.AddToScheme)
	model.AddScheme(extensionsv1alpha1.AddToScheme)
	model.AddScheme(workloadsv1alpha1.AddToScheme)
	model.AddScheme(storagev1alpha1.AddToScheme)
}

// PlanBuilder implementation

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.transCtx.Context, c.req.NamespacedName, cluster); err != nil {
		return err
	}
	c.AddTransformer(&initTransformer{cluster: cluster})
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
		if c.transCtx.Cluster.IsDeleting() {
			return
		}
		preCheckCondition := meta.FindStatusCondition(c.transCtx.Cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
		if preCheckCondition == nil {
			// this should not happen
			return
		}
		// if pre-check failed, this is a fast return, no need to set apply resource condition
		if preCheckCondition.Status != metav1.ConditionTrue {
			sendWarningEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonPreCheckFailed, err)
			return
		}
		setApplyResourceCondition(&c.transCtx.Cluster.Status.Conditions, c.transCtx.Cluster.Generation, err)
		sendWarningEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonApplyResourcesFailed, err)
	}()

	// new a DAG and apply chain on it
	dag := graph.NewDAG()
	err = c.transformers.ApplyTo(c.transCtx, dag)
	c.transCtx.Logger.V(1).Info(fmt.Sprintf("DAG: %s", dag))

	// construct execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFuncWithLogging,
		cli:      c.cli,
		transCtx: c.transCtx,
	}
	return plan, err
}

// Plan implementation

func (p *clusterPlan) Execute() error {
	less := func(v1, v2 graph.Vertex) bool {
		getWeight := func(v graph.Vertex) int {
			lifecycleVertex, ok := v.(*model.ObjectVertex)
			if !ok {
				return defaultWeight
			}
			switch lifecycleVertex.Obj.(type) {
			case *appsv1alpha1.Cluster:
				return clusterWeight
			case *appsv1.StatefulSet, *appsv1.Deployment:
				return workloadWeight
			default:
				return defaultWeight
			}
		}
		return getWeight(v1) <= getWeight(v2)
	}
	err := p.dag.WalkReverseTopoOrder(p.walkFunc, less)
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
	sendWarningEventWithError(p.transCtx.GetRecorder(), clusterCopy, ReasonApplyResourcesFailed, err)
	return p.cli.Status().Patch(p.transCtx.Context, clusterCopy, client.MergeFrom(p.transCtx.OrigCluster))
}

// Do the real works

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &clusterPlanBuilder{
		req: req,
		cli: cli,
		transCtx: &clusterTransformContext{
			Context:       ctx.Ctx,
			Client:        model.NewGraphClient(cli),
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

func (c *clusterPlanBuilder) defaultWalkFuncWithLogging(vertex graph.Vertex) error {
	node, ok := vertex.(*model.ObjectVertex)
	err := c.defaultWalkFunc(vertex)
	switch {
	case err == nil:
		return err
	case !ok:
		c.transCtx.Logger.Error(err, "")
	case node.Action == nil:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%T", node))
	case apierrors.IsConflict(err):
		return err
	default:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%s %T error", *node.Action, node.Obj))
	}
	return err
}

func (c *clusterPlanBuilder) defaultWalkFunc(vertex graph.Vertex) error {
	node, ok := vertex.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", vertex)
	}
	if node.Action == nil {
		return errors.New("node action can't be nil")
	}

	// cluster object has more business to do, handle them here
	if _, ok = node.Obj.(*appsv1alpha1.Cluster); ok {
		if err := c.reconcileCluster(node); err != nil {
			return err
		}
	}
	return c.reconcileObject(node)
}

func (c *clusterPlanBuilder) reconcileObject(node *model.ObjectVertex) error {
	switch *node.Action {
	case model.CREATE:
		err := c.cli.Create(c.transCtx.Context, node.Obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case model.UPDATE:
		err := c.cli.Update(c.transCtx.Context, node.Obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case model.PATCH:
		patch := client.MergeFrom(node.OriObj)
		if err := c.cli.Patch(c.transCtx.Context, node.Obj, patch); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case model.DELETE:
		if controllerutil.RemoveFinalizer(node.Obj, constant.DBClusterFinalizerName) {
			err := c.cli.Update(c.transCtx.Context, node.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		// delete secondary objects
		if _, ok := node.Obj.(*appsv1alpha1.Cluster); !ok {
			err := intctrlutil.BackgroundDeleteObject(c.cli, c.transCtx.Context, node.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case model.STATUS:
		patch := client.MergeFrom(node.OriObj)
		if err := c.cli.Status().Patch(c.transCtx.Context, node.Obj, patch); err != nil {
			return err
		}
		// handle condition and phase changing triggered events
		if newCluster, ok := node.Obj.(*appsv1alpha1.Cluster); ok {
			oldCluster, _ := node.OriObj.(*appsv1alpha1.Cluster)
			c.emitConditionUpdatingEvent(oldCluster.Status.Conditions, newCluster.Status.Conditions)
			c.emitStatusUpdatingEvent(oldCluster.Status, newCluster.Status)
		}
	case model.NOOP:
		// nothing
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileCluster(node *model.ObjectVertex) error {
	cluster := node.Obj.(*appsv1alpha1.Cluster).DeepCopy()
	origCluster := node.OriObj.(*appsv1alpha1.Cluster)
	switch *node.Action {
	// cluster.meta and cluster.spec might change
	case model.STATUS:
		if !reflect.DeepEqual(cluster.ObjectMeta, origCluster.ObjectMeta) || !reflect.DeepEqual(cluster.Spec, origCluster.Spec) {
			patch := client.MergeFrom(origCluster.DeepCopy())
			if err := c.cli.Patch(c.transCtx.Context, cluster, patch); err != nil {
				return err
			}
		}
	case model.CREATE, model.UPDATE:
		return fmt.Errorf("cluster can't be created or updated: %s", cluster.Name)
	}
	return nil
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

func (c *clusterPlanBuilder) emitStatusUpdatingEvent(oldStatus, newStatus appsv1alpha1.ClusterStatus) {
	cluster := c.transCtx.Cluster
	newPhase := newStatus.Phase
	if newPhase == oldStatus.Phase {
		return
	}
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
	}
}
