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

package apps

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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=resourcequotas/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=replicasets/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// dataprotection get list and delete
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;delete;deletecollection
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

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

func clusterUpdateHandler(cli client.Client, ctx context.Context, clusterDef *appsv1alpha1.ClusterDefinition) error {
	labelSelector, err := labels.Parse("clusterdefinition.kubeblocks.io/name=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &appsv1alpha1.ClusterList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, cluster := range list.Items {
		if cluster.Status.ClusterDefGeneration != clusterDef.Generation {
			patch := client.MergeFrom(cluster.DeepCopy())
			if cluster.Status.Operations == nil {
				cluster.Status.Operations = &appsv1alpha1.Operations{}
			}
			cluster.Status.Operations.HorizontalScalable =
				getSupportHorizontalScalingComponents(&cluster, clusterDef)
			if err = cli.Status().Patch(ctx, &cluster, patch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ClusterReconciler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != component.ProbeRoleChangedCheckPath {
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
	cluster := &appsv1alpha1.Cluster{}
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

	// should patch the label first to prevent the label from being modified by the user.
	if err = r.patchClusterLabelsIfNotExist(ctx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	reqCtx.Log.Info("get clusterDef and clusterVersion")
	clusterDefinition := &appsv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Name: cluster.Spec.ClusterDefRef,
	}, clusterDefinition); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		// If using RequeueWithError and the user fixed this error,
		// it may take up to 1000s to reconcile again, causing the user to think that the repair is not effective.
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}

	if cluster.Status.ObservedGeneration == cluster.Generation {
		// reconcile the phase and conditions of the Cluster.status
		if err = r.reconcileClusterStatus(reqCtx.Ctx, cluster, clusterDefinition); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	reqCtx.Log.Info("update cluster status")
	if err = r.updateClusterPhaseToCreatingOrUpdating(reqCtx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Name: cluster.Spec.ClusterVersionRef,
	}, clusterVersion); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterVersion.Status.Phase,
		appsv1alpha1.ClusterVersionKind, clusterVersion.Name); res != nil {
		return *res, err
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, clusterConditionMgr, clusterDefinition.Status.Phase,
		appsv1alpha1.ClusterDefinitionKind, clusterDefinition.Name); res != nil {
		return *res, err
	}

	if err = r.reconcileStatusOperations(ctx, cluster, clusterDefinition); err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}
	// validate config and send warning event log necessarily
	if err = cluster.ValidateEnabledLogs(clusterDefinition); err != nil {
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}

	// validate primaryIndex and send warning event log necessarily
	if err = cluster.ValidatePrimaryIndex(clusterDefinition); err != nil {
		_ = clusterConditionMgr.setPreCheckErrorCondition(err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// preCheck succeed, starting the cluster provisioning
	if err = clusterConditionMgr.setProvisioningStartedCondition(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	clusterDeepCopy := cluster.DeepCopy()
	shouldRequeue, err := reconcileClusterWorkloads(reqCtx, r.Client, clusterDefinition, clusterVersion, cluster)
	if err != nil {
		// this is a block to handle error.
		// so when update cluster conditions failed, we can ignore it.
		_ = clusterConditionMgr.setApplyResourcesFailedCondition(err)
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}
	if shouldRequeue {
		if err = r.patchClusterStatus(reqCtx.Ctx, cluster, clusterDeepCopy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "")
	}

	if err = r.handleClusterStatusAfterApplySucceed(ctx, cluster, clusterDeepCopy, clusterDefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// patchClusterStatus patches the cluster status.
func (r *ClusterReconciler) patchClusterStatus(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterDeepCopy *appsv1alpha1.Cluster) error {
	if reflect.DeepEqual(cluster.Status, clusterDeepCopy.Status) {
		return nil
	}
	patch := client.MergeFrom(clusterDeepCopy)
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// handleClusterStatusAfterApplySucceed when cluster apply resources successful, handle the status
func (r *ClusterReconciler) handleClusterStatusAfterApplySucceed(
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterDeepCopy *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition) error {
	patch := client.MergeFrom(clusterDeepCopy)
	// apply resources succeed, record the condition and event
	applyResourcesCondition := newApplyResourcesCondition()
	cluster.SetStatusCondition(applyResourcesCondition)
	// if cluster status is ConditionsError, do it before updated the observedGeneration.
	r.updateClusterPhaseWhenConditionsError(cluster)
	// update observed generation
	cluster.Status.ObservedGeneration = cluster.Generation
	cluster.Status.ClusterDefGeneration = clusterDef.Generation
	if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	r.Recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
	return nil
}

func (r *ClusterReconciler) patchClusterLabelsIfNotExist(
	ctx context.Context,
	cluster *appsv1alpha1.Cluster) error {
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cdLabelName := cluster.Labels[clusterDefLabelKey]
	cvLabelName := cluster.Labels[clusterVersionLabelKey]
	cdName, cvName := cluster.Spec.ClusterDefRef, cluster.Spec.ClusterVersionRef
	if cdLabelName == cdName && cvLabelName == cvName {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Labels[clusterDefLabelKey] = cdName
	cluster.Labels[clusterVersionLabelKey] = cvName
	return r.Client.Patch(ctx, cluster, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO: add filter predicate for core API objects
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Complete(r)
}

func (r *ClusterReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) (*ctrl.Result, error) {
	//
	// delete any external resources
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		if cluster.Status.Phase != appsv1alpha1.DeletingPhase {
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
	case appsv1alpha1.Delete, appsv1alpha1.WipeOut:
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
		if cluster.Spec.TerminationPolicy == appsv1alpha1.WipeOut {
			// TODO check whether delete backups together with cluster is allowed
			// wipe out all backups
			if err := r.deleteBackups(reqCtx, cluster); err != nil && !apierrors.IsNotFound(err) {
				res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
				return &res, err
			}
		}
	}

	// it's possible at time of external resource deletion, cluster definition has already been deleted.
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

	// all resources created in reconcileClusterWorkloads should be handled properly

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.StatefulSetSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.DeploymentSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.ServiceSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.SecretSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.ConfigMapSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, intctrlutil.PodDisruptionBudgetSignature, inNS, ml); err != nil {
		return ret, err
	}

	return nil, nil
}

func removeFinalizer[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	r *ClusterReconciler, reqCtx intctrlutil.RequestCtx, _ func(T, L), opts ...client.ListOption) (*ctrl.Result, error) {
	var (
		objList L
	)
	if err := r.List(reqCtx.Ctx, PL(&objList), opts...); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	for _, obj := range reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T) {
		pobj := PT(&obj)
		if !controllerutil.ContainsFinalizer(pobj, dbClusterFinalizerName) {
			continue
		}
		patch := client.MergeFrom(PT(pobj.DeepCopy()))
		controllerutil.RemoveFinalizer(pobj, dbClusterFinalizerName)
		if err := r.Patch(reqCtx.Ctx, pobj, patch); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}
	return nil, nil
}

func (r *ClusterReconciler) deletePVCs(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	// it's possible at time of external resource deletion, cluster definition has already been deleted.
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

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

func (r *ClusterReconciler) deleteBackupPolicies(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	return r.Client.DeleteAllOf(reqCtx.Ctx, &dataprotectionv1alpha1.BackupPolicy{}, inNS, ml)
}

func (r *ClusterReconciler) deleteBackups(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
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
		// check backup delete protection label
		deleteProtection, exists := backup.GetLabels()[intctrlutil.BackupProtectionLabelKey]
		// not found backup-protection or value is Delete, delete it.
		if !exists || deleteProtection == intctrlutil.BackupDelete {
			if err := r.Delete(reqCtx.Ctx, &backup); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkReferencingCRStatus checks if cluster referenced CR is available
func (r *ClusterReconciler) checkReferencedCRStatus(
	reqCtx intctrlutil.RequestCtx,
	conMgr clusterConditionManager,
	referencedCRPhase appsv1alpha1.Phase,
	crKind, crName string) (*ctrl.Result, error) {
	if referencedCRPhase == appsv1alpha1.AvailablePhase {
		return nil, nil
	}
	message := fmt.Sprintf("%s: %s is unavailable, this problem needs to be solved first.", crKind, crName)
	if err := conMgr.setReferenceCRUnavailableCondition(message); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	res, err := intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	return &res, err
}

func (r *ClusterReconciler) needCheckClusterForReady(cluster *appsv1alpha1.Cluster) bool {
	return slices.Index([]appsv1alpha1.Phase{"", appsv1alpha1.DeletingPhase, appsv1alpha1.VolumeExpandingPhase},
		cluster.Status.Phase) == -1
}

// existsOperations checks if the cluster are doing operations
func (r *ClusterReconciler) existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	return len(opsRequestMap) > 0
}

// updateClusterPhase updates cluster.status.phase
func (r *ClusterReconciler) updateClusterPhaseToCreatingOrUpdating(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	needPatch := false
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		needPatch = true
		cluster.Status.Phase = appsv1alpha1.CreatingPhase
		cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
		for _, v := range cluster.Spec.ComponentSpecs {
			cluster.Status.Components[v.Name] = appsv1alpha1.ClusterComponentStatus{
				Phase: appsv1alpha1.CreatingPhase,
			}
		}
	} else if util.IsCompleted(cluster.Status.Phase) && !r.existsOperations(cluster) {
		needPatch = true
		cluster.Status.Phase = appsv1alpha1.SpecUpdatingPhase
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
func (r *ClusterReconciler) updateClusterPhaseWhenConditionsError(cluster *appsv1alpha1.Cluster) {
	if cluster.Status.Phase != appsv1alpha1.ConditionsErrorPhase {
		return
	}
	if cluster.Status.ObservedGeneration == 0 {
		cluster.Status.Phase = appsv1alpha1.CreatingPhase
		return
	}
	opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	// if no operations in cluster, means user update the cluster.spec directly
	if len(opsRequestSlice) == 0 {
		cluster.Status.Phase = appsv1alpha1.SpecUpdatingPhase
		return
	}
	// if exits opsRequests are running, set the cluster phase to the early target phase with the OpsRequest
	cluster.Status.Phase = opsRequestSlice[0].ToClusterPhase
}

// reconcileClusterStatus reconciles phase and conditions of the Cluster.status.
func (r *ClusterReconciler) reconcileClusterStatus(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition) error {
	if !r.needCheckClusterForReady(cluster) {
		return nil
	}
	if cluster.Status.Components == nil {
		return nil
	}

	var (
		currentClusterPhase       appsv1alpha1.Phase
		existsAbnormalOrFailed    bool
		replicasNotReadyCompNames = map[string]struct{}{}
		notReadyCompNames         = map[string]struct{}{}
	)

	// analysis the status of components and calculate the cluster phase .
	analysisComponentsStatus := func(cluster *appsv1alpha1.Cluster) {
		var (
			runningCompCount int
			stoppedCompCount int
		)
		for k, v := range cluster.Status.Components {
			if v.PodsReady == nil || !*v.PodsReady {
				replicasNotReadyCompNames[k] = struct{}{}
				notReadyCompNames[k] = struct{}{}
			}
			switch v.Phase {
			case appsv1alpha1.AbnormalPhase, appsv1alpha1.FailedPhase:
				existsAbnormalOrFailed = true
				notReadyCompNames[k] = struct{}{}
			case appsv1alpha1.RunningPhase:
				runningCompCount += 1
			case appsv1alpha1.StoppedPhase:
				stoppedCompCount += 1
			}
		}
		switch len(cluster.Status.Components) {
		case 0:
			// if no components, return
			return
		case runningCompCount:
			currentClusterPhase = appsv1alpha1.RunningPhase
		case runningCompCount + stoppedCompCount:
			// cluster is Stopped when cluster is not Running and all components are Stopped or Running
			currentClusterPhase = appsv1alpha1.StoppedPhase
		}
	}

	// remove the invalid component in status.components when spec.components changed and analysis the status of components.
	removeInvalidComponentsAndAnalysis := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		tmpCompsStatus := map[string]appsv1alpha1.ClusterComponentStatus{}
		compsStatus := cluster.Status.Components
		for _, v := range cluster.Spec.ComponentSpecs {
			if compStatus, ok := compsStatus[v.Name]; ok {
				tmpCompsStatus[v.Name] = compStatus
			}
		}
		var needPatch bool
		if len(tmpCompsStatus) != len(compsStatus) {
			// keep valid components' status
			cluster.Status.Components = tmpCompsStatus
			needPatch = true
		}
		analysisComponentsStatus(cluster)
		return needPatch, nil
	}

	// handle the cluster conditions with ClusterReady and ReplicasReady type.
	handleClusterReadyCondition := func(cluster *appsv1alpha1.Cluster) (needPatch bool, postFunc postHandler) {
		return handleNotReadyConditionForCluster(cluster, r.Recorder, replicasNotReadyCompNames, notReadyCompNames)
	}

	// processes cluster phase changes.
	processClusterPhaseChanges := func(cluster *appsv1alpha1.Cluster,
		oldPhase,
		currPhase appsv1alpha1.Phase,
		eventType string,
		eventMessage string,
		doAction func(cluster *appsv1alpha1.Cluster)) (bool, postHandler) {
		if oldPhase == currPhase {
			return false, nil
		}
		cluster.Status.Phase = currPhase
		if doAction != nil {
			doAction(cluster)
		}
		postFuncAfterPatch := func(currCluster *appsv1alpha1.Cluster) error {
			r.Recorder.Event(currCluster, eventType, string(currPhase), eventMessage)
			return opsutil.MarkRunningOpsRequestAnnotation(ctx, r.Client, currCluster)
		}
		return true, postFuncAfterPatch
	}
	// handle the Cluster.status when some components of cluster are Abnormal or Failed.
	handleExistAbnormalOrFailed := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if !existsAbnormalOrFailed {
			return false, nil
		}
		oldPhase := cluster.Status.Phase
		componentMap, clusterAvailabilityEffectMap, _ := getComponentRelatedInfo(cluster, clusterDef, "")
		// handle the cluster status when some components are not ready.
		handleClusterAbnormalOrFailedPhase(cluster, componentMap, clusterAvailabilityEffectMap)
		currPhase := cluster.Status.Phase
		if !util.IsFailedOrAbnormal(currPhase) {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s is %s, check according to the components message", cluster.Name, currPhase)
		return processClusterPhaseChanges(cluster, oldPhase, currPhase, corev1.EventTypeWarning, message, nil)
	}

	// handle the Cluster.status when cluster is Stopped.
	handleClusterIsStopped := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if currentClusterPhase != appsv1alpha1.StoppedPhase {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase, corev1.EventTypeNormal, message, nil)
	}

	// handle the Cluster.status when cluster is Running.
	handleClusterIsRunning := func(cluster *appsv1alpha1.Cluster) (bool, postHandler) {
		if currentClusterPhase != appsv1alpha1.RunningPhase {
			return false, nil
		}
		message := fmt.Sprintf("Cluster: %s is ready, current phase is Running.", cluster.Name)
		action := func(currCluster *appsv1alpha1.Cluster) {
			currCluster.SetStatusCondition(newClusterReadyCondition(currCluster.Name))
		}
		oldPhase := cluster.Status.Phase
		return processClusterPhaseChanges(cluster, oldPhase, currentClusterPhase, corev1.EventTypeNormal, message, action)
	}
	return doChainClusterStatusHandler(ctx, r.Client, cluster, removeInvalidComponentsAndAnalysis,
		handleClusterReadyCondition, handleExistAbnormalOrFailed, handleClusterIsStopped, handleClusterIsRunning)
}

// reconcileStatusOperations when Cluster.spec updated, we need reconcile the Cluster.status.operations.
func (r *ClusterReconciler) reconcileStatusOperations(ctx context.Context, cluster *appsv1alpha1.Cluster, clusterDef *appsv1alpha1.ClusterDefinition) error {
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &appsv1alpha1.Operations{}
	}

	var (
		err                       error
		upgradable                bool
		volumeExpansionComponents []appsv1alpha1.OperationComponent
		oldOperations             = cluster.Status.Operations.DeepCopy()
		operations                = *cluster.Status.Operations
		clusterVersionList        = &appsv1alpha1.ClusterVersionList{}
	)
	// determine whether to support volumeExpansion when creating the cluster or add/delete component.
	// because volumeClaimTemplates are forbidden to update except for storage size when component created.
	if cluster.Status.ObservedGeneration == 0 || len(cluster.Spec.ComponentSpecs) != len(cluster.Status.Components) {
		if volumeExpansionComponents, err = getSupportVolumeExpansionComponents(ctx, r.Client, cluster); err != nil {
			return err
		}
		operations.VolumeExpandable = volumeExpansionComponents
	}
	// determine whether to support horizontalScaling
	operations.HorizontalScalable = getSupportHorizontalScalingComponents(cluster, clusterDef)
	// set default supported operations
	clusterComponentNames := getComponentsNames(cluster)
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

// getComponentsNames get all components' names
func getComponentsNames(
	cluster *appsv1alpha1.Cluster) []string {
	clusterComponentNames := make([]string, 0)
	for _, v := range cluster.Spec.ComponentSpecs {
		clusterComponentNames = append(clusterComponentNames, v.Name)
	}
	return clusterComponentNames
}

// getSupportHorizontalScalingComponents gets the components that support horizontalScaling
func getSupportHorizontalScalingComponents(
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition) []appsv1alpha1.OperationComponent {
	horizontalScalableComponents := make([]appsv1alpha1.OperationComponent, 0)

	// determine whether to support horizontalScaling
	for _, v := range cluster.Spec.ComponentSpecs {
		for _, component := range clusterDef.Spec.ComponentDefs {
			if v.ComponentDefRef != component.Name {
				continue
			}
			horizontalScalableComponents = append(horizontalScalableComponents, appsv1alpha1.OperationComponent{
				Name: v.Name,
			})
			break
		}
	}

	return horizontalScalableComponents
}
