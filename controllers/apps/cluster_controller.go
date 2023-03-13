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
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
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
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;delete;deletecollection
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// ClusterStatusEventHandler is the event handler for the cluster status event
type ClusterStatusEventHandler struct{}

var _ k8score.EventHandler = &ClusterStatusEventHandler{}

func init() {
	k8score.EventHandlerMap["cluster-status-handler"] = &ClusterStatusEventHandler{}
}

// Handle is the event handler for the cluster status event.
func (r *ClusterStatusEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != component.ProbeRoleChangedCheckPath {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}

	// parse probe event message when field path is probe-role-changed-check
	message := k8score.ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return nil
	}

	// if probe message event is checkRoleFailed, it means the cluster is abnormal, need to handle the cluster status
	if message.Event == k8score.ProbeEventCheckRoleFailed {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}
	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
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

	reqCtx.Log.V(1).Info("reconcile", "cluster", req.NamespacedName)
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	deletionHandler := func() (*ctrl.Result, error) { return r.deleteExternalResources(reqCtx, cluster) }
	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, cluster, dbClusterFinalizerName, deletionHandler); res != nil {
		return *res, err
	}

	// should patch the label first to prevent the label from being modified by the user.
	if err := r.patchClusterLabelsIfNotExist(ctx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	condMgr := clusterConditionManager{
		Client:   r.Client,
		Recorder: r.Recorder,
		ctx:      ctx,
		cluster:  cluster,
	}
	clusterDefinition, clusterVersion, res, err := r.getACheckReferencedCRs(reqCtx, condMgr, cluster)
	if res != nil {
		return *res, err
	}

	if cluster.Status.ObservedGeneration == cluster.Generation {
		// checks if the controller is handling the garbage of restore.
		if handlingRestoreGarbage, err := r.handleGarbageOfRestoreBeforeRunning(ctx, cluster); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		} else if handlingRestoreGarbage {
			return intctrlutil.Reconciled()
		}
		// reconcile the phase and conditions of the Cluster.status
		if err = r.reconcileClusterStatus(reqCtx.Ctx, cluster, clusterDefinition); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if err = r.cleanupAnnotationsAfterRunning(reqCtx, cluster); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	// do a cross-check with referenced cluster definition
	if res, err := r.checkWithReferencedCD(reqCtx, condMgr, clusterDefinition, cluster); res != nil {
		return *res, err
	}

	if err := r.updateClusterStatusBeforeProvisioning(reqCtx, condMgr, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	clusterCopy := cluster.DeepCopy()
	requeue, err := reconcileClusterWorkloads(reqCtx, r.Client, clusterDefinition, clusterVersion, cluster)
	if err != nil {
		_ = condMgr.setApplyResourcesFailedCondition(err)
		return intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
	}
	if requeue {
		if err = r.patchClusterStatus(reqCtx.Ctx, cluster, clusterCopy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "")
	}

	if err = r.updateClusterStatusAfterProvisioning(reqCtx, cluster, clusterCopy, clusterDefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *ClusterReconciler) checkWithReferencedCD(reqCtx intctrlutil.RequestCtx, condMgr clusterConditionManager,
	cd *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster) (*ctrl.Result, error) {
	if err := cluster.ValidateEnabledLogs(cd); err != nil {
		_ = condMgr.setPreCheckErrorCondition(err)
		res, err := intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
		return &res, err
	}
	if err := cluster.ValidatePrimaryIndex(cd); err != nil {
		_ = condMgr.setPreCheckErrorCondition(err)
		res, err := intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	return nil, nil
}

func (r *ClusterReconciler) updateClusterStatusBeforeProvisioning(reqCtx intctrlutil.RequestCtx,
	condMgr clusterConditionManager, cluster *appsv1alpha1.Cluster) error {
	if err := r.updateClusterPhaseToCreatingOrUpdating(reqCtx, cluster); err != nil {
		return err
	}
	return condMgr.setProvisioningStartedCondition()
}

// updateClusterStatusAfterProvisioning when cluster apply resources successful, handle the status
func (r *ClusterReconciler) updateClusterStatusAfterProvisioning(reqCtx intctrlutil.RequestCtx,
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
	if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return err
	}
	r.Recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
	return nil
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
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	inNS := client.InNamespace(cluster.Namespace)

	// all resources created in reconcileClusterWorkloads should be handled properly

	if ret, err := removeFinalizer(r, reqCtx, generics.StatefulSetSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, generics.DeploymentSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, generics.ServiceSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, generics.SecretSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, generics.ConfigMapSignature, inNS, ml); err != nil {
		return ret, err
	}

	if ret, err := removeFinalizer(r, reqCtx, generics.PodDisruptionBudgetSignature, inNS, ml); err != nil {
		return ret, err
	}

	return nil, nil
}

func removeFinalizer[T generics.Object, PT generics.PObject[T],
	L generics.ObjList[T], PL generics.PObjList[T, L]](
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
		constant.AppInstanceLabelKey: cluster.GetName(),
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
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backupPolicies
	return r.Client.DeleteAllOf(reqCtx.Ctx, &dataprotectionv1alpha1.BackupPolicy{}, inNS, ml)
}

func (r *ClusterReconciler) deleteBackups(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.GetName(),
	}
	// clean backups
	backups := &dataprotectionv1alpha1.BackupList{}
	if err := r.List(reqCtx.Ctx, backups, inNS, ml); err != nil {
		return err
	}
	for _, backup := range backups.Items {
		// check backup delete protection label
		deleteProtection, exists := backup.GetLabels()[constant.BackupProtectionLabelKey]
		// not found backup-protection or value is Delete, delete it.
		if !exists || deleteProtection == constant.BackupDelete {
			if err := r.Delete(reqCtx.Ctx, &backup); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ClusterReconciler) getACheckReferencedCRs(reqCtx intctrlutil.RequestCtx, condMgr clusterConditionManager,
	cluster *appsv1alpha1.Cluster) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, *ctrl.Result, error) {
	cd, res, err := getACheckReferencedCR(reqCtx, r.Client, condMgr, cluster.Spec.ClusterDefRef, generics.ClusterDefinitionSignature)
	if res != nil {
		return nil, nil, res, err
	}

	// cluster version is empty
	if len(cluster.Spec.ClusterVersionRef) == 0 {
		return cd, &appsv1alpha1.ClusterVersion{}, nil, nil
	}

	cv, res, err := getACheckReferencedCR(reqCtx, r.Client, condMgr, cluster.Spec.ClusterVersionRef, generics.ClusterVersionSignature)
	if res != nil {
		return nil, nil, res, err
	}
	return cd, cv, nil, nil
}

func getACheckReferencedCR[T generics.Object, PT generics.PObject[T], L generics.ObjList[T]](reqCtx intctrlutil.RequestCtx,
	cli client.Client, condMgr clusterConditionManager, resourceName string, _ func(T, L)) (PT, *ctrl.Result, error) {
	var obj T
	pobj := PT(&obj)
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: resourceName}, pobj); err != nil {
		_ = condMgr.setPreCheckErrorCondition(err)
		// If using RequeueWithError and the user fixed this error,
		// it may take up to 1000s to reconcile again, causing the user to think that the repair is not effective.
		res, err := intctrlutil.RequeueAfter(ControllerErrorRequeueTime, reqCtx.Log, "")
		return nil, &res, err
	}

	if res, err := checkReferencedCRStatus(reqCtx, condMgr, pobj); res != nil {
		return nil, res, err
	}

	return pobj, nil, nil
}

