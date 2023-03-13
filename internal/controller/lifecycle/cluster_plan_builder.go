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
	"golang.org/x/exp/maps"
	"reflect"
	"strings"

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
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	types2 "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type clusterPlanBuilder struct {
	ctx      intctrlutil.RequestCtx
	cli      client.Client
	req      ctrl.Request
	recorder record.EventRecorder
	cluster  *appsv1alpha1.Cluster
	conMgr   clusterConditionManager2
}

type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cluster  *appsv1alpha1.Cluster
	conMgr   clusterConditionManager2
}

var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

func (c *clusterPlanBuilder) getCompoundCluster() (*compoundCluster, error) {
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

	cc := &compoundCluster{
		cluster: cluster,
		cd:      *cd,
		cv:      *cv,
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
				if err := c.cli.Update(c.ctx.Ctx, cluster); err != nil {
					return err
				}
				//patch := client.MergeFrom(origCluster.DeepCopy())
				//if err := c.cli.Patch(c.ctx.Ctx, cluster, patch); err != nil {
				//	return err
				//}
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
			return err
		}
	case DELETE:
		if controllerutil.RemoveFinalizer(node.obj, dbClusterFinalizerName) {
			err := c.cli.Update(c.ctx.Ctx, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case STATUS:
		patch := client.MergeFrom(node.oriObj)
		return c.cli.Status().Patch(c.ctx.Ctx, node.obj, patch)
	}
	return nil
}

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.ctx.Ctx, c.req.NamespacedName, cluster); err != nil {
		return err
	}
	c.cluster = cluster
	return nil
}

func (c *clusterPlanBuilder) Validate() error {
	chain := &graph.ValidatorChain{
		&clusterDefinitionValidator{req: c.req, cli: c.cli, ctx: c.ctx, cluster: c.cluster},
		&clusterVersionValidator{req: c.req, cli: c.cli, ctx: c.ctx, cluster: c.cluster},
		&enableLogsValidator{req: c.req, cli: c.cli, ctx: c.ctx, cluster: c.cluster},
		&rplSetPrimaryIndexValidator{req: c.req, cli: c.cli, ctx: c.ctx, cluster: c.cluster},
	}
	err := chain.WalkThrough()
	if err != nil {
		_ = c.conMgr.setPreCheckErrorCondition(c.cluster, err)
	}
	return err
}

