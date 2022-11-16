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

package k8score

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StorageClassReconciler reconciles a StorageClass object
type StorageClassReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *StorageClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("StorageClass", req.NamespacedName),
	}

	reqCtx.Log.V(1).Info("StorageClass watcher")

	storageClass := &storagev1.StorageClass{}
	if err := r.Client.Get(ctx, req.NamespacedName, storageClass); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "getStorageClassError")
	}

	if err := r.handleClusterVolumeExpansion(reqCtx, storageClass); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "handleErrorWhenStorageClassChanged")
	}

	return intctrlutil.Reconciled()
}

// handleClusterVolumeExpansion when StorageClass changed, we should handle the PVC of cluster whether volume expansion is supported
func (r *StorageClassReconciler) handleClusterVolumeExpansion(reqCtx intctrlutil.RequestCtx, storageClass *storagev1.StorageClass) error {
	var err error
	clusterList := &dbaasv1alpha1.ClusterList{}
	if err = r.Client.List(reqCtx.Ctx, clusterList); err != nil {
		return err
	}
	// handle the created cluster
	storageCLassName := storageClass.Name
	for _, cluster := range clusterList.Items {
		// if cluster not used the StorageClass, continue
		if !clusterContainsStorageClass(&cluster, storageCLassName) {
			continue
		}
		patch := client.MergeFrom(cluster.DeepCopy())
		if needPatchClusterStatusOperations, err := r.needSyncClusterStatusOperations(reqCtx, &cluster, storageClass); err != nil {
			return err
		} else if !needPatchClusterStatusOperations {
			continue
		}
		if err = r.Client.Status().Patch(reqCtx.Ctx, &cluster, patch); err != nil {
			return err
		}
	}
	return nil
}

// needSyncClusterStatusOperations check cluster whether sync status.operations.volumeExpandable
func (r *StorageClassReconciler) needSyncClusterStatusOperations(reqCtx intctrlutil.RequestCtx, cluster *dbaasv1alpha1.Cluster, storageClass *storagev1.StorageClass) (bool, error) {
	// get cluster pvc list
	inNS := client.InNamespace(cluster.Namespace)
	ml := client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  cluster.GetName(),
		intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcList, inNS, ml); err != nil {
		return false, err
	}
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &dbaasv1alpha1.Operations{}
	}
	// if no pvc, do it
	if len(pvcList.Items) == 0 {
		return handleNoExistsPVC(reqCtx, r.Client, cluster)
	}
	var (
		needSyncStatusOperations bool
		// save the handled pvc
		handledPVCMap = map[string]struct{}{}
	)
	for _, v := range pvcList.Items {
		if *v.Spec.StorageClassName != storageClass.Name {
			continue
		}
		componentName := v.Labels[intctrlutil.AppComponentLabelKey]
		volumeClaimTemplateName := getVolumeClaimTemplateName(v.Name, cluster.Name, componentName)
		componentVolumeClaimName := fmt.Sprintf("%s-%s", componentName, volumeClaimTemplateName)
		if _, ok := handledPVCMap[componentVolumeClaimName]; ok {
			continue
		}
		// check whether volumeExpandable changed, then sync cluster.status.operations
		if needSync := needSyncClusterStatus(storageClass, componentName, volumeClaimTemplateName, cluster); needSync {
			needSyncStatusOperations = true
		}
		handledPVCMap[componentVolumeClaimName] = struct{}{}
	}
	return needSyncStatusOperations, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1.StorageClass{}).
		Complete(r)
}
