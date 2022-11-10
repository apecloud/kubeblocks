/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/storage"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers;statefulsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// NOTES: owned K8s core API resources controller-gen RBAC marker is maintained at {REPO}/controllers/k8score/rbac.go

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type probeMessage struct {
	Event        string `json:"event,omitempty"`
	OriginalRole string `json:"originalRole,omitempty"`
	Role         string `json:"role,omitempty"`
}

func init() {
	clusterDefUpdateHandlers["cluster"] = clusterUpdateHandler
	k8score.EventHandlerMap["cluster-controller"] = &ClusterReconciler{}
}

func clusterUpdateHandler(cli client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error {

	labelSelector, err := labels.Parse("clusterdefinition.kubeblocks.io/name=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &dbaasv1alpha1.ClusterList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, cluster := range list.Items {
		if cluster.Status.ClusterDefGeneration != clusterDef.GetObjectMeta().GetGeneration() {
			patch := client.MergeFrom(cluster.DeepCopy())
			// sync status.Operations.HorizontalScalable
			horizontalScalableComponents, _ := getSupportHorizontalScalingComponents(&cluster, clusterDef)
			cluster.Status.Operations.HorizontalScalable = horizontalScalableComponents
			cluster.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
			if err = cli.Status().Patch(ctx, &cluster, patch); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ClusterReconciler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != k8score.ProbeRoleChangedCheckPath {
		return nil
	}

	// get role
	message := &probeMessage{}
	re := regexp.MustCompile(`Readiness probe failed: {.*({.*}).*}`)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) != 2 {
		return nil
	}
	msg := strings.ReplaceAll(matches[1], "\\", "")
	err := json.Unmarshal([]byte(msg), message)
	if err != nil {
		// not role related message, ignore it
		reqCtx.Log.Info("not role message", "message", event.Message, "error", err)
		return nil
	}
	role := strings.ToLower(message.Role)
	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}

	return updateConsensusSetRoleLabel(cli, reqCtx.Ctx, podName, role)
}

func updateConsensusSetRoleLabel(cli client.Client, ctx context.Context, podName types.NamespacedName, role string) error {
	// get pod
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = role
	err := cli.Patch(ctx, pod, patch)
	if err != nil {
		return err
	}

	// update cluster status
	// get cluster obj
	cluster := &dbaasv1alpha1.Cluster{}
	err = cli.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Labels[intctrlutil.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}

	// get componentDef this pod belongs to
	componentName := pod.Labels[intctrlutil.AppComponentLabelKey]
	typeName := component.GetComponentTypeName(*cluster, componentName)
	componentDef, err := component.GetComponentFromClusterDefinition(ctx, cli, cluster, typeName)
	if err != nil {
		return err
	}

	// get all role names
	leaderName := componentDef.ConsensusSpec.Leader.Name
	followersMap := make(map[string]dbaasv1alpha1.ConsensusMember, 0)
	for _, follower := range componentDef.ConsensusSpec.Followers {
		followersMap[follower.Name] = follower
	}
	learnerName := ""
	if componentDef.ConsensusSpec.Learner != nil {
		learnerName = componentDef.ConsensusSpec.Learner.Name
	}

	// prepare cluster status patch
	patch = client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]*dbaasv1alpha1.ClusterStatusComponent)
	}
	if cluster.Status.Components[componentName] == nil {
		cluster.Status.Components[componentName] = &dbaasv1alpha1.ClusterStatusComponent{
			Type:  typeName,
			Phase: dbaasv1alpha1.RunningPhase,
			ConsensusSetStatus: &dbaasv1alpha1.ConsensusSetStatus{
				Leader: dbaasv1alpha1.ConsensusMemberStatus{
					Pod: consensusSetStatusDefaultPodName,
				},
			},
		}
	}
	componentStatus := cluster.Status.Components[componentName]
	if componentStatus.ConsensusSetStatus == nil {
		componentStatus.ConsensusSetStatus = &dbaasv1alpha1.ConsensusSetStatus{
			Leader: dbaasv1alpha1.ConsensusMemberStatus{
				Pod: consensusSetStatusDefaultPodName,
			},
		}
	}
	consensusSetStatus := componentStatus.ConsensusSetStatus

	resetLeader := func() {
		if consensusSetStatus.Leader.Pod == pod.Name {
			consensusSetStatus.Leader.Pod = consensusSetStatusDefaultPodName
			consensusSetStatus.Leader.AccessMode = dbaasv1alpha1.None
			consensusSetStatus.Leader.Name = ""
		}
	}
	resetLearner := func() {
		if consensusSetStatus.Learner != nil && consensusSetStatus.Learner.Pod == pod.Name {
			consensusSetStatus.Learner = nil
		}
	}

	resetFollower := func() {
		for index, member := range consensusSetStatus.Followers {
			if member.Pod == pod.Name {
				consensusSetStatus.Followers = append(consensusSetStatus.Followers[:index], consensusSetStatus.Followers[index+1:]...)
			}
		}
	}

	// set pod.Name to the right status field
	accessMode := dbaasv1alpha1.AccessMode("")
	needUpdate := false
	switch role {
	case leaderName:
		consensusSetStatus.Leader.Pod = pod.Name
		consensusSetStatus.Leader.AccessMode = componentDef.ConsensusSpec.Leader.AccessMode
		consensusSetStatus.Leader.Name = componentDef.ConsensusSpec.Leader.Name
		accessMode = componentDef.ConsensusSpec.Leader.AccessMode
		resetLearner()
		resetFollower()
		needUpdate = true
	case learnerName:
		if consensusSetStatus.Learner == nil {
			consensusSetStatus.Learner = &dbaasv1alpha1.ConsensusMemberStatus{}
		}
		consensusSetStatus.Learner.Pod = pod.Name
		consensusSetStatus.Learner.AccessMode = componentDef.ConsensusSpec.Learner.AccessMode
		consensusSetStatus.Learner.Name = componentDef.ConsensusSpec.Learner.Name
		accessMode = componentDef.ConsensusSpec.Learner.AccessMode
		resetLeader()
		resetFollower()
		needUpdate = true
	default:
		if follower, ok := followersMap[role]; ok {
			exist := false
			for _, member := range consensusSetStatus.Followers {
				if member.Pod == pod.Name {
					exist = true
				}
			}
			if !exist {
				member := dbaasv1alpha1.ConsensusMemberStatus{
					Pod:        pod.Name,
					AccessMode: follower.AccessMode,
					Name:       follower.Name,
				}
				accessMode = follower.AccessMode
				consensusSetStatus.Followers = append(consensusSetStatus.Followers, member)
				resetLeader()
				resetLearner()
				needUpdate = true
			}
		}
	}

	// finally, update cluster status
	if needUpdate {
		err = cli.Status().Patch(ctx, cluster, patch)
		if err != nil {
			return err
		}

		// update pod accessMode label
		patchAccessMode := client.MergeFrom(pod.DeepCopy())
		pod.Labels[consensusSetAccessModeLabelKey] = string(accessMode)
		return cli.Patch(ctx, pod, patchAccessMode)
	}

	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.Info("get cluster", "cluster", req.NamespacedName)
	cluster := &dbaasv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, cluster, dbClusterFinalizerName, func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, cluster)
	})
	if res != nil {
		return *res, err
	}

	if cluster.Status.ObservedGeneration == cluster.GetObjectMeta().GetGeneration() {
		// check cluster all pods is ready
		if err = r.checkAndPatchToRunning(reqCtx.Ctx, cluster); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	reqCtx.Log.Info("get clusterdef and appversion")
	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ClusterDefRef,
	}, clusterdefinition); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
	}
	appversion := &dbaasv1alpha1.AppVersion{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.AppVersionRef,
	}, appversion); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(cluster, r.Recorder, err, reqCtx.Log)
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, cluster, appversion.Status.Phase, dbaasv1alpha1.AppVersionKind); res != nil {
		return *res, err
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, cluster, clusterdefinition.Status.Phase, dbaasv1alpha1.ClusterDefinitionKind); res != nil {
		return *res, err
	}

	reqCtx.Log.Info("update cluster status")
	if err = r.updateClusterPhaseToCreatingOrUpdating(reqCtx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err = r.reconcileStatusOperations(ctx, cluster, clusterdefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	task, err := buildClusterCreationTasks(clusterdefinition, appversion, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if err = task.Exec(reqCtx, r.Client); err != nil {
		// record the event when the execution task reports an error.
		r.Recorder.Event(cluster, corev1.EventTypeWarning, intctrlutil.EventReasonRunTaskFailed, err.Error())
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// update observed generation
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.ObservedGeneration = cluster.ObjectMeta.Generation
	cluster.Status.ClusterDefGeneration = clusterdefinition.ObjectMeta.Generation
	if err = r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if cluster.ObjectMeta.Labels == nil {
		cluster.ObjectMeta.Labels = map[string]string{}
	}
	_, ok := cluster.ObjectMeta.Labels[clusterDefLabelKey]
	if !ok {
		cluster.ObjectMeta.Labels[clusterDefLabelKey] = clusterdefinition.Name
		cluster.ObjectMeta.Labels[appVersionLabelKey] = appversion.Name
		if err = r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.Cluster{}).
		//
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Complete(r)
}

func (r *ClusterReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) (*ctrl.Result, error) {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	switch cluster.Spec.TerminationPolicy {
	case dbaasv1alpha1.DoNotTerminate:
		if cluster.Status.Phase != dbaasv1alpha1.DeletingPhase {
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.ObservedGeneration = cluster.Generation
			cluster.Status.Phase = dbaasv1alpha1.DeletingPhase
			cluster.Status.Message = fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
			if err := r.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
				res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
				return &res, err
			}
		}
		res, err := intctrlutil.Reconciled()
		return &res, err
	case dbaasv1alpha1.Delete, dbaasv1alpha1.WipeOut:
		if err := r.deletePVCs(reqCtx, cluster); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}

	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Get(reqCtx.Ctx, client.ObjectKey{
		Name: cluster.Spec.ClusterDefRef,
	}, clusterDef); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}

	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
		intctrlutil.AppNameLabelKey:     fmt.Sprintf("%s-%s", clusterDef.Spec.Type, clusterDef.Name),
	}
	inNS := client.InNamespace(cluster.Namespace)
	stsList := &appsv1.StatefulSetList{}
	if err := r.List(reqCtx.Ctx, stsList, inNS, ml); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	for _, sts := range stsList.Items {
		if !controllerutil.ContainsFinalizer(&sts, dbClusterFinalizerName) {
			continue
		}
		patch := client.MergeFrom(sts.DeepCopy())
		controllerutil.RemoveFinalizer(&sts, dbClusterFinalizerName)
		if err := r.Patch(reqCtx.Ctx, &sts, patch); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}
	svcList := &corev1.ServiceList{}
	if err := r.List(reqCtx.Ctx, svcList, inNS, ml); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	for _, svc := range svcList.Items {
		if !controllerutil.ContainsFinalizer(&svc, dbClusterFinalizerName) {
			continue
		}
		patch := client.MergeFrom(svc.DeepCopy())
		controllerutil.RemoveFinalizer(&svc, dbClusterFinalizerName)
		if err := r.Patch(reqCtx.Ctx, &svc, patch); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}
	secretList := &corev1.SecretList{}
	if err := r.List(reqCtx.Ctx, secretList, inNS, ml); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	for _, secret := range secretList.Items {
		if !controllerutil.ContainsFinalizer(&secret, dbClusterFinalizerName) {
			continue
		}
		patch := client.MergeFrom(secret.DeepCopy())
		controllerutil.RemoveFinalizer(&secret, dbClusterFinalizerName)
		if err := r.Patch(reqCtx.Ctx, &secret, patch); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}
	return nil, nil
}