// Build only cluster Creation, Update and Deletion supported.
// TODO: Validations and Corrections (cluster labels correction, primaryIndex spec validation etc.)
func (c *clusterPlanBuilder) Build() (graph.Plan, error) {
	_ = c.conMgr.setProvisioningStartedCondition(c.cluster)
	var err error
	defer func() {
		if err != nil {
			_ = c.conMgr.setApplyResourcesFailedCondition(c.cluster, err)
		}
	}()

	var cc *compoundCluster
	cc, err = c.getCompoundCluster()
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
		&clusterTransformer{cc: *cc, cli: c.cli, ctx: c.ctx},
		// tls certs secret
		&tlsCertsTransformer{cc: *cc, cli: roClient, ctx: c.ctx},
		// add our finalizer to all objects
		&ownershipTransformer{finalizer: dbClusterFinalizerName},
		// make all workload objects depending on credential secret
		&credentialTransformer{},
		// make config configmap immutable
		&configTransformer{},
		// read old snapshot from cache, and generate diff plan
		&objectActionTransformer{cc: *cc, cli: roClient, ctx: c.ctx},
		// handle TerminationPolicyType=DoNotTerminate
		&doNotTerminateTransformer{},
		// horizontal scaling
		&stsHorizontalScalingTransformer{cc: *cc, cli: roClient, ctx: c.ctx},
		// stateful set pvc Update
		&stsPVCTransformer{cc: *cc, cli: c.cli, ctx: c.ctx},
		// replication set horizontal scaling
		//&rplSetHorizontalScalingTransformer{cc: *cc, cli: c.cli, ctx: c.ctx},
		// finally, update cluster status
		&clusterStatusTransformer{cc: *cc, cli: c.cli, ctx: c.ctx, recorder: c.recorder},
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
		if !reflect.DeepEqual(&origObj.Spec, &stsObj.Spec) {
			// sync component phase
			syncComponentPhaseWhenSpecUpdating(c.cluster, componentName)
		}

		return stsObj, nil
	}

	handleDeploy := func(origObj, deployProto *appsv1.Deployment) (client.Object, error) {
		deployObj := origObj.DeepCopy()
		deployProto.Spec.Template.Annotations = mergeAnnotations(deployObj.Spec.Template.Annotations,
			deployProto.Spec.Template.Annotations)
		deployObj.Spec = deployProto.Spec
		if !reflect.DeepEqual(&origObj.Spec, &deployObj.Spec) {
			// sync component phase
			// TODO: syncComponentPhaseWhenSpecUpdating
			//componentName := deployObj.Labels[constant.KBAppComponentLabelKey]
			//syncComponentPhaseWhenSpecUpdating(cluster, componentName)
		}
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

	switch node.obj.(type) {
	case *appsv1.StatefulSet:
		return handleSts(node.oriObj.(*appsv1.StatefulSet), node.obj.(*appsv1.StatefulSet))
	case *appsv1.Deployment:
		return handleDeploy(node.oriObj.(*appsv1.Deployment), node.obj.(*appsv1.Deployment))
	case *corev1.Service:
		return handleSvc(node.oriObj.(*corev1.Service), node.obj.(*corev1.Service))
	case *corev1.PersistentVolumeClaim:
		return handlePVC(node.oriObj.(*corev1.PersistentVolumeClaim), node.obj.(*corev1.PersistentVolumeClaim))
	case *corev1.Secret, *corev1.ConfigMap:
		return node.obj, nil
	}

	return node.obj, nil
}

func (c *clusterPlanBuilder) handleClusterDeletion(cluster *appsv1alpha1.Cluster) error {
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		if cluster.Status.Phase != appsv1alpha1.DeletingPhase {
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.ObservedGeneration = cluster.Generation
			cluster.Status.Message = fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
			if err := c.cli.Status().Patch(c.ctx.Ctx, cluster, patch); err != nil {
				return err
			}
		}
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

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if restartAnnotation, ok := originalAnnotations[constant.RestartAnnotationKey]; ok {
		if targetAnnotations == nil {
			targetAnnotations = map[string]string{}
		}
		targetAnnotations[constant.RestartAnnotationKey] = restartAnnotation
	}
	return targetAnnotations
}

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations, targetAnnotations map[string]string) map[string]string {
	if len(originalAnnotations) == 0 {
		return targetAnnotations
	}
	tmpAnnotations := make(map[string]string, len(originalAnnotations)+len(targetAnnotations))
	for k, v := range originalAnnotations {
		if !strings.HasPrefix(k, "prometheus.io") {
			tmpAnnotations[k] = v
		}
	}
	maps.Copy(tmpAnnotations, targetAnnotations)
	return tmpAnnotations
}

// syncComponentPhaseWhenSpecUpdating when workload of the component changed
// and component phase is not the phase of operations, sync component phase to 'SpecUpdating'.
func syncComponentPhaseWhenSpecUpdating(cluster *appsv1alpha1.Cluster,
	componentName string) {
	if len(componentName) == 0 {
		return
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
			componentName: {
				Phase: appsv1alpha1.SpecUpdatingPhase,
			},
		}
		return
	}
	compStatus := cluster.Status.Components[componentName]
	// if component phase is not the phase of operations, sync component phase to 'SpecUpdating'
	if util.IsCompleted(compStatus.Phase) {
		compStatus.Phase = appsv1alpha1.SpecUpdatingPhase
		cluster.Status.Components[componentName] = compStatus
	}
}
