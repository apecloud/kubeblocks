/*
Copyright 2022.

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
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func init() {
	clusterDefUpdateHandlers["cluster"] = clusterUpdateHandler
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
			item.Status.ClusterDefSyncStatus = "OutOfSync"
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
			if err = r.checkClusterIsReady(reqCtx.Ctx, cluster); err != nil {
				return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "checkClusterIsReady")
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

	if res, err := r.checkReferencedCRStatus(reqCtx, cluster, appversion.Status.Phase, dbaasv1alpha1.AppVersionKind); res != nil {
		return *res, err
	}

	if res, err := r.checkReferencedCRStatus(reqCtx, cluster, clusterdefinition.Status.Phase, dbaasv1alpha1.ClusterDefinitionKind); res != nil {
		return *res, err
	}

	if err = r.updateClusterPhaseToCreatingOrUpdating(reqCtx.Ctx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	task, err := buildClusterCreationTasks(clusterdefinition, appversion, cluster)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if err = task.Exec(reqCtx.Ctx, r.Client); err != nil {
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
func (r *ClusterReconciler) updateClusterPhaseToCreatingOrUpdating(ctx context.Context, cluster *dbaasv1alpha1.Cluster) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	if cluster.Status.Phase == "" {
		cluster.Status.Phase = dbaasv1alpha1.CreatingPhase
	} else if cluster.Status.Phase != dbaasv1alpha1.CreatingPhase {
		cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
	}
	return r.Client.Status().Patch(ctx, cluster, patch)
}

// checkClusterIsReady Check whether the cluster related pod resources are running. if ok, update Cluster.status.phase to Running
func (r *ClusterReconciler) checkClusterIsReady(ctx context.Context, cluster *dbaasv1alpha1.Cluster) error {
	statefulSetList := &appsv1.StatefulSetList{}
	if err := r.Client.List(ctx, statefulSetList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{appInstanceLabelKey: cluster.Name}); err != nil {
		return err
	}
	isOk := true
	for _, v := range statefulSetList.Items {
		if v.Status.AvailableReplicas != *v.Spec.Replicas ||
			v.Status.CurrentRevision != v.Status.UpdateRevision {
			isOk = false
			break
		}
	}
	if !isOk {
		return fmt.Errorf("cluster is not ready")
	}

	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	return r.Client.Status().Patch(ctx, cluster, patch)
}
