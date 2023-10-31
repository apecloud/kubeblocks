/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package storage

import (
	"context"
	"sync"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// StorageProviderReconciler reconciles a StorageProvider object
type StorageProviderReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	mu                 sync.Mutex
	driverDependencies map[string][]string // driver name => list of provider names
}

// +kubebuilder:rbac:groups=storage.kubeblocks.io,resources=storageproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.kubeblocks.io,resources=storageproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.kubeblocks.io,resources=storageproviders/finalizers,verbs=update

// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *StorageProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("storageprovider", req.NamespacedName)
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      logger,
		Recorder: r.Recorder,
	}

	// get provider object
	provider := &storagev1alpha1.StorageProvider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to get StorageProvider")
	}

	// add dependency to CSIDrive
	r.ensureDependency(provider)

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, provider, storageFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, provider)
	})
	if res != nil {
		return *res, err
	}

	// check CSI driver if specified
	if provider.Spec.CSIDriverName != "" {
		err := r.checkCSIDriver(reqCtx, provider.Spec.CSIDriverName)
		if err != nil {
			// update status for the CSI driver check
			if updateStatusErr := r.updateStatus(reqCtx, provider, err); updateStatusErr != nil {
				return intctrlutil.CheckedRequeueWithError(updateStatusErr, reqCtx.Log,
					"failed to update status")
			}
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log,
				"failed to check CSIDriver %s", provider.Spec.CSIDriverName)
		}
	}

	// update status
	if updateStatusErr := r.updateStatus(reqCtx, provider, nil); updateStatusErr != nil {
		return intctrlutil.CheckedRequeueWithError(updateStatusErr, reqCtx.Log,
			"failed to update status")
	}

	return intctrlutil.Reconciled()
}

func (r *StorageProviderReconciler) updateStatus(reqCtx intctrlutil.RequestCtx,
	provider *storagev1alpha1.StorageProvider,
	checkErr error) error {
	var phase storagev1alpha1.StorageProviderPhase
	var cond metav1.Condition
	if checkErr == nil {
		phase = storagev1alpha1.StorageProviderReady
		cond = metav1.Condition{
			Type:               storagev1alpha1.ConditionTypeCSIDriverInstalled,
			Status:             metav1.ConditionTrue,
			Reason:             CSIDriverObjectFound,
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: provider.Generation,
		}
	} else {
		phase = storagev1alpha1.StorageProviderNotReady
		cond = metav1.Condition{
			Type:               storagev1alpha1.ConditionTypeCSIDriverInstalled,
			Status:             metav1.ConditionUnknown,
			Reason:             CheckCSIDriverFailed,
			Message:            checkErr.Error(),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: provider.Generation,
		}
	}

	if phase == provider.Status.Phase {
		return nil
	}
	patch := client.MergeFrom(provider.DeepCopy())
	provider.Status.Phase = phase
	meta.SetStatusCondition(&provider.Status.Conditions, cond)
	return r.Client.Status().Patch(reqCtx.Ctx, provider, patch)
}

func (r *StorageProviderReconciler) checkCSIDriver(reqCtx intctrlutil.RequestCtx, driverName string) error {
	// check existence of CSIDriver
	return r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: driverName}, &storagev1.CSIDriver{})
}

func (r *StorageProviderReconciler) ensureDependency(provider *storagev1alpha1.StorageProvider) {
	if provider.Spec.CSIDriverName == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.driverDependencies == nil {
		r.driverDependencies = make(map[string][]string)
	}
	driverName := provider.Spec.CSIDriverName
	list := r.driverDependencies[driverName]
	for _, x := range list {
		if x == provider.Name {
			return
		}
	}
	r.driverDependencies[driverName] = append(list, provider.Name)
}

func (r *StorageProviderReconciler) removeDependency(provider *storagev1alpha1.StorageProvider) {
	if provider.Spec.CSIDriverName == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.driverDependencies[provider.Spec.CSIDriverName]
	for i, x := range list {
		if x == provider.Name {
			list[i] = list[len(list)-1]
			r.driverDependencies[provider.Spec.CSIDriverName] = list[:len(list)-1]
			return
		}
	}
}

func (r *StorageProviderReconciler) deleteExternalResources(
	reqCtx intctrlutil.RequestCtx, provider *storagev1alpha1.StorageProvider) error {
	r.removeDependency(provider)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.StorageProvider{}).
		Watches(&storagev1.CSIDriver{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
				r.mu.Lock()
				defer r.mu.Unlock()
				driverName := object.GetName()
				list := r.driverDependencies[driverName]
				var ret []reconcile.Request
				for _, x := range list {
					ret = append(ret, reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name: x,
						},
					})
				}
				return ret
			})).
		Complete(r)
}
