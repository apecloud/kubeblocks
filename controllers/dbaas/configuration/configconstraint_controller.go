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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ConfigConstraintReconciler reconciles a ConfigConstraint object
type ConfigConstraintReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configconstraint,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configconstraint/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configconstraint/finalizers,verbs=update

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
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	configCSTR := &dbaasv1alpha1.ConfigConstraint{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, configCSTR); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, configCSTR, cfgcore.ConfigurationTemplateFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(configCSTR, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing ClusterDefinition or ClusterVersion.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, configCSTR,
			cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(configCSTR.GetName()),
			recordEvent, &dbaasv1alpha1.ClusterDefinitionList{},
			&dbaasv1alpha1.ClusterVersionList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if configCSTR.Status.ObservedGeneration == configCSTR.GetObjectMeta().GetGeneration() {
		return intctrlutil.Reconciled()
	}

	if ok, err := checkConfigurationTemplate(reqCtx, configCSTR); !ok || err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "ValidateConfigurationTemplate")
	}

	statusPatch := client.MergeFrom(configCSTR.DeepCopy())
	// configCSTR.Spec.ConfigurationSchema.Schema = cfgcore.GenerateOpenAPISchema(configCSTR.Spec.ConfigurationSchema.CUE)
	if err := updateConfigurationSchema(&configCSTR.Spec); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to generate configuration open api schema")
	}
	configCSTR.Status.ObservedGeneration = configCSTR.GetObjectMeta().GetGeneration()
	configCSTR.Status.Phase = dbaasv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, configCSTR, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, configCSTR)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigConstraintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.ConfigConstraint{}).
		// for other resource
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