func (r *ClusterReconciler) deletePVCs(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {

	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Get(reqCtx.Ctx, client.ObjectKey{
		Name: cluster.Spec.ClusterDefRef,
	}, clusterDef); err != nil {
		return err
	}

	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
		intctrlutil.AppNameLabelKey:     fmt.Sprintf("%s-%s", clusterDef.Spec.Type, clusterDef.Name),
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(reqCtx.Ctx, pvcList, inNS, ml); err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if err := r.Delete(reqCtx.Ctx, &pvc); err != nil {
			return err
		}
	}
	return nil
}

// checkReferencingCRStatus check cluster referenced CR is available
func (r *ClusterReconciler) checkReferencedCRStatus(reqCtx intctrlutil.RequestCtx,
	cluster *dbaasv1alpha1.Cluster,
	referencedCRPhase dbaasv1alpha1.Phase,
	crKind string) (*ctrl.Result, error) {
	if referencedCRPhase == dbaasv1alpha1.AvailablePhase {
		return nil, nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Message = fmt.Sprintf("%s.status.phase is unavailable, this problem needs to be solved first", crKind)
	if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	r.Recorder.Event(cluster, corev1.EventTypeWarning, intctrlutil.EventReasonRefCRUnavailable, cluster.Status.Message)
	res, err := intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "")
	return &res, err
}

