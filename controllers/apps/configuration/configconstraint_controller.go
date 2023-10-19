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

package configuration

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ConfigConstraintReconciler reconciles a ConfigConstraint object
type ConfigConstraintReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configconstraints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=configconstraints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigConstraint object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ConfigConstraintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithName("ConfigConstraintReconcile").WithValues("ConfigConstraint", req.NamespacedName.Name),
		Recorder: r.Recorder,
	}

	configConstraint := &appsv1alpha1.ConfigConstraint{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, configConstraint); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, configConstraint, constant.ConfigurationTemplateFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(configConstraint, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing of ClusterDefinition or ClusterVersion.")
		}
		if configConstraint.Status.Phase != appsv1alpha1.CCDeletingPhase {
			err := updateConfigConstraintStatus(r.Client, reqCtx, configConstraint, appsv1alpha1.CCDeletingPhase)
			// if fail to update ConfigConstraint status, return error,
			// so that it can be retried
			if err != nil {
				return nil, err
			}
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, configConstraint,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(configConstraint.GetName()),
			recordEvent, &appsv1alpha1.ClusterDefinitionList{},
			&appsv1alpha1.ClusterVersionList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if configConstraint.Status.ObservedGeneration == configConstraint.Generation && configConstraint.Status.IsConfigConstraintTerminalPhases() {
		return intctrlutil.Reconciled()
	}

	if ok, err := checkConfigConstraint(reqCtx, configConstraint); !ok || err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "ValidateConfigurationTemplate")
	}

	// Automatically convert cue to openAPISchema.
	if err := updateConfigSchema(configConstraint, r.Client, ctx); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to generate openAPISchema")
	}

	err = updateConfigConstraintStatus(r.Client, reqCtx, configConstraint, appsv1alpha1.CCAvailablePhase)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, configConstraint)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigConstraintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ConfigConstraint{}).
		// for other resource
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
