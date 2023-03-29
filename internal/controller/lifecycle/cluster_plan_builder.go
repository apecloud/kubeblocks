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
	"errors"
	"fmt"
	"reflect"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// clusterPlanBuilder a graph.PlanBuilder implementation for Cluster reconciliation
type clusterPlanBuilder struct {
	ctx      intctrlutil.RequestCtx
	cli      client.Client
	req      ctrl.Request
	recorder record.EventRecorder
	cluster  *appsv1alpha1.Cluster
	conMgr   clusterConditionManager2
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cluster  *appsv1alpha1.Cluster
	conMgr   clusterConditionManager2
}

var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.ctx.Ctx, c.req.NamespacedName, cluster); err != nil {
		return err
	}
	c.cluster = cluster
	return nil
}

func (c *clusterPlanBuilder) Validate() error {
	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err := c.cli.Get(c.ctx.Ctx, key, object)
		if err != nil {
			if apierrors.IsNotFound(err) {
				if setErr := c.conMgr.setPreCheckErrorCondition(c.cluster, err); util.IgnoreNoOps(setErr) != nil {
					return setErr
				}
				c.recorder.Eventf(c.cluster, corev1.EventTypeWarning, constant.ReasonNotFoundCR, err.Error())
			}
			return newRequeueError(requeueDuration, err.Error())
		}
		return nil
	}

	// validate cd & cv existences
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := validateExistence(types.NamespacedName{Name: c.cluster.Spec.ClusterDefRef}, cd); err != nil {
		return err
	}
	var cv *appsv1alpha1.ClusterVersion
	if len(c.cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err := validateExistence(types.NamespacedName{Name: c.cluster.Spec.ClusterVersionRef}, cv); err != nil {
			return err
		}
	}

	// validate cd & cv availability
	if cd.Status.Phase != appsv1alpha1.AvailablePhase || (cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase) {
		message := fmt.Sprintf("ref resource is unavailable, this problem needs to be solved first. cd: %v, cv: %v", cd, cv)
		if err := c.conMgr.setReferenceCRUnavailableCondition(c.cluster, message); util.IgnoreNoOps(err) != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
		}
		return newRequeueError(requeueDuration, message)
	}

	// validate logs
	// and a sample validator chain
	chain := &graph.ValidatorChain{
		&enableLogsValidator{cluster: c.cluster, clusterDef: cd},
	}
	if err := chain.WalkThrough(); err != nil {
		_ = c.conMgr.setPreCheckErrorCondition(c.cluster, err)
		return newRequeueError(requeueDuration, err.Error())
	}

	return nil
}

// Build only cluster Creation, Update and Deletion supported.
func (c *clusterPlanBuilder) Build() (graph.Plan, error) {
	_ = c.conMgr.setProvisioningStartedCondition(c.cluster)
	var err error
	defer func() {
		if err != nil {
			_ = c.conMgr.setApplyResourcesFailedCondition(c.cluster, err)
		}
	}()

	var cr *clusterRefResources
	cr, err = c.getClusterRefResources()
	if err != nil {
		return nil, err
	}
	var roClient types2.ReadonlyClient = delegateClient{Client: c.cli}

	// TODO: remove all cli & ctx fields from transformers, keep them in pure-dag-manipulation form
	// build transformer chain
	chain := &graph.TransformerChain{
		// init dag, that is put cluster vertex into dag
		&initTransformer{cluster: c.cluster},
		// fix cd&cv labels of cluster
		&fixClusterLabelsTransformer{},
		// cluster to K8s objects and put them into dag
		&clusterTransformer{cc: *cr, cli: c.cli, ctx: c.ctx},
		// tls certs secret
		&tlsCertsTransformer{cr: *cr, cli: roClient, ctx: c.ctx},
		// add our finalizer to all objects
		&ownershipTransformer{finalizer: dbClusterFinalizerName},
		// make all workload objects depending on credential secret
		&credentialTransformer{},
		// make config configmap immutable
		&configTransformer{},
		// read old snapshot from cache, and generate diff plan
		&objectActionTransformer{cli: roClient, ctx: c.ctx},
		// handle TerminationPolicyType=DoNotTerminate
		&doNotTerminateTransformer{},
		// horizontal scaling
		&stsHorizontalScalingTransformer{cr: *cr, cli: roClient, ctx: c.ctx},
		// stateful set pvc Update
		&stsPVCTransformer{cli: c.cli, ctx: c.ctx},
		// replication set horizontal scaling
		&rplSetHorizontalScalingTransformer{cr: *cr, cli: c.cli, ctx: c.ctx},
		// finally, update cluster status
		&clusterStatusTransformer{cc: *cr, cli: c.cli, ctx: c.ctx, recorder: c.recorder, conMgr: c.conMgr},
	}

	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	if err = chain.ApplyTo(dag); err != nil {
		return nil, err
	}

	c.ctx.Log.Info(fmt.Sprintf("DAG: %s", dag))
	// we got the execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFunc,
		cluster:  c.cluster,
		conMgr:   c.conMgr,
	}
	return plan, nil
}

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
// TODO: change ctx to context.Context
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request, recorder record.EventRecorder) graph.PlanBuilder {
	conMgr := clusterConditionManager2{
		Client:   cli,
		Recorder: recorder,
		ctx:      ctx.Ctx,
	}
	return &clusterPlanBuilder{
		ctx:      ctx,
		cli:      cli,
		req:      req,
		recorder: recorder,
		conMgr:   conMgr,
	}
}

