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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusters/finalizers,verbs=update

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=core,resources=secrets;configmaps;services;resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status;resourcequotas/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers;secrets/finalizers;configmaps/finalizers;resourcequotas/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update

// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

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
	k8score.StorageClassHandlerMap["cluster-controller"] = handleClusterVolumeExpansion
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
			if cluster.Status.Operations == nil {
				cluster.Status.Operations = &dbaasv1alpha1.Operations{}
			}
			cluster.Status.Operations.HorizontalScalable = horizontalScalableComponents
			cluster.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
			if err = cli.Status().Patch(ctx, &cluster, patch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ClusterReconciler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != ProbeRoleChangedCheckPath {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}
	var (
		role        string
		err         error
		annotations = event.GetAnnotations()
	)
	// filter role changed event that has been handled
	if annotations != nil && annotations[CSRoleChangedAnnotKey] == CSRoleChangedAnnotHandled {
		return nil
	}

	if role, err = handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[CSRoleChangedAnnotKey] = CSRoleChangedAnnotHandled
	if err = cli.Patch(reqCtx.Ctx, event, patch); err != nil {
		return err
	}
	if role != "" {
		return nil
	}
	// if role is empty, means the event is not role changed event, handle it.
	return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
}

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) (string, error) {
	// get role
	message := &probeMessage{}
	re := regexp.MustCompile(`Readiness probe failed: ({.*})`)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) != 2 {
		return "", nil
	}
	msg := matches[1]
	err := json.Unmarshal([]byte(msg), message)
	if err != nil {
		// not role related message, ignore it
		reqCtx.Log.Info("not role message", "message", event.Message, "error", err)
		return "", nil
	}
	role := strings.ToLower(message.Role)

	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	// get pod
	pod := &corev1.Pod{}
	if err = cli.Get(reqCtx.Ctx, podName, pod); err != nil {
		return role, err
	}
	// event belongs to old pod with the same name, ignore it
	if pod.UID != event.InvolvedObject.UID {
		return role, nil
	}

	return role, consensusset.UpdateConsensusSetRoleLabel(cli, reqCtx, pod, role)
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
	clusterConditionMgr := clusterConditionManager{
		Client:   r.Client,
		Recorder: r.Recorder,
		ctx:      ctx,
		cluster:  cluster,
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, cluster, dbClusterFinalizerName, func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, cluster)
	})
	if res != nil {
		return *res, err
	}

	reqCtx.Log.Info("get clusterDef and clusterVersion")
	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ClusterDefRef,
	}, clusterdefinition); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	if cluster.Status.ObservedGeneration == cluster.GetObjectMeta().GetGeneration() {
		// check cluster all pods is ready
		if err = r.checkAndPatchToRunning(reqCtx.Ctx, cluster, clusterdefinition); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ClusterVersionRef,
	}, clusterVersion); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterVersion.Status.Phase,
		dbaasv1alpha1.ClusterVersionKind, clusterVersion.Name); res != nil {
		return *res, err
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterdefinition.Status.Phase,
		dbaasv1alpha1.ClusterDefinitionKind, clusterdefinition.Name); res != nil {
		return *res, err
	}

	reqCtx.Log.Info("update cluster status")
	if err = r.updateClusterPhaseToCreatingOrUpdating(reqCtx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err = r.reconcileStatusOperations(ctx, cluster, clusterdefinition); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	// validate config and send warning event log necessarily
	if err = cluster.ValidateEnabledLogs(clusterdefinition); err != nil {
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// preCheck succeed, starting the cluster provisioning
	if err = clusterConditionMgr.setProvisioningStartedCondition(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	shouldRequeue, err := createCluster(reqCtx, r.Client, clusterdefinition, clusterVersion, cluster)
	if err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setApplyResourcesFailedCondition(err)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if shouldRequeue {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "")
	}

	if err = r.handleClusterStatusAfterApplySucceed(ctx, cluster, clusterdefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err = r.patchClusterLabels(ctx, cluster, clusterdefinition, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// handleClusterStatusAfterApplySucceed when cluster apply resources successful, handle the status
func (r *ClusterReconciler) handleClusterStatusAfterApplySucceed(
	ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	// apply resources succeed, record the condition and event
	applyResourcesCondition := newApplyResourcesCondition()
	cluster.SetStatusCondition(applyResourcesCondition)
	// if cluster status is ConditionsError, do it before updated the observedGeneration.
	r.updateClusterPhaseWhenConditionsError(cluster)
	// update observed generation
	cluster.Status.ObservedGeneration = cluster.ObjectMeta.Generation
	cluster.Status.ClusterDefGeneration = clusterDef.ObjectMeta.Generation
	if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	r.Recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
	return nil
}

func (r *ClusterReconciler) patchClusterLabels(
	ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition,
	clusterVersion *dbaasv1alpha1.ClusterVersion) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	_, ok := cluster.Labels[clusterDefLabelKey]
	if !ok {
		cluster.Labels[clusterDefLabelKey] = clusterDef.Name
		cluster.Labels[clusterVersionLabelKey] = clusterVersion.Name
		return r.Client.Patch(ctx, cluster, patch)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: add filter predicate for core API objects
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.Cluster{}).
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
			cluster.Status.Message = fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
			if err := r.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
				res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
				return &res, err
			}
		}
		res, err := intctrlutil.Reconciled()
		return &res, err
	case dbaasv1alpha1.Delete, dbaasv1alpha1.WipeOut:
		if err := r.deletePVCs(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
		// The backup policy must be cleaned up when the cluster is deleted.
		// Automatic backup scheduling needs to be stopped at this point.
		if err := r.deleteBackupPolicies(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
		if cluster.Spec.TerminationPolicy == dbaasv1alpha1.WipeOut {
			// wipe out all backups
			if err := r.deleteBackups(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
				res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
				return &res, err
			}
		}
	}

	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Get(reqCtx.Ctx, client.ObjectKey{
		Name: cluster.Spec.ClusterDefRef,
	}, clusterDef); err != nil && !apierrors.IsNotFound(err) {
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

func (r *ClusterReconciler) deleteBackupPolicies(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	backupPolicies := &dataprotectionv1alpha1.BackupPolicyList{}
	if err := r.List(reqCtx.Ctx, backupPolicies, inNS, ml); err != nil {
		return err
	}
	for _, policy := range backupPolicies.Items {
		if err := r.Delete(reqCtx.Ctx, &policy); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterReconciler) deleteBackups(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backups
	backups := &dataprotectionv1alpha1.BackupList{}
	if err := r.List(reqCtx.Ctx, backups, inNS, ml); err != nil {
		return err
	}
	for _, backup := range backups.Items {
		if err := r.Delete(reqCtx.Ctx, &backup); err != nil {
			return err
		}
	}
	return nil
}

// checkReferencingCRStatus checks if cluster referenced CR is available
func (r *ClusterReconciler) checkReferencedCRStatus(
	reqCtx intctrlutil.RequestCtx,
	conMgr clusterConditionManager,
	referencedCRPhase dbaasv1alpha1.Phase,
	crKind, crName string) (*ctrl.Result, error) {
	if referencedCRPhase == dbaasv1alpha1.AvailablePhase {
		return nil, nil
	}
	message := fmt.Sprintf("%s: %s is unavailable, this problem needs to be solved first.", crKind, crName)
	if err := conMgr.setReferenceCRUnavailableCondition(message); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	res, err := intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "")
	return &res, err
}

func (r *ClusterReconciler) needCheckClusterForReady(cluster *dbaasv1alpha1.Cluster) bool {
	return slices.Index([]dbaasv1alpha1.Phase{"", dbaasv1alpha1.RunningPhase, dbaasv1alpha1.DeletingPhase, dbaasv1alpha1.VolumeExpandingPhase},
		cluster.Status.Phase) == -1
}

// existsOperations checks if the cluster are doing operations
func (r *ClusterReconciler) existsOperations(cluster *dbaasv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	return len(opsRequestMap) > 0
}

// updateClusterPhase updates cluster.status.phase
func (r *ClusterReconciler) updateClusterPhaseToCreatingOrUpdating(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {
	needPatch := false
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		needPatch = true
		cluster.Status.Phase = dbaasv1alpha1.CreatingPhase
	} else if slices.Index([]dbaasv1alpha1.Phase{
		dbaasv1alpha1.RunningPhase,
		dbaasv1alpha1.FailedPhase,
		dbaasv1alpha1.AbnormalPhase}, cluster.Status.Phase) != -1 && !r.existsOperations(cluster) {
		needPatch = true
		cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
	}
	if !needPatch {
		return nil
	}
	if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return err
	}
	// send an event when cluster perform operations
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(cluster.Status.Phase),
		"Start %s in Cluster: %s", cluster.Status.Phase, cluster.Name)
	return nil
}

// updateClusterPhaseWhenConditionsError when cluster status is ConditionsError and the cluster applies resources successful,
// we should update the cluster to the correct state
func (r *ClusterReconciler) updateClusterPhaseWhenConditionsError(cluster *dbaasv1alpha1.Cluster) {
	if cluster.Status.Phase != dbaasv1alpha1.ConditionsErrorPhase {
		return
	}
	if cluster.Status.ObservedGeneration == 0 {
		cluster.Status.Phase = dbaasv1alpha1.CreatingPhase
		return
	}
	opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	// if no operations in cluster, means user update the cluster.spec directly
	if len(opsRequestSlice) == 0 {
		cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
		return
	}
	// if exits opsRequests are running, set the cluster phase to the early target phase with the OpsRequest
	cluster.Status.Phase = opsRequestSlice[0].ToClusterPhase
}

// checkAndPatchToRunning patches Cluster.status.phase to Running
func (r *ClusterReconciler) checkAndPatchToRunning(ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	if !r.needCheckClusterForReady(cluster) {
		return nil
	}
	// synchronize the latest status of components
	if err := r.handleComponentStatus(ctx, cluster, clusterDef); err != nil {
		return err
	}

	if cluster.Status.Components == nil {
		return nil
	}

	var (
		clusterIsRunning       = true
		isReady                = true
		existsAbnormalOrFailed bool
		needSync               bool
	)
	for _, v := range cluster.Status.Components {
		// if pods of the components are not ready, return
		if v.PodsReady == nil || !*v.PodsReady {
			isReady = false
		}
		if util.IsFailedOrAbnormal(v.Phase) {
			existsAbnormalOrFailed = true
		}
		if v.Phase != dbaasv1alpha1.RunningPhase {
			clusterIsRunning = false
		}
	}
	patch := client.MergeFrom(cluster.DeepCopy())

	readyCondition := newAllReplicasPodsReadyConditions()
	if isReady {
		cluster.SetStatusCondition(readyCondition)
		needSync = true
	}
	if clusterIsRunning {
		cluster.Status.Phase = dbaasv1alpha1.RunningPhase
		cluster.SetStatusCondition(newClusterReadyCondition(cluster.Name))
		needSync = true
	} else if existsAbnormalOrFailed {
		// abnormal or failed components exist
		needSync = true
		componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster, clusterDef, "")
		handleClusterStatusPhaseByEvent(cluster, componentMap, clusterAvailabilityEffectMap)
	}
	if !needSync {
		return nil
	}
	if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	if isReady {
		// send an event when all pods of the components are ready
		r.Recorder.Event(cluster, corev1.EventTypeNormal, readyCondition.Reason, readyCondition.Message)
	}
	if clusterIsRunning {
		// send an event when Cluster.status.phase change to Running
		r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(dbaasv1alpha1.RunningPhase), "Cluster: %s is ready, current phase is Running.", cluster.Name)
		// mark OpsRequest annotation to reconcile for cluster scope OpsRequest
		return opsutil.MarkRunningOpsRequestAnnotation(ctx, r.Client, cluster)
	}
	return nil
}

