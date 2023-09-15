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
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ComponentDefinitionReconciler reconciles a ComponentDefinition object
type ComponentDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ComponentDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("component", req.NamespacedName),
		Recorder: r.Recorder,
	}

	rctx.Log.V(1).Info("reconcile", "component", req.NamespacedName)

	cmpd := &appsv1alpha1.ComponentDefinition{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, cmpd); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	return r.reconcile(rctx, cmpd)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ComponentDefinition{}).
		Complete(r)
}

func (r *ComponentDefinitionReconciler) reconcile(rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, cmpd, componentDefinitionFinalizerName, r.deletionHandler(rctx, cmpd))
	if res != nil {
		return *res, err
	}

	if cmpd.Status.ObservedGeneration == cmpd.Generation &&
		slices.Contains([]appsv1alpha1.Phase{appsv1alpha1.AvailablePhase}, cmpd.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err = r.validate(r.Client, rctx, cmpd); err != nil {
		if err1 := r.unavailable(r.Client, rctx, cmpd, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	err = r.available(r.Client, rctx, cmpd)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, cmpd)

	return intctrlutil.Reconciled()
}

func (r *ComponentDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(cmpd, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, cmpd, constant.ComponentDefinitionLabelKey,
			recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ComponentDefinitionReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return r.status(cli, rctx, cmpd, appsv1alpha1.AvailablePhase, "")
}

func (r *ComponentDefinitionReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition, err error) error {
	return r.status(cli, rctx, cmpd, appsv1alpha1.UnavailablePhase, err.Error())
}

func (r *ComponentDefinitionReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition, phase appsv1alpha1.Phase, message string) error {
	patch := client.MergeFrom(cmpd.DeepCopy())
	cmpd.Status.ObservedGeneration = cmpd.Generation
	cmpd.Status.Phase = phase
	cmpd.Status.Message = message
	return cli.Status().Patch(rctx.Ctx, cmpd, patch)
}

func (r *ComponentDefinitionReconciler) validate(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	for _, validator := range []func(client.Client, intctrlutil.RequestCtx, *appsv1alpha1.ComponentDefinition) error{
		r.validateRuntime,
		r.validateVolumes,
		r.validateServices,
		r.validateConfigs,
		r.validateLogConfigs,
		r.validateMonitor,
		r.validateScripts,
		r.validateConnectionCredentials,
		r.validatePolicyRules,
		r.validateLabels,
		r.validateSystemAccounts,
		r.validateUpdateStrategy,
		r.validateRoles,
		r.validateRoleArbitrator,
		r.validateLifecycleActions,
		r.validateComponentDefRef,
	} {
		if err := validator(cli, rctx, cmpd); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateRuntime(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateVolumes(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	hasVolumeToProtect := false
	for _, vol := range cmpd.Spec.Volumes {
		if vol.HighWatermark > 0 && vol.HighWatermark < 100 {
			hasVolumeToProtect = true
			break
		}
	}
	if hasVolumeToProtect {
		if cmpd.Spec.LifecycleActions.Readonly == nil || cmpd.Spec.LifecycleActions.Readwrite == nil {
			return fmt.Errorf("the Readonly and Readwrite actions are needed to protect volumes")
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateServices(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateConfigs(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	// if err := appsconfig.ReconcileConfigSpecsForReferencedCR(r.Client, rctx, dbClusterDef); err != nil {
	//	return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	// }
	return nil
}

func (r *ComponentDefinitionReconciler) validateLogConfigs(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateMonitor(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateScripts(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateConnectionCredentials(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	for _, cc := range cmpd.Spec.ConnectionCredentials {
		if err := r.validateConnectionCredential(cli, rctx, cmpd, cc); err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateConnectionCredential(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition, cc appsv1alpha1.ConnectionCredential) error {
	if err := r.validateConnectionCredentialService(cmpd, cc); err != nil {
		return err
	}
	if err := r.validateConnectionCredentialAccount(cmpd, cc); err != nil {
		return err
	}
	return nil
}

func (r *ComponentDefinitionReconciler) validateConnectionCredentialService(cmpd *appsv1alpha1.ComponentDefinition,
	cc appsv1alpha1.ConnectionCredential) error {
	if len(cc.ServiceName) == 0 && !cc.HeadlessService {
		return fmt.Errorf("nether service name nor headless service is defined for connection credential: %s", cc.Name)
	}
	if len(cmpd.Spec.Services) == 0 {
		return fmt.Errorf("there is no service defined which is needed by connection credentials")
	}

	if cc.HeadlessService {
		// TODO: other headless service ports
		return r.validateConnectionCredentialPort(cmpd, cc, cmpd.Spec.Services[0].Ports)
	}
	for _, svc := range cmpd.Spec.Services {
		if svc.Name == cc.ServiceName {
			return r.validateConnectionCredentialPort(cmpd, cc, svc.Ports)
		}
	}
	return fmt.Errorf("there is no matched service for connection credential: %s", cc.Name)
}

func (r *ComponentDefinitionReconciler) validateConnectionCredentialPort(cmpd *appsv1alpha1.ComponentDefinition,
	cc appsv1alpha1.ConnectionCredential, ports []corev1.ServicePort) error {
	if len(cc.PortName) == 0 {
		switch len(ports) {
		case 0:
			return fmt.Errorf("there is no port defined for connection credential: %s", cc.Name)
		case 1:
			return nil
		default:
			return fmt.Errorf("there are multiple ports defined, it must be specified a port for connection credential: %s", cc.Name)
		}
	}
	for _, port := range ports {
		if port.Name == cc.PortName {
			return nil
		}
	}
	return fmt.Errorf("there is no matched port for connection credential: %s", cc.Name)
}

func (r *ComponentDefinitionReconciler) validateConnectionCredentialAccount(cmpd *appsv1alpha1.ComponentDefinition,
	cc appsv1alpha1.ConnectionCredential) error {
	if len(cc.AccountName) == 0 {
		return nil
	}
	if cmpd.Spec.SystemAccounts == nil {
		return fmt.Errorf("there is no account defined for connection credential: %s", cc.Name)
	}
	for _, account := range cmpd.Spec.SystemAccounts.Accounts {
		if string(account.Name) == cc.AccountName {
			return nil
		}
	}
	return fmt.Errorf("there is no matched account for connection credential: %s", cc.Name)
}

func (r *ComponentDefinitionReconciler) validatePolicyRules(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateLabels(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateSystemAccounts(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	if cmpd.Spec.SystemAccounts == nil {
		return nil
	}
	if len(cmpd.Spec.SystemAccounts.Accounts) == 0 {
		return nil
	}
	if cmpd.Spec.LifecycleActions.AccountProvision != nil {
		return nil
	}
	return fmt.Errorf("the AccountProvision action is needed to provision system accounts")
}

func (r *ComponentDefinitionReconciler) validateUpdateStrategy(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateRoles(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateRoleArbitrator(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateLifecycleActions(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}

func (r *ComponentDefinitionReconciler) validateComponentDefRef(cli client.Client, rctx intctrlutil.RequestCtx,
	cmpd *appsv1alpha1.ComponentDefinition) error {
	return nil
}
