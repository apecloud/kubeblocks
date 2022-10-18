/*
Copyright 2022 The KubeBlocks Authors

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

	"github.com/apecloud/kubeblocks/controllers/k8score"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func init() {
	clusterDefUpdateHandlers["cluster"] = clusterUpdateHandler
	k8score.EventHandlerMap["cluster-controller"] = &ClusterReconciler{}
}

func clusterUpdateHandler(cli client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error {

	labelSelector, err := labels.Parse("clusterdefinition.infracreate.com/name=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &dbaasv1alpha1.ClusterList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != clusterDef.GetObjectMeta().GetGeneration() {
			patch := client.MergeFrom(item.DeepCopy())
			// sync status.Operations.HorizontalScalable
			horizontalScalableComponents, _ := getSupportHorizontalScalingComponents(&item, clusterDef)
			item.Status.Operations.HorizontalScalable = horizontalScalableComponents
			item.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
			if err = cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
}

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// TODO probeMessage should be defined by @xuanchi
type probeMessage struct {
	Data probeMessageData `json:"data,omitempty"`
}

type probeMessageData struct {
	Role string `json:"role,omitempty"`
}

func (r *ClusterReconciler) Handle(cli client.Client, ctx context.Context, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != k8score.ProbeRoleChangedCheckPath {
		return nil
	}

	// get role
	message := &probeMessage{}
	err := json.Unmarshal([]byte(event.Message), message)
	if err != nil {
		return err
	}
	role := strings.ToLower(message.Data.Role)

	// get pod
	pod := &corev1.Pod{}
	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}

	// update label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[consensusSetRoleLabelKey] = role
	err = cli.Patch(ctx, pod, patch)
	if err != nil {
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers;statefulsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update
// NOTES: owned K8s core API resources controller-gen RBAC marker is maintained at {REPO}/controllers/k8score/rbac.go

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
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
	}

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
		if r.needCheckClusterForReady(cluster) {
			if ok, err := r.checkClusterIsReady(reqCtx.Ctx, cluster); !ok || err != nil {
				return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "checkClusterIsReady")
			}
			if err = r.patchClusterToRunning(reqCtx.Ctx, cluster); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			}
		}
		return intctrlutil.Reconciled()
	}

	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.ClusterDefRef,
	}, clusterdefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	appversion := &dbaasv1alpha1.AppVersion{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.AppVersionRef,
	}, appversion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, cluster, appversion.Status.Phase, dbaasv1alpha1.AppVersionKind); res != nil {
		return *res, err
	}

	if res, err = r.checkReferencedCRStatus(reqCtx, cluster, clusterdefinition.Status.Phase, dbaasv1alpha1.ClusterDefinitionKind); res != nil {
		return *res, err
	}

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
	if err = task.Exec(reqCtx.Ctx, r.Client); err != nil {
		// record the event when the execution task reports an error.
		r.Recorder.Event(cluster, corev1.EventTypeWarning, "RunTaskFailed", err.Error())
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
		appInstanceLabelKey: cluster.GetName(),
		appNameLabelKey:     fmt.Sprintf("%s-%s", clusterDef.Spec.Type, clusterDef.Name),
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
	for _, component := range clusterDef.Spec.Components {
		ml := client.MatchingLabels{
			appInstanceLabelKey: fmt.Sprintf("%s-%s", cluster.GetName(), component.TypeName),
			appNameLabelKey:     fmt.Sprintf("%s-%s", clusterDef.Spec.Type, clusterDef.Name),
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
	cluster.Status.Message = fmt.Sprintf("%s.status.phase is not Available, this problem needs to be solved first", crKind)
	if err := r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
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

// checkClusterIsReady Check whether the cluster related pod resources are running. if ok, update Cluster.status.phase to Running
func (r *ClusterReconciler) checkClusterIsReady(ctx context.Context, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		statefulSetList         = &appsv1.StatefulSetList{}
		isOk                    = true
		needSyncStatusComponent bool
	)
	if err := r.Client.List(ctx, statefulSetList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels{appInstanceLabelKey: cluster.Name}); err != nil {
		return false, err
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	for _, v := range statefulSetList.Items {
		// check whether the statefulset has reached the final state
		if v.Status.AvailableReplicas != *v.Spec.Replicas ||
			v.Status.CurrentRevision != v.Status.UpdateRevision ||
			v.Status.ObservedGeneration != v.GetGeneration() {
			isOk = false
		}
		// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
		if ok := r.patchStatusComponentsWithStatefulSet(cluster, &v, cluster.Status.Phase); ok {
			needSyncStatusComponent = true
		}

		// if v is consensusSet
		typeName := getComponentTypeName(*cluster, v)
		componentDef, err := getComponent(ctx, r.Client, cluster, typeName)
		if err != nil {
			return false, err
		}

		switch componentDef.ComponentType {
		case dbaasv1alpha1.Consensus:
			end, err := handleConsensusSetUpdate(ctx, r.Client, cluster, &v)
			if err != nil {
				return false, err
			}
			if !end {
				isOk = false
			}
		case dbaasv1alpha1.Stateful:
			// TODO wait other component type added
		}
	}

	if needSyncStatusComponent {
		if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
			return false, err
		}
	}

	return isOk, nil
}

// patchStatusComponentsWithStatefulSet  Modify status.components information, include component phase
func (r *ClusterReconciler) patchStatusComponentsWithStatefulSet(cluster *dbaasv1alpha1.Cluster, statefulSet *appsv1.StatefulSet, phase dbaasv1alpha1.Phase) bool {
	var (
		cName           string
		ok              bool
		statusComponent *dbaasv1alpha1.ClusterStatusComponent
	)
	//  if it does not belong to this component, return false
	if cName, ok = statefulSet.Labels[appComponentLabelKey]; !ok {
		return false
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = cluster.Status.Components[cName]; !ok {
		cluster.Status.Components[cName] = &dbaasv1alpha1.ClusterStatusComponent{Phase: phase}
		return true
	} else if statusComponent.Phase != phase {
		statusComponent.Phase = phase
		return true
	}
	return false
}

// patchClusterToRunning patch Cluster.status.phase to Running
func (r *ClusterReconciler) patchClusterToRunning(ctx context.Context, cluster *dbaasv1alpha1.Cluster) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	if err := r.Client.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	// send an event when Cluster.status.phase change to Running
	r.Recorder.Eventf(cluster, corev1.EventTypeNormal, string(dbaasv1alpha1.RunningPhase), "Cluster: %s is ready, current phase is Running", cluster.Name)
	return nil
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
