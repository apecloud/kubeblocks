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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// clusterPlanBuilder a graph.PlanBuilder implementation for Cluster reconciliation
type clusterPlanBuilder struct {
	ctx           intctrlutil.RequestCtx
	cli           client.Client
	req           ctrl.Request
	recorder      record.EventRecorder
	cluster       *appsv1alpha1.Cluster
	originCluster appsv1alpha1.Cluster
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	ctx      intctrlutil.RequestCtx
	cli      client.Client
	recorder record.EventRecorder
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cluster  *appsv1alpha1.Cluster
}

var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.ctx.Ctx, c.req.NamespacedName, cluster); err != nil {
		return err
	}
	c.cluster = cluster
	c.originCluster = *cluster.DeepCopy()
	// handles the cluster phase and ops condition first to indicates what the current cluster is doing.
	c.handleClusterPhase()
	c.handleLatestOpsRequestProcessingCondition()
	return nil
}

// updateClusterPhase handles the cluster phase and ops condition first to indicates what the current cluster is doing.
func (c *clusterPlanBuilder) handleClusterPhase() {
	clusterPhase := c.cluster.Status.Phase
	if isClusterUpdating(*c.cluster) {
		if clusterPhase == "" {
			c.cluster.Status.Phase = appsv1alpha1.CreatingClusterPhase
		} else if clusterPhase != appsv1alpha1.CreatingClusterPhase {
			c.cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
		}
	}
}

// updateLatestOpsRequestProcessingCondition handles the latest opsRequest processing condition.
func (c *clusterPlanBuilder) handleLatestOpsRequestProcessingCondition() {
	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(c.cluster)
	if len(opsRecords) == 0 {
		return
	}
	ops := opsRecords[0]
	opsBehaviour, ok := appsv1alpha1.OpsRequestBehaviourMapper[ops.Type]
	if !ok {
		return
	}
	opsCondition := newOpsRequestProcessingCondition(ops.Name, string(ops.Type), opsBehaviour.ProcessingReasonInClusterCondition)
	oldCondition := meta.FindStatusCondition(c.cluster.Status.Conditions, opsCondition.Type)
	if oldCondition == nil {
		// if this condition not exists, insert it to the first position.
		opsCondition.LastTransitionTime = metav1.Now()
		c.cluster.Status.Conditions = append([]metav1.Condition{opsCondition}, c.cluster.Status.Conditions...)
	} else {
		meta.SetStatusCondition(&c.cluster.Status.Conditions, opsCondition)
	}
}

func (c *clusterPlanBuilder) Validate() error {
	var err error
	defer func() {
		if err != nil {
			_ = c.updateClusterStatusWithCondition(newFailedProvisioningStartedCondition(err.Error(), ReasonPreCheckFailed))
		}
	}()

	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err = c.cli.Get(c.ctx.Ctx, key, object)
		if err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
		return nil
	}

	// validate cd & cv existences
	cd := &appsv1alpha1.ClusterDefinition{}
	if err = validateExistence(types.NamespacedName{Name: c.cluster.Spec.ClusterDefRef}, cd); err != nil {
		return err
	}
	var cv *appsv1alpha1.ClusterVersion
	if len(c.cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err = validateExistence(types.NamespacedName{Name: c.cluster.Spec.ClusterVersionRef}, cv); err != nil {
			return err
		}
	}

	// validate cd & cv availability
	if cd.Status.Phase != appsv1alpha1.AvailablePhase || (cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase) {
		message := fmt.Sprintf("ref resource is unavailable, this problem needs to be solved first. cd: %v, cv: %v", cd, cv)
		err = errors.New(message)
		return newRequeueError(requeueDuration, message)
	}

	// validate logs
	// and a sample validator chain
	chain := &graph.ValidatorChain{
		&enableLogsValidator{cluster: c.cluster, clusterDef: cd},
	}
	if err = chain.WalkThrough(); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	return nil
}