func (r *ClusterReconciler) needCheckClusterForReady(cluster *dbaasv1alpha1.Cluster) bool {
	return slices.Index([]dbaasv1alpha1.Phase{"", dbaasv1alpha1.RunningPhase, dbaasv1alpha1.DeletingPhase},
		cluster.Status.Phase) == -1
}

// updateClusterPhase update cluster.status.phase
func (r *ClusterReconciler) updateClusterPhaseToCreatingOrUpdating(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {
	if slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.CreatingPhase, dbaasv1alpha1.UpdatingPhase},
		cluster.Status.Phase) != -1 {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		cluster.Status.Phase = dbaasv1alpha1.CreatingPhase
	} else {
		cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
	}
	// send an event when cluster perform operations
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(cluster.Status.Phase),
		"Start %s in Cluster: %s", cluster.Status.Phase, cluster.Name)
	return r.Client.Status().Patch(reqCtx.Ctx, cluster, patch)
}

// checkAndPatchToRunning patch Cluster.status.phase to Running
func (r *ClusterReconciler) checkAndPatchToRunning(ctx context.Context, cluster *dbaasv1alpha1.Cluster) error {
	if !r.needCheckClusterForReady(cluster) {
		return nil
	}
	// synchronize the latest status of components
	if err := r.handleComponentStatus(ctx, cluster); err != nil {
		return err
	}
	if cluster.Status.Components == nil {
		return nil
	}
	for _, v := range cluster.Status.Components {
		if v.Phase != dbaasv1alpha1.RunningPhase {
			return nil
		}
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	// send an event when Cluster.status.phase change to Running
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(dbaasv1alpha1.RunningPhase), "Cluster: %s is ready, current phase is Running", cluster.Name)
	return nil
}

