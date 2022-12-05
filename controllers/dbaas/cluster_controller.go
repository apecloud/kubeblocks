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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	"github.com/apecloud/kubeblocks/controllers/dbaas/operations"
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
	if event.InvolvedObject.FieldPath != k8score.ProbeRoleChangedCheckPath {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}

	// filter role changed event that has been handled
	annotations := event.GetAnnotations()
	if annotations != nil && annotations[CSRoleChangedAnnotKey] == CSRoleChangedAnnotHandled {
		return nil
	}

	if err := handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[CSRoleChangedAnnotKey] = CSRoleChangedAnnotHandled
	return cli.Patch(reqCtx.Ctx, event, patch)
}

func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
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
	// get pod
	pod := &corev1.Pod{}
	if err := cli.Get(reqCtx.Ctx, podName, pod); err != nil {
		return err
	}
	// event belongs to old pod with the same name, ignore it
	if pod.UID != event.InvolvedObject.UID {
		return nil
	}

	return component.UpdateConsensusSetRoleLabel(cli, reqCtx, pod, role)
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
	// validate config and send warning event log necessarily
	conditionList := cluster.ValidateEnabledLogs(clusterdefinition)
	if len(conditionList) > 0 {
		patch := client.MergeFrom(cluster.DeepCopy())
		for _, cond := range conditionList {
			meta.SetStatusCondition(&cluster.Status.Conditions, *cond)
		}
		if err = r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		// send events after status patched
		for _, cond := range conditionList {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, cond.Reason, cond.Message)
		}
	}

	res, err = createCluster(reqCtx, r.Client, clusterdefinition, appversion, cluster)
	if err != nil || res != nil {
		if err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, intctrlutil.EventReasonRunTaskFailed, err.Error())
		}
		return *res, err
	}

	// update observed generation
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.ObservedGeneration = cluster.ObjectMeta.Generation
	cluster.Status.ClusterDefGeneration = clusterdefinition.ObjectMeta.Generation
	if err = r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	_, ok := cluster.Labels[clusterDefLabelKey]
	if !ok {
		cluster.Labels[clusterDefLabelKey] = clusterdefinition.Name
		cluster.Labels[appVersionLabelKey] = appversion.Name
		if err = r.Client.Patch(reqCtx.Ctx, cluster, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
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
	return slices.Index([]dbaasv1alpha1.Phase{"", dbaasv1alpha1.RunningPhase, dbaasv1alpha1.DeletingPhase, dbaasv1alpha1.VolumeExpandingPhase},
		cluster.Status.Phase) == -1
}

// existsOperations check the cluster are doing operations
func (r *ClusterReconciler) existsOperations(cluster *dbaasv1alpha1.Cluster) bool {
	opsRequestMap, _ := operations.GetOpsRequestMapFromCluster(cluster)
	return len(opsRequestMap) > 0
}

// updateClusterPhase update cluster.status.phase
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
	// TODO handle when component phase is Failed/Abnormal.
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
	if err = getObjectListForCluster(ctx, r.Client, cluster, statefulSetList); err != nil {
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
			// TODO check pod is ready by role label
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
	if err := getObjectListForCluster(ctx, r.Client, cluster, deploymentList); err != nil {
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
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &dbaasv1alpha1.Operations{}
	}

	var (
		err                       error
		upgradable                bool
		volumeExpansionComponents []dbaasv1alpha1.OperationComponent
		oldOperations             = cluster.Status.Operations.DeepCopy()
		operations                = *cluster.Status.Operations
		appVersionList            = &dbaasv1alpha1.AppVersionList{}
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
	cluster.Status.Operations = &operations
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// getSupportHorizontalScalingComponents Get the components that support horizontalScaling
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