// handleComponentStatus cluster controller and component controller are tuned asynchronously.
// before processing whether the component is running, need to synchronize the latest status of components firstly.
// it can prevent the use of expired component status, which may lead to inconsistent cluster status.
func (r *ClusterReconciler) handleComponentStatus(ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) error {
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
	if needSyncStatefulSetStatus, err = r.handleComponentStatusWithStatefulSet(ctx, cluster, clusterDef); err != nil {
		return err
	}
	if needSyncDeploymentStatus || needSyncStatefulSetStatus {
		if err = r.Client.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
		return opsutil.MarkRunningOpsRequestAnnotation(ctx, r.Client, cluster)
	}
	return nil
}

// handleComponentStatusWithStatefulSet handles the component status with statefulSet. One statefulSet corresponds to one component.
func (r *ClusterReconciler) handleComponentStatusWithStatefulSet(ctx context.Context,
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) (bool, error) {
	var (
		needSyncComponentStatus bool
		statefulSetList         = &appsv1.StatefulSetList{}
		err                     error
	)

	if err = getObjectListForCluster(ctx, r.Client, cluster, statefulSetList); err != nil {
		return false, err
	}
	for _, sts := range statefulSetList.Items {
		componentName := sts.GetLabels()[intctrlutil.AppComponentLabelKey]
		if len(componentName) == 0 {
			continue
		}
		typeName := util.GetComponentTypeName(*cluster, componentName)
		componentDef := util.GetComponentDefFromClusterDefinition(clusterDef, typeName)
		component := util.GetComponentByName(cluster, componentName)
		currComponent := components.NewComponentByType(ctx, r.Client, cluster, componentDef, component)
		if currComponent == nil {
			continue
		}
		componentIsRunning, err := currComponent.IsRunning(&sts)
		if err != nil {
			return false, err
		}
		podsIsReady, err := currComponent.PodsReady(&sts)
		if err != nil {
			return false, err
		}
		if ok, err := components.NeedSyncStatusComponents(cluster, currComponent, componentName, componentIsRunning, podsIsReady); err != nil {
			return false, err
		} else if ok {
			needSyncComponentStatus = true
		}
	}
	return needSyncComponentStatus, nil
}