// handleComponentStatus cluster controller and component controller are tuned asynchronously.
// before processing whether the component is running, need to synchronize the latest status of components firstly.
// it can prevent the use of expired component status, which may lead to inconsistent cluster status.
func (r *ClusterReconciler) handleComponentStatus(ctx context.Context, cluster *dbaasv1alpha1.Cluster) error {
	var (
		needSyncDeploymentStatus  bool
		needSyncStatefulSetStatus bool
		err                       error
	)
	patch := client.MergeFrom(cluster.DeepCopy())
	// handle stateless component status
	if needSyncDeploymentStatus, err = r.handleComponentStatusWithDeployment(ctx, cluster); err != nil {
		return err
	}
	// handle stateful/consensus component status
	if needSyncStatefulSetStatus, err = r.handleComponentStatusWithStatefulSet(ctx, cluster); err != nil {
		return err
	}
	if needSyncDeploymentStatus || needSyncStatefulSetStatus {
		if err = r.Client.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
		return component.MarkRunningOpsRequestAnnotation(ctx, r.Client, cluster)
	}
	return nil
}

// handleComponentStatusWithStatefulSet handle the component status with statefulSet. One statefulSet corresponds to one component.
func (r *ClusterReconciler) handleComponentStatusWithStatefulSet(ctx context.Context, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		needSyncComponentStatus bool
		statefulSetList         = &appsv1.StatefulSetList{}
		componentTypeMap        map[string]dbaasv1alpha1.ComponentType
		err                     error
	)
	if componentTypeMap, err = getComponentTypeMapWithCluster(ctx, r.Client, cluster); err != nil {
		return false, err
	}
	if err = getObjectList(ctx, r.Client, cluster, statefulSetList); err != nil {
		return false, err
	}
	for _, sts := range statefulSetList.Items {
		componentName := sts.GetLabels()[intctrlutil.AppComponentLabelKey]
		if len(componentName) == 0 {
			continue
		}
		componentType := componentTypeMap[componentName]
		statefulStatusRevisionIsEquals := true
		switch componentType {
		case dbaasv1alpha1.Consensus:
			if statefulStatusRevisionIsEquals, err = checkConsensusStatefulSetRevision(ctx, r.Client, &sts); err != nil {
				return false, err
			}
		case dbaasv1alpha1.Stateful:
			// when stateful updateStrategy is rollingUpdate, need to check revision
			if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
				statefulStatusRevisionIsEquals = false
			}
		}
		componentIsRunning := component.StatefulSetIsReady(&sts, statefulStatusRevisionIsEquals)
		if ok := component.NeedSyncStatusComponents(cluster, componentName, componentIsRunning); ok {
			needSyncComponentStatus = true
		}
	}
	return needSyncComponentStatus, nil
}