func checkReferencedCRStatus(reqCtx intctrlutil.RequestCtx, condMgr clusterConditionManager, obj client.Object) (*ctrl.Result, error) {
	if reflect.ValueOf(obj).MethodByName("Ready").Call([]reflect.Value{})[0].Bool() {
		return nil, nil
	}

	message := fmt.Sprintf("%s: %s is unavailable, this problem needs to be solved first.",
		obj.GetObjectKind(), obj.GetName())
	if err := condMgr.setReferenceCRUnavailableCondition(message); err != nil {
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

// updateClusterPhase updates cluster.status.phase
func (r *ClusterReconciler) updateClusterPhaseToCreatingOrUpdating(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	needPatch := false
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		needPatch = true
		cluster.Status.Phase = appsv1alpha1.CreatingPhase
		cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
		for _, v := range cluster.Spec.ComponentSpecs {
			cluster.Status.SetComponentStatus(v.Name, appsv1alpha1.ClusterComponentStatus{
				Phase: appsv1alpha1.CreatingPhase,
			})
		}
	} else if componentutil.IsCompleted(cluster.Status.Phase) && !existsOperations(cluster) {
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
	if len(cluster.Status.Components) == 0 {
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
		handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
		currPhase := cluster.Status.Phase
		if !componentutil.IsFailedOrAbnormal(currPhase) {
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

// cleanupAnnotationsAfterRunning cleans up the cluster annotations after cluster is Running.
func (r *ClusterReconciler) cleanupAnnotationsAfterRunning(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	if cluster.Status.Phase != appsv1alpha1.RunningPhase {
		return nil
	}
	if _, ok := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]; !ok {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	delete(cluster.Annotations, constant.RestoreFromBackUpAnnotationKey)
	return r.Client.Patch(reqCtx.Ctx, cluster, patch)
}

// handleRestoreGarbageBeforeRunning handles the garbage for restore before cluster phase changes to Running.
func (r *ClusterReconciler) handleGarbageOfRestoreBeforeRunning(ctx context.Context, cluster *appsv1alpha1.Cluster) (bool, error) {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(cluster)
	if err != nil {
		return false, err
	}
	if clusterBackupResourceMap == nil {
		return false, nil
	}
	// check if all components are running.
	for _, v := range cluster.Status.Components {
		if v.Phase != appsv1alpha1.RunningPhase {
			return false, nil
		}
	}
	// remove the garbage for restore if the cluster restores from backup.
	return r.removeGarbageWithRestore(ctx, cluster, clusterBackupResourceMap)
}

// removeGarbageWithRestore removes the garbage for restore when all components are Running.
func (r *ClusterReconciler) removeGarbageWithRestore(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterBackupResourceMap map[string]string) (bool, error) {
	var (
		doRemoveInitContainers bool
		err                    error
	)
	clusterPatch := client.MergeFrom(cluster.DeepCopy())
	for k, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if doRemoveInitContainers, err = r.removeStsInitContainerForRestore(ctx, cluster, k, v); err != nil {
			return false, err
		}
	}
	if doRemoveInitContainers {
		// reset the component phase to Creating during removing the init containers of statefulSet.
		return doRemoveInitContainers, r.Client.Status().Patch(ctx, cluster, clusterPatch)
	}
	return false, nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (r *ClusterReconciler) removeStsInitContainerForRestore(ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	componentName,
	backupName string) (bool, error) {
	// get the sts list of component
	stsList := &appsv1.StatefulSetList{}
	if err := componentutil.GetObjectListByComponentName(ctx, r.Client, *cluster, stsList, componentName); err != nil {
		return false, err
	}
	var doRemoveInitContainers bool
	for _, sts := range stsList.Items {
		initContainers := sts.Spec.Template.Spec.InitContainers
		restoreInitContainerName := component.GetRestoredInitContainerName(backupName)
		restoreInitContainerIndex, _ := intctrlutil.GetContainerByName(initContainers, restoreInitContainerName)
		if restoreInitContainerIndex == -1 {
			continue
		}
		doRemoveInitContainers = true
		initContainers = append(initContainers[:restoreInitContainerIndex], initContainers[restoreInitContainerIndex+1:]...)
		sts.Spec.Template.Spec.InitContainers = initContainers
		if err := r.Client.Update(ctx, &sts); err != nil {
			return false, err
		}
	}
	if doRemoveInitContainers {
		// if need to remove init container, reset component to Creating.
		compStatus := cluster.Status.Components[componentName]
		compStatus.Phase = appsv1alpha1.CreatingPhase
		cluster.Status.SetComponentStatus(componentName, compStatus)
	}
	return doRemoveInitContainers, nil
}