func (c *clusterPlanBuilder) handleProvisionStartedCondition() {
	// set provisioning cluster condition
	condition := newProvisioningStartedCondition(c.cluster.Name, c.cluster.Generation)
	oldCondition := meta.FindStatusCondition(c.cluster.Status.Conditions, condition.Type)
	if conditionIsChanged(oldCondition, condition) {
		meta.SetStatusCondition(&c.cluster.Status.Conditions, condition)
		c.recorder.Event(c.cluster, corev1.EventTypeNormal, condition.Reason, condition.Message)
	}
}

// Build only cluster Creation, Update and Deletion supported.
func (c *clusterPlanBuilder) Build() (graph.Plan, error) {
	// set provisioning cluster condition
	c.handleProvisionStartedCondition()
	var err error
	defer func() {
		if err != nil {
			_ = c.updateClusterStatusWithCondition(newFailedApplyResourcesCondition(err.Error()))
		}
	}()

	var cr *clusterRefResources
	cr, err = c.getClusterRefResources()
	if err != nil {
		return nil, err
	}

	// TODO: remove all cli & ctx fields from transformers, keep them in pure-dag-manipulation form
	// build transformer chain
	chain := &graph.TransformerChain{
		// init dag, that is put cluster vertex into dag
		&initTransformer{cluster: c.cluster, originCluster: &c.originCluster},
		// fill class related info
		&fillClass{cc: *cr, cli: c.cli, ctx: c.ctx},
		// fix cd&cv labels of cluster
		&fixClusterLabelsTransformer{},
		// create cluster connection credential secret object
		&clusterCredentialTransformer{cc: *cr},
		// create all components objects
		&componentTransformer{cc: *cr, cli: c.cli, ctx: c.ctx},
		// add our finalizer to all objects
		&ownershipTransformer{finalizer: dbClusterFinalizerName},
		// make all non-secret objects depending on all secrets
		&secretTransformer{},
		// make config configmap immutable
		&configTransformer{},
		// handle TerminationPolicyType=DoNotTerminate
		&doNotTerminateTransformer{},
		// finally, update cluster status
		newClusterStatusTransformer(c.ctx, c.cli, c.recorder, *cr),
	}

	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	if err = chain.ApplyTo(dag); err != nil {
		return nil, err
	}

	c.ctx.Log.Info(fmt.Sprintf("DAG: %s", dag))
	// we got the execution plan
	plan := &clusterPlan{
		ctx:      c.ctx,
		cli:      c.cli,
		recorder: c.recorder,
		dag:      dag,
		walkFunc: c.defaultWalkFuncWithLogging,
		cluster:  c.cluster,
	}
	return plan, nil
}

func (c *clusterPlanBuilder) updateClusterStatusWithCondition(condition metav1.Condition) error {
	oldCondition := meta.FindStatusCondition(c.cluster.Status.Conditions, condition.Type)
	meta.SetStatusCondition(&c.cluster.Status.Conditions, condition)
	if !reflect.DeepEqual(c.cluster.Status, c.originCluster.Status) {
		if err := c.cli.Status().Patch(c.ctx.Ctx, c.cluster, client.MergeFrom(c.originCluster.DeepCopy())); err != nil {
			return err
		}
	}
	// Normal events are only sent once.
	if !conditionIsChanged(oldCondition, condition) && condition.Status == metav1.ConditionTrue {
		return nil
	}
	eventType := corev1.EventTypeWarning
	if condition.Status == metav1.ConditionTrue {
		eventType = corev1.EventTypeNormal
	}
	c.recorder.Event(c.cluster, eventType, condition.Reason, condition.Message)
	return nil
}

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
// TODO: change ctx to context.Context
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request, recorder record.EventRecorder) graph.PlanBuilder {
	return &clusterPlanBuilder{
		ctx:      ctx,
		cli:      cli,
		req:      req,
		recorder: recorder,
	}
}

func (p *clusterPlan) Execute() error {
	err := p.dag.WalkReverseTopoOrder(p.walkFunc)
	if err != nil {
		if hErr := p.handleDAGWalkError(err); hErr != nil {
			return hErr
		}
	}
	return err
}