// handleComponentStatusWithDeployment handle the component status with deployment. One deployment corresponds to one component.
func (r *ClusterReconciler) handleComponentStatusWithDeployment(ctx context.Context, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		needSyncComponentStatus bool
		deploymentList          = &appsv1.DeploymentList{}
	)
	if err := getObjectList(ctx, r.Client, cluster, deploymentList); err != nil {
		return false, err
	}
	for _, deploy := range deploymentList.Items {
		componentName := deploy.GetLabels()[intctrlutil.AppComponentLabelKey]
		if len(componentName) == 0 {
			continue
		}
		componentIsRunning := component.DeploymentIsReady(&deploy)
		if ok := component.NeedSyncStatusComponents(cluster, componentName, componentIsRunning); ok {
			needSyncComponentStatus = true
		}
	}
	return needSyncComponentStatus, nil
}

// reconcileStatusOperations when Cluster.spec updated, we need reconcile the Cluster.status.operations.
func (r *ClusterReconciler) reconcileStatusOperations(ctx context.Context, cluster *dbaasv1alpha1.Cluster, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	var (
		err                       error
		upgradable                bool
		volumeExpansionComponents []*dbaasv1alpha1.OperationComponent
		oldOperations             = cluster.Status.Operations.DeepCopy()
		operations                = cluster.Status.Operations
		appVersionList            = &dbaasv1alpha1.AppVersionList{}
	)
	// determine whether to support volumeExpansion
	if volumeExpansionComponents, err = r.getSupportVolumeExpansionComponents(ctx, cluster); err != nil {
		return err
	}
	operations.VolumeExpandable = volumeExpansionComponents

	// determine whether to support horizontalScaling
	horizontalScalableComponents, clusterComponentNames := getSupportHorizontalScalingComponents(cluster, clusterDef)
	operations.HorizontalScalable = horizontalScalableComponents
	// set default supported operations
	operations.Restartable = clusterComponentNames
	operations.VerticalScalable = clusterComponentNames

	// Determine whether to support upgrade
	if err = r.Client.List(ctx, appVersionList, client.MatchingLabels{clusterDefLabelKey: cluster.Spec.ClusterDefRef}); err != nil {
		return err
	}
	if len(appVersionList.Items) > 1 {
		upgradable = true
	}
	operations.Upgradable = upgradable

	// check whether status.operations is changed
	if reflect.DeepEqual(oldOperations, operations) {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Operations = operations
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// getSupportVolumeExpansionComponents Get the components that support volume expansion and the volumeClaimTemplates
func (r *ClusterReconciler) getSupportVolumeExpansionComponents(ctx context.Context,
	cluster *dbaasv1alpha1.Cluster) ([]*dbaasv1alpha1.OperationComponent, error) {
	var (
		storageClassMap             = map[string]bool{}
		hasCheckDefaultStorageClass bool
		// the default storageClass may not exist, so use a bool key to check
		defaultStorageClassAllowExpansion bool
		volumeExpansionComponents         = make([]*dbaasv1alpha1.OperationComponent, 0)
	)
	for _, v := range cluster.Spec.Components {
		operationComponent := &dbaasv1alpha1.OperationComponent{}
		for _, vct := range v.VolumeClaimTemplates {
			if vct.Spec == nil {
				continue
			}
			if ok, err := r.checkStorageClassIsSupportExpansion(ctx, storageClassMap, vct.Spec.StorageClassName,
				&hasCheckDefaultStorageClass, &defaultStorageClassAllowExpansion); err != nil {
				return nil, err
			} else if ok {
				operationComponent.VolumeClaimTemplateNames = append(operationComponent.VolumeClaimTemplateNames, vct.Name)
			}
		}

		if len(operationComponent.VolumeClaimTemplateNames) > 0 {
			operationComponent.Name = v.Name
			volumeExpansionComponents = append(volumeExpansionComponents, operationComponent)
		}
	}
	return volumeExpansionComponents, nil
}

// checkStorageClassIsSupportExpansion check whether the storageClass supports volume expansion
func (r *ClusterReconciler) checkStorageClassIsSupportExpansion(ctx context.Context,
	storageClassMap map[string]bool,
	storageClassName *string,
	hasCheckDefaultStorageClass *bool,
	defaultStorageClassAllowExpansion *bool) (bool, error) {
	var (
		ok  bool
		err error
	)
	if storageClassName != nil {
		if ok, err = r.checkSpecifyStorageClass(ctx, storageClassMap, *storageClassName); err != nil {
			return false, err
		}
		return ok, nil
	} else {
		// get the default StorageClass whether supports volume expansion for the first time
		if !*hasCheckDefaultStorageClass {
			if *defaultStorageClassAllowExpansion, err = r.checkDefaultStorageClass(ctx); err != nil {
				return false, err
			}
			*hasCheckDefaultStorageClass = true
		}
		return *defaultStorageClassAllowExpansion, nil
	}
}

// checkStorageClassIsSupportExpansion check whether the specified storageClass supports volume expansion
func (r *ClusterReconciler) checkSpecifyStorageClass(ctx context.Context, storageClassMap map[string]bool, storageClassName string) (bool, error) {
	var (
		supportVolumeExpansion bool
	)
	if val, ok := storageClassMap[storageClassName]; ok {
		return val, nil
	}
	// if storageClass is not in the storageClassMap, get it
	storageClass := &storagev1.StorageClass{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: storageClassName}, storageClass); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	// get bool value of StorageClass.AllowVolumeExpansion and put it to storageClassMap
	if storageClass != nil && storageClass.AllowVolumeExpansion != nil {
		supportVolumeExpansion = *storageClass.AllowVolumeExpansion
	}
	storageClassMap[storageClassName] = supportVolumeExpansion
	return supportVolumeExpansion, nil
}

// checkDefaultStorageClass check whether the default storageClass supports volume expansion
func (r *ClusterReconciler) checkDefaultStorageClass(ctx context.Context) (bool, error) {
	storageClassList := &storagev1.StorageClassList{}
	if err := r.Client.List(ctx, storageClassList); err != nil {
		return false, err
	}
	// check the first default storageClass
	for _, sc := range storageClassList.Items {
		if _, ok := sc.Annotations[storage.IsDefaultStorageClassAnnotation]; ok {
			return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion, nil
		}
	}
	return false, nil
}

// getSupportHorizontalScalingComponents Get the components that support horizontalScaling
func getSupportHorizontalScalingComponents(cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) ([]*dbaasv1alpha1.OperationComponent, []string) {
	var (
		clusterComponentNames        = make([]string, 0)
		horizontalScalableComponents = make([]*dbaasv1alpha1.OperationComponent, 0)
	)
	// determine whether to support horizontalScaling
	for _, v := range cluster.Spec.Components {
		clusterComponentNames = append(clusterComponentNames, v.Name)
		for _, component := range clusterDef.Spec.Components {
			if v.Type != component.TypeName || (component.MinAvailable != 0 &&
				component.MaxAvailable == component.MinAvailable) {
				continue
			}
			horizontalScalableComponents = append(horizontalScalableComponents, &dbaasv1alpha1.OperationComponent{
				Name: v.Name,
				Min:  component.MinAvailable,
				Max:  component.MaxAvailable,
			})
			break
		}
	}

	return horizontalScalableComponents, clusterComponentNames
}