// handleComponentStatusWithDeployment handles the component status with deployment. One deployment corresponds to one component.
func (r *ClusterReconciler) handleComponentStatusWithDeployment(ctx context.Context, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		needSyncComponentStatus bool
		deploymentList          = &appsv1.DeploymentList{}
	)
	if err := getObjectListForCluster(ctx, r.Client, cluster, deploymentList); err != nil {
		return false, err
	}
	for _, deploy := range deploymentList.Items {
		componentName := deploy.GetLabels()[intctrlutil.AppComponentLabelKey]
		if len(componentName) == 0 {
			continue
		}
		deployIsReady := stateless.DeploymentIsReady(&deploy)
		statelessComponent := stateless.NewStateless(ctx, r.Client, cluster)
		if ok, err := components.NeedSyncStatusComponents(cluster, statelessComponent,
			componentName, deployIsReady, deployIsReady); err != nil {
			return false, err
		} else if ok {
			needSyncComponentStatus = true
		}
	}
	return needSyncComponentStatus, nil
}

// reconcileStatusOperations when Cluster.spec updated, we need reconcile the Cluster.status.operations.
func (r *ClusterReconciler) reconcileStatusOperations(ctx context.Context, cluster *dbaasv1alpha1.Cluster, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &dbaasv1alpha1.Operations{}
	}

	var (
		err                       error
		upgradable                bool
		volumeExpansionComponents []dbaasv1alpha1.OperationComponent
		oldOperations             = cluster.Status.Operations.DeepCopy()
		operations                = *cluster.Status.Operations
		clusterVersionList        = &dbaasv1alpha1.ClusterVersionList{}
	)
	// determine whether to support volumeExpansion when creating the cluster. because volumeClaimTemplates is forbidden to update except for storage size when cluster created.
	if cluster.Status.ObservedGeneration == 0 {
		if volumeExpansionComponents, err = getSupportVolumeExpansionComponents(ctx, r.Client, cluster); err != nil {
			return err
		}
		operations.VolumeExpandable = volumeExpansionComponents
	}
	// determine whether to support horizontalScaling
	horizontalScalableComponents, clusterComponentNames := getSupportHorizontalScalingComponents(cluster, clusterDef)
	operations.HorizontalScalable = horizontalScalableComponents
	// set default supported operations
	operations.Restartable = clusterComponentNames
	operations.VerticalScalable = clusterComponentNames

	// Determine whether to support upgrade
	if err = r.Client.List(ctx, clusterVersionList, client.MatchingLabels{clusterDefLabelKey: cluster.Spec.ClusterDefRef}); err != nil {
		return err
	}
	if len(clusterVersionList.Items) > 1 {
		upgradable = true
	}
	operations.Upgradable = upgradable

	// check whether status.operations is changed
	if reflect.DeepEqual(oldOperations, operations) {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Operations = &operations
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// getSupportHorizontalScalingComponents gets the components that support horizontalScaling
func getSupportHorizontalScalingComponents(
	cluster *dbaasv1alpha1.Cluster,
	clusterDef *dbaasv1alpha1.ClusterDefinition) ([]dbaasv1alpha1.OperationComponent, []string) {
	var (
		clusterComponentNames        = make([]string, 0)
		horizontalScalableComponents = make([]dbaasv1alpha1.OperationComponent, 0)
	)
	// determine whether to support horizontalScaling
	for _, v := range cluster.Spec.Components {
		clusterComponentNames = append(clusterComponentNames, v.Name)
		for _, component := range clusterDef.Spec.Components {
			if v.Type != component.TypeName || (component.MinReplicas != 0 &&
				component.MaxReplicas == component.MinReplicas) {
				continue
			}
			horizontalScalableComponents = append(horizontalScalableComponents, dbaasv1alpha1.OperationComponent{
				Name: v.Name,
				Min:  component.MinReplicas,
				Max:  component.MaxReplicas,
			})
			break
		}
	}

	return horizontalScalableComponents, clusterComponentNames
}

// getObjectList gets k8s workload list with cluster
func getObjectListForCluster(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, objectList client.ObjectList) error {
	matchLabels := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  cluster.Name,
		intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
	}
	inNamespace := client.InNamespace(cluster.Namespace)
	return cli.List(ctx, objectList, matchLabels, inNamespace)
}