func (p *clusterPlan) handleDAGWalkError(err error) error {
	condition := newFailedApplyResourcesCondition(err.Error())
	meta.SetStatusCondition(&p.cluster.Status.Conditions, condition)
	p.recorder.Event(p.cluster, corev1.EventTypeWarning, condition.Reason, condition.Message)
	rootVertex, _ := ictrltypes.FindRootVertex(p.dag)
	if rootVertex == nil {
		return nil
	}
	originCluster, _ := rootVertex.ObjCopy.(*appsv1alpha1.Cluster)
	if originCluster == nil || reflect.DeepEqual(originCluster.Status, p.cluster.Status) {
		return nil
	}
	return p.cli.Status().Patch(p.ctx.Ctx, p.cluster, client.MergeFrom(originCluster.DeepCopy()))
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

func (c *clusterPlanBuilder) defaultWalkFuncWithLogging(vertex graph.Vertex) error {
	node, ok := vertex.(*ictrltypes.LifecycleVertex)
	err := c.defaultWalkFunc(vertex)
	if err != nil {
		//fmt.Printf("plan - walking object %s error: %s\n", node, err.Error())
		if !ok {
			c.ctx.Log.Error(err, "")
		} else if node.Action == nil {
			c.ctx.Log.Error(err, "%T", node)
		} else {
			c.ctx.Log.Error(err, "%s %T error", *node.Action, node.Obj)
		}
	}
	return err
}

func (c *clusterPlanBuilder) defaultWalkFunc(vertex graph.Vertex) error {
	node, ok := vertex.(*ictrltypes.LifecycleVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", vertex)
	}
	if node.Action == nil {
		return errors.New("node action can't be nil")
	}

	// cluster object has more business to do, handle them here
	if _, ok := node.Obj.(*appsv1alpha1.Cluster); ok {
		if done, err := c.reconcileCluster(node); err != nil {
			return err
		} else if done {
			return nil
		}
	}
	return c.reconcileObject(node)
}

func (c *clusterPlanBuilder) reconcileObject(node *ictrltypes.LifecycleVertex) error {
	switch *node.Action {
	case ictrltypes.CREATE:
		err := c.cli.Create(c.ctx.Ctx, node.Obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case ictrltypes.UPDATE:
		if node.Immutable {
			return nil
		}
		err := c.cli.Update(c.ctx.Ctx, node.Obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case ictrltypes.DELETE:
		if controllerutil.RemoveFinalizer(node.Obj, dbClusterFinalizerName) {
			err := c.cli.Update(c.ctx.Ctx, node.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		if node.Orphan {
			err := c.cli.Delete(c.ctx.Ctx, node.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case ictrltypes.STATUS:
		if node.Immutable {
			return nil
		}
		patch := client.MergeFrom(node.ObjCopy)
		if err := c.cli.Status().Patch(c.ctx.Ctx, node.Obj, patch); err != nil {
			return err
		}
		for _, postHandle := range node.PostHandleAfterStatusPatch {
			if err := postHandle(); err != nil {
				return err
			}
		}
	case ictrltypes.NOOP:
		// nothing
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileCluster(node *ictrltypes.LifecycleVertex) (bool, error) {
	cluster := node.Obj.(*appsv1alpha1.Cluster).DeepCopy()
	origCluster := node.ObjCopy.(*appsv1alpha1.Cluster)
	switch *node.Action {
	// cluster.meta and cluster.spec might change
	case ictrltypes.CREATE, ictrltypes.UPDATE, ictrltypes.STATUS:
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
				return false, err
			}
		}
	case ictrltypes.DELETE:
		if err := c.handleClusterDeletion(cluster); err != nil {
			return false, err
		}
		if cluster.Spec.TerminationPolicy == appsv1alpha1.DoNotTerminate {
			return true, nil
		}
	}
	return false, nil
}

func (c *clusterPlanBuilder) handleClusterDeletion(cluster *appsv1alpha1.Cluster) error {
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		c.recorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate", "spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
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
