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

package apps

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ServiceDescriptorReconciler reconciles a ServiceDescriptor object
type ServiceDescriptorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=servicedescriptors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=servicedescriptors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=servicedescriptors/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceDescriptor object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ServiceDescriptorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("serviceDescriptor", req.NamespacedName),
		Recorder: r.Recorder,
	}

	serviceDescriptor := &appsv1alpha1.ServiceDescriptor{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, serviceDescriptor); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, serviceDescriptor, constant.ServiceDescriptorFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(serviceDescriptor, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing service referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, serviceDescriptor,
			constant.ServiceDescriptorNameLabelKey, recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if serviceDescriptor.Status.ObservedGeneration == serviceDescriptor.Generation &&
		slices.Contains(serviceDescriptor.Status.GetTerminalPhases(), serviceDescriptor.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err := r.checkServiceDescriptor(reqCtx, serviceDescriptor); err != nil {
		if err := r.updateServiceDescriptorStatus(r.Client, reqCtx, serviceDescriptor, appsv1alpha1.UnavailablePhase); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "InvalidServiceDescriptor update unavailable status failed")
		}
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "InvalidServiceDescriptor")
	}

	err = r.updateServiceDescriptorStatus(r.Client, reqCtx, serviceDescriptor, appsv1alpha1.AvailablePhase)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, serviceDescriptor)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceDescriptorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ServiceDescriptor{}).
		Complete(r)
}

// checkServiceDescriptor checks if the service descriptor is valid.
func (r *ServiceDescriptorReconciler) checkServiceDescriptor(reqCtx intctrlutil.RequestCtx, serviceDescriptor *appsv1alpha1.ServiceDescriptor) error {
	secretRefExistFn := func(envFrom *corev1.EnvVarSource) bool {
		if envFrom == nil || envFrom.SecretKeyRef == nil {
			return true
		}
		secret := &corev1.Secret{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace, Name: envFrom.SecretKeyRef.Name}, secret); err != nil {
			return false
		}
		// TODO: check secret data key exist
		return true
	}

	if serviceDescriptor.Spec.ServiceKind == "" {
		return fmt.Errorf("serviceDescriptor %s serviceKind is empty", serviceDescriptor.Name)
	}

	if serviceDescriptor.Spec.ServiceVersion == "" {
		return fmt.Errorf("serviceDescriptor %s serviceVersion is empty", serviceDescriptor.Name)
	}

	if serviceDescriptor.Spec.Endpoint != nil && !secretRefExistFn(serviceDescriptor.Spec.Endpoint.ValueFrom) {
		return fmt.Errorf("endpoint.valueFrom.secretRef %s not found", serviceDescriptor.Spec.Endpoint.ValueFrom.SecretKeyRef.Name)
	}

	if serviceDescriptor.Spec.Auth != nil {
		if serviceDescriptor.Spec.Auth.Username != nil && !secretRefExistFn(serviceDescriptor.Spec.Auth.Username.ValueFrom) {
			return fmt.Errorf("auth.username.valueFrom.secretRef %s not found", serviceDescriptor.Spec.Auth.Username.ValueFrom.SecretKeyRef.Name)
		}
		if serviceDescriptor.Spec.Auth.Password != nil && !secretRefExistFn(serviceDescriptor.Spec.Auth.Password.ValueFrom) {
			return fmt.Errorf("auth.Password.valueFrom.secretRef %s not found", serviceDescriptor.Spec.Auth.Password.ValueFrom.SecretKeyRef.Name)
		}
	}

	if serviceDescriptor.Spec.Port != nil && !secretRefExistFn(serviceDescriptor.Spec.Port.ValueFrom) {
		return fmt.Errorf("port.valueFrom.secretRef %s not found", serviceDescriptor.Spec.Port.ValueFrom.SecretKeyRef.Name)
	}
	return nil
}

// updateServiceDescriptorStatus updates the status of the service descriptor.
func (r *ServiceDescriptorReconciler) updateServiceDescriptorStatus(cli client.Client, ctx intctrlutil.RequestCtx, serviceDescriptor *appsv1alpha1.ServiceDescriptor, phase appsv1alpha1.Phase) error {
	patch := client.MergeFrom(serviceDescriptor.DeepCopy())
	serviceDescriptor.Status.Phase = phase
	serviceDescriptor.Status.ObservedGeneration = serviceDescriptor.Generation
	return cli.Status().Patch(ctx.Ctx, serviceDescriptor, patch)
}
