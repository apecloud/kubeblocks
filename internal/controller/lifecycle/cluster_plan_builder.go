/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
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
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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

// Build only cluster Creation, Update and Deletion supported.
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
			return
		}
		setApplyResourceCondition(&c.transCtx.Cluster.Status.Conditions, c.transCtx.Cluster.Generation, err)
	}()

	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	if err = c.transformers.ApplyTo(c.transCtx, dag); err != nil {
		return nil, err
	}
	c.transCtx.Logger.Info(fmt.Sprintf("DAG: %s", dag))

	// we got the execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFunc,
		cli:      c.cli,
		transCtx: c.transCtx,
	}
	return plan, nil
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
	condition := newFailedApplyResourcesCondition(err.Error())
	meta.SetStatusCondition(&p.transCtx.Cluster.Status.Conditions, condition)
	p.transCtx.EventRecorder.Event(p.transCtx.Cluster, corev1.EventTypeWarning, condition.Reason, condition.Message)
	return p.cli.Status().Patch(p.transCtx.Context, p.transCtx.Cluster, client.MergeFrom(p.transCtx.OrigCluster.DeepCopy()))
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
	updateComponentPhaseIfNeeded := func(orig, curr client.Object) {
		switch orig.(type) {
		case *appsv1.StatefulSet, *appsv1.Deployment:
			componentName := orig.GetLabels()[constant.KBAppComponentLabelKey]
			origSpec := reflect.ValueOf(orig).Elem().FieldByName("Spec").Interface()
			newSpec := reflect.ValueOf(curr).Elem().FieldByName("Spec").Interface()
			if !reflect.DeepEqual(origSpec, newSpec) {
				// sync component phase
				updateComponentPhaseWithOperation(c.transCtx.Cluster, componentName)
			}
		}
	}
	// cluster object has more business to do, handle them here
	if _, ok := node.obj.(*appsv1alpha1.Cluster); ok {
		cluster := node.obj.(*appsv1alpha1.Cluster).DeepCopy()
		origCluster := node.oriObj.(*appsv1alpha1.Cluster)
		switch *node.action {
		// cluster.meta and cluster.spec might change
		case CREATE, UPDATE, STATUS:
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
					c.transCtx.Logger.Error(err, fmt.Sprintf("patch %T error, orig: %v, curr: %v", origCluster, origCluster, cluster))
					return err
				}
			}
		case DELETE:
			if err := c.handleClusterDeletion(cluster); err != nil {
				return err
			}
			if cluster.Spec.TerminationPolicy == appsv1alpha1.DoNotTerminate {
				return nil
			}
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
			c.transCtx.Logger.Error(err, fmt.Sprintf("update %T error, orig: %v, curr: %v", o, node.oriObj, o))
			return err
		}
		// TODO: find a better comparison way that knows whether fields are updated before calling the Update func
		updateComponentPhaseIfNeeded(node.oriObj, o)
	case DELETE:
		if controllerutil.RemoveFinalizer(node.obj, dbClusterFinalizerName) {
			err := c.cli.Update(c.transCtx.Context, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				c.transCtx.Logger.Error(err, fmt.Sprintf("delete %T error, orig: %v, curr: %v", node.obj, node.oriObj, node.obj))
				return err
			}
		}
		if node.isOrphan {
			err := c.cli.Delete(c.transCtx.Context, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		// TODO: delete backup objects created in scale-out
		// TODO: should manage backup objects in a better way
		if isTypeOf[*snapshotv1.VolumeSnapshot](node.obj) ||
			isTypeOf[*dataprotectionv1alpha1.BackupPolicy](node.obj) ||
			isTypeOf[*dataprotectionv1alpha1.Backup](node.obj) {
			_ = c.cli.Delete(c.transCtx.Context, node.obj)
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

func (c *clusterPlanBuilder) handleClusterDeletion(cluster *appsv1alpha1.Cluster) error {
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		c.transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate", "spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return nil
	case appsv1alpha1.Delete, appsv1alpha1.WipeOut:
		if err := c.deletePVCs(cluster); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err := c.deleteConfigMaps(cluster); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		// The backup policy must be cleaned up when the cluster is deleted.
		// Automatic backup scheduling needs to be stopped at this point.
		if err := c.deleteBackupPolicies(cluster); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if cluster.Spec.TerminationPolicy == appsv1alpha1.WipeOut {
			// TODO check whether delete backups together with cluster is allowed
			// wipe out all backups
			if err := c.deleteBackups(cluster); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (c *clusterPlanBuilder) deletePVCs(cluster *appsv1alpha1.Cluster) error {
	// it's possible at time of external resource deletion, cluster definition has already been deleted.
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := c.cli.List(c.transCtx.Context, pvcList, inNS, ml); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if err := c.cli.Delete(c.transCtx.Context, &pvc); err != nil {
			return err
		}
	}
	return nil
}

func (c *clusterPlanBuilder) deleteConfigMaps(cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:  cluster.GetName(),
		constant.AppManagedByLabelKey: constant.AppName,
	}
	return c.cli.DeleteAllOf(c.transCtx.Context, &corev1.ConfigMap{}, inNS, ml)
}

func (c *clusterPlanBuilder) deleteBackupPolicies(cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	return c.cli.DeleteAllOf(c.transCtx.Context, &dataprotectionv1alpha1.BackupPolicy{}, inNS, ml)
}

func (c *clusterPlanBuilder) deleteBackups(cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backups
	backups := &dataprotectionv1alpha1.BackupList{}
	if err := c.cli.List(c.transCtx.Context, backups, inNS, ml); err != nil {
		return err
	}
	for _, backup := range backups.Items {
		// check backup delete protection label
		deleteProtection, exists := backup.GetLabels()[constant.BackupProtectionLabelKey]
		// not found backup-protection or value is Delete, delete it.
		if !exists || deleteProtection == constant.BackupDelete {
			if err := c.cli.Delete(c.transCtx.Context, &backup); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *clusterPlanBuilder) emitConditionUpdatingEvent(oldConditions, newConditions []metav1.Condition) {
	for _, newCondition := range newConditions {
		oldCondition := meta.FindStatusCondition(oldConditions, newCondition.Type)
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
