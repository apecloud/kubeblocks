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
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ServiceConnectionCredentialReconciler reconciles a ServiceConnectionCredential object
type ServiceConnectionCredentialReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=serviceconnectioncredentials,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=serviceconnectioncredentials/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=serviceconnectioncredentials/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ServiceConnectionCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	serviceConnCredential := &appsv1alpha1.ServiceConnectionCredential{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, serviceConnCredential); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, serviceConnCredential, constant.ServiceConnectionCredentialFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(serviceConnCredential, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing service referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, serviceConnCredential,
			constant.ServiceConnectionCredentialNameLabelKey, recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if serviceConnCredential.Status.ObservedGeneration == serviceConnCredential.Generation &&
		slices.Contains(serviceConnCredential.Status.GetTerminalPhases(), serviceConnCredential.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err := r.checkServiceConnectionCredential(reqCtx, serviceConnCredential); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "InvalidServiceConnectionCredential")
	}

	err = r.updateServiceConnectionCredentialStatus(r.Client, reqCtx, serviceConnCredential, appsv1alpha1.AvailablePhase)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, serviceConnCredential)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceConnectionCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ServiceConnectionCredential{}).
		Complete(r)
}

// checkServiceConnectionCredential checks if the service connection credential is valid.
func (r *ServiceConnectionCredentialReconciler) checkServiceConnectionCredential(reqCtx intctrlutil.RequestCtx, serviceConnCredential *appsv1alpha1.ServiceConnectionCredential) error {
	secretRefExistFn := func(envFrom *corev1.EnvVarSource) bool {
		if envFrom == nil || envFrom.SecretKeyRef == nil {
			return true
		}
		secret := &corev1.Secret{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace, Name: envFrom.SecretKeyRef.Name}, secret); err != nil {
			return false
		}
		return true
	}

	if serviceConnCredential.Spec.Endpoint != nil {
		if !secretRefExistFn(serviceConnCredential.Spec.Endpoint.ValueFrom) {
			return fmt.Errorf("endpoint.valueFrom.secretRef %s not found", serviceConnCredential.Spec.Endpoint.ValueFrom.SecretKeyRef.Name)
		}
	}

	if serviceConnCredential.Spec.Auth != nil {
		if serviceConnCredential.Spec.Auth.Username != nil {
			if !secretRefExistFn(serviceConnCredential.Spec.Auth.Username.ValueFrom) {
				return fmt.Errorf("auth.username.valueFrom.secretRef %s not found", serviceConnCredential.Spec.Auth.Username.ValueFrom.SecretKeyRef.Name)
			}
		}
		if serviceConnCredential.Spec.Auth.Password != nil {
			if !secretRefExistFn(serviceConnCredential.Spec.Auth.Password.ValueFrom) {
				return fmt.Errorf("auth.Password.valueFrom.secretRef %s not found", serviceConnCredential.Spec.Auth.Password.ValueFrom.SecretKeyRef.Name)
			}
		}
	}

	if serviceConnCredential.Spec.Port != nil {
		if !secretRefExistFn(serviceConnCredential.Spec.Port.ValueFrom) {
			return fmt.Errorf("port.valueFrom.secretRef %s not found", serviceConnCredential.Spec.Port.ValueFrom.SecretKeyRef.Name)
		}
	}

	return nil
}

// updateServiceConnectionCredentialStatus updates the status of the service connection credential.
func (r *ServiceConnectionCredentialReconciler) updateServiceConnectionCredentialStatus(cli client.Client, ctx intctrlutil.RequestCtx, serviceConnCredential *appsv1alpha1.ServiceConnectionCredential, phase appsv1alpha1.Phase) error {
	patch := client.MergeFrom(serviceConnCredential.DeepCopy())
	serviceConnCredential.Status.Phase = phase
	serviceConnCredential.Status.ObservedGeneration = serviceConnCredential.Generation
	return cli.Status().Patch(ctx.Ctx, serviceConnCredential, patch)
}
