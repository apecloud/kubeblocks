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

package statefulset

import (
	"context"
	"encoding/json"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StatefulSetReconciler reconciles an Event object
type StatefulSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// NOTES: controller-gen RBAC marker is maintained at rbac.go

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("statefulSet", req.NamespacedName),
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, sts); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.checkStatefulSetStatusAndSyncCluster(reqCtx, sts); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// checkStatefulSetStatusAndSyncCluster check statefulSet status and sync cluster status when the status changed
func (r *StatefulSetReconciler) checkStatefulSetStatusAndSyncCluster(reqCtx intctrlutil.RequestCtx, sts *appsv1.StatefulSet) error {
	var (
		statefulStatusRevisionIsEquals bool
		componentIsRunning             = true
		err                            error
		cluster                        = &dbaasv1alpha1.Cluster{}
		labels                         = sts.GetLabels()
	)

	if labels == nil {
		return nil
	}
	if err = r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: labels[appInstanceLabelKey], Namespace: sts.Namespace}, cluster); err != nil {
		return err
	}

	// handle update operations by component type. when statefulSet is ok, statefulStatusRevisionIsEquals will be true
	if statefulStatusRevisionIsEquals, err = r.handleUpdateByComponentType(reqCtx, sts, cluster); err != nil {
		return err
	}

	// judge whether statefulSet is ready
	if sts.Status.AvailableReplicas != *sts.Spec.Replicas ||
		sts.Status.ObservedGeneration != sts.GetGeneration() ||
		!statefulStatusRevisionIsEquals {
		componentIsRunning = false
	}
	// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
	patch := client.MergeFrom(cluster.DeepCopy())
	componentName := labels[appComponentLabelKey]
	if ok := r.needSyncStatusComponents(cluster, componentName, componentIsRunning); !ok {
		return nil
	}
	reqCtx.Log.Info("component phase changed", "componentName", componentName, "phase", cluster.Status.Components[componentName].Phase)
	if err = r.Client.Status().Patch(reqCtx.Ctx, cluster, patch); err != nil {
		return err
	}
	return r.markRunningOpsRequestAnnotation(reqCtx, cluster)
}

// handleUpdateByComponentType handle cluster update operations according to component type and check statefulSet revision
func (r *StatefulSetReconciler) handleUpdateByComponentType(reqCtx intctrlutil.RequestCtx, sts *appsv1.StatefulSet, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		componentDef                   *dbaasv1alpha1.ClusterDefinitionComponent
		err                            error
		statefulStatusRevisionIsEquals bool
		labels                         = sts.GetLabels()
	)
	for _, v := range cluster.Spec.Components {
		if v.Name != labels[appComponentLabelKey] {
			continue
		}
		if componentDef, err = GetComponentFromClusterDefinition(reqCtx.Ctx, r.Client, cluster, v.Type); err != nil || componentDef == nil {
			return false, err
		}
		switch componentDef.ComponentType {
		case dbaasv1alpha1.Consensus:
			// Consensus do not judge whether the revisions are consistent
			if statefulStatusRevisionIsEquals, err = handleConsensusSetUpdate(reqCtx.Ctx, r.Client, cluster, sts); err != nil {
				return false, err
			}
		case dbaasv1alpha1.Stateful:
			// when stateful updateStrategy is rollingUpdate, need to check revision
			if sts.Status.UpdateRevision == sts.Status.CurrentRevision {
				statefulStatusRevisionIsEquals = true
			}
		}
		break
	}
	return statefulStatusRevisionIsEquals, err
}

// needSyncStatusComponents Determine whether the component status needs to be modified
func (r *StatefulSetReconciler) needSyncStatusComponents(cluster *dbaasv1alpha1.Cluster, componentName string, componentIsRunning bool) bool {
	var (
		ok              bool
		statusComponent *dbaasv1alpha1.ClusterStatusComponent
	)
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = cluster.Status.Components[componentName]; !ok {
		cluster.Status.Components[componentName] = &dbaasv1alpha1.ClusterStatusComponent{Phase: cluster.Status.Phase}
		return true
	}
	// if componentIsRunning is false, means the cluster has an operation running.
	// so we sync the cluster phase to component phase when cluster phase is not Running.
	if cluster.Status.Phase != dbaasv1alpha1.RunningPhase && !componentIsRunning && statusComponent.Phase == dbaasv1alpha1.RunningPhase {
		statusComponent.Phase = cluster.Status.Phase
		return true
	}
	// if componentIsRunning is true and component status is not Running.
	// we should change component phase to Running
	if statusComponent.Phase != dbaasv1alpha1.RunningPhase && componentIsRunning {
		statusComponent.Phase = dbaasv1alpha1.RunningPhase
		return true
	}
	return false
}

// markRunningOpsRequestAnnotation mark reconcile annotation to the OpsRequest running in the current cluster.
// then the related OpsRequest can reconcile
func (r *StatefulSetReconciler) markRunningOpsRequestAnnotation(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster) error {
	var (
		opsRequestMap   map[dbaasv1alpha1.Phase]string
		opsRequestValue string
		ok              bool
		err             error
	)
	if cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = cluster.Annotations[OpsRequestAnnotationKey]; !ok {
		return nil
	}
	if err = json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return err
	}
	// mark annotation for updating operations
	for k, v := range opsRequestMap {
		if k != dbaasv1alpha1.UpdatingPhase {
			continue
		}
		if err = r.patchOpsRequestAnnotation(reqCtx, cluster, v); err != nil {
			return err
		}
	}
	return nil
}

// patchOpsRequestAnnotation patch the reconcile annotation to OpsRequest
func (r *StatefulSetReconciler) patchOpsRequestAnnotation(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster, opsRequestName string) error {
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: opsRequestName, Namespace: cluster.Namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	opsRequest.Annotations[OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339)
	return r.Client.Patch(reqCtx.Ctx, opsRequest, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(filterLabels)).
		Complete(r)
}

// filterLabels filter the resources according to labels
func filterLabels(object client.Object) bool {
	matchLabels := []string{appInstanceLabelKey, appComponentLabelKey}
	objLabels := object.GetLabels()
	if objLabels == nil {
		return false
	}
	for _, l := range matchLabels {
		if _, ok := objLabels[l]; !ok {
			return false
		}
	}
	return true
}