func (p *clusterPlan) Execute() error {
	err := p.dag.WalkReverseTopoOrder(p.walkFunc)
	if err != nil {
		_ = p.conMgr.setApplyResourcesFailedCondition(p.cluster, err)
	}
	return err
}

func (c *clusterPlanBuilder) getClusterRefResources() (*clusterRefResources, error) {
	cluster := c.cluster
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := c.cli.Get(c.ctx.Ctx, types.NamespacedName{
		Name: cluster.Spec.ClusterDefRef,
	}, cd); err != nil {
		return nil, err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		if err := c.cli.Get(c.ctx.Ctx, types.NamespacedName{
			Name: cluster.Spec.ClusterVersionRef,
		}, cv); err != nil {
			return nil, err
		}
	}

	cc := &clusterRefResources{
		cd: *cd,
		cv: *cv,
	}
	return cc, nil
}

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
				updateComponentPhaseWithOperation(c.cluster, componentName)
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
				if err := c.cli.Patch(c.ctx.Ctx, cluster, patch); err != nil {
					c.ctx.Log.Error(err, fmt.Sprintf("patch %T error, orig: %v, curr: %v", origCluster, origCluster, cluster))
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
		err := c.cli.Create(c.ctx.Ctx, node.obj)
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
		err = c.cli.Update(c.ctx.Ctx, o)
		if err != nil && !apierrors.IsNotFound(err) {
			c.ctx.Log.Error(err, fmt.Sprintf("update %T error, orig: %v, curr: %v", o, node.oriObj, o))
			return err
		}
		// TODO: find a better comparison way that knows whether fields are updated before calling the Update func
		updateComponentPhaseIfNeeded(node.oriObj, o)
	case DELETE:
		if controllerutil.RemoveFinalizer(node.obj, dbClusterFinalizerName) {
			err := c.cli.Update(c.ctx.Ctx, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				c.ctx.Log.Error(err, fmt.Sprintf("delete %T error, orig: %v, curr: %v", node.obj, node.oriObj, node.obj))
				return err
			}
		}
		if node.isOrphan {
			err := c.cli.Delete(c.ctx.Ctx, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		// TODO: delete backup objects created in scale-out
		// TODO: should manage backup objects in a better way
		if isTypeOf[*snapshotv1.VolumeSnapshot](node.obj) ||
			isTypeOf[*dataprotectionv1alpha1.BackupPolicy](node.obj) ||
			isTypeOf[*dataprotectionv1alpha1.Backup](node.obj) {
			_ = c.cli.Delete(c.ctx.Ctx, node.obj)
		}

	case STATUS:
		patch := client.MergeFrom(node.oriObj)
		return c.cli.Status().Patch(c.ctx.Ctx, node.obj, patch)
	}
	return nil
}

func (c *clusterPlanBuilder) buildUpdateObj(node *lifecycleVertex) (client.Object, error) {
	handleSts := func(origObj, stsProto *appsv1.StatefulSet) (client.Object, error) {
		stsObj := origObj.DeepCopy()
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
			c.recorder.Eventf(c.cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"Start horizontal scale component %s from %d to %d",
				componentName,
				*stsObj.Spec.Replicas,
				*stsProto.Spec.Replicas)
		}
		// keep the original template annotations.
		// if annotations exist and are replaced, the statefulSet will be updated.
		stsProto.Spec.Template.Annotations = mergeAnnotations(stsObj.Spec.Template.Annotations,
			stsProto.Spec.Template.Annotations)
		stsObj.Spec.Template = stsProto.Spec.Template
		stsObj.Spec.Replicas = stsProto.Spec.Replicas
		stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
		return stsObj, nil
	}

	handleDeploy := func(origObj, deployProto *appsv1.Deployment) (client.Object, error) {
		deployObj := origObj.DeepCopy()
		deployProto.Spec.Template.Annotations = mergeAnnotations(deployObj.Spec.Template.Annotations,
			deployProto.Spec.Template.Annotations)
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
		// if cluster.Status.Phase != appsv1alpha1.DeletingClusterPhase {
		// 	patch := client.MergeFrom(cluster.DeepCopy())
		// 	cluster.Status.ObservedGeneration = cluster.Generation
		// 	// cluster.Status.Message = fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		// 	if err := r.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		// 		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		// 	}
		// }
		// TODO: add warning event
		return nil
	case appsv1alpha1.Delete, appsv1alpha1.WipeOut:
		if err := c.deletePVCs(cluster); err != nil && !apierrors.IsNotFound(err) {
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
	if err := c.cli.List(c.ctx.Ctx, pvcList, inNS, ml); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if err := c.cli.Delete(c.ctx.Ctx, &pvc); err != nil {
			return err
		}
	}
	return nil
}

func (c *clusterPlanBuilder) deleteBackupPolicies(cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	return c.cli.DeleteAllOf(c.ctx.Ctx, &dataprotectionv1alpha1.BackupPolicy{}, inNS, ml)
}

func (c *clusterPlanBuilder) deleteBackups(cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backups
	backups := &dataprotectionv1alpha1.BackupList{}
	if err := c.cli.List(c.ctx.Ctx, backups, inNS, ml); err != nil {
		return err
	}
	for _, backup := range backups.Items {
		// check backup delete protection label
		deleteProtection, exists := backup.GetLabels()[constant.BackupProtectionLabelKey]
		// not found backup-protection or value is Delete, delete it.
		if !exists || deleteProtection == constant.BackupDelete {
			if err := c.cli.Delete(c.ctx.Ctx, &backup); err != nil {
				return err
			}
		}
	}
	return nil
}
