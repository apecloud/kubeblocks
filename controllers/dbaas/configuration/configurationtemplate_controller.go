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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ConfigurationTemplateReconciler reconciles a ConfigurationTemplate object
type ConfigurationTemplateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configurationtemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configurationtemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=configurationtemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ConfigurationTemplate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ConfigurationTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	configTpl := &dbaasv1alpha1.ConfigurationTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, configTpl); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, configTpl, ConfigurationTemplateFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(configTpl, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing ClusterDefinition or AppVersion.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, configTpl,
			cfgcore.GenerateUniqLabelKeyWithConfig(configTpl.GetName()),
			recordEvent, &dbaasv1alpha1.ClusterDefinitionList{},
			&dbaasv1alpha1.AppVersionList{}); res != nil || err != nil {
			return res, err
		}

		return r.deleteExternalResources(reqCtx, configTpl)
	})
	if res != nil {
		return *res, err
	}

	if configTpl.Status.ObservedGeneration == configTpl.GetObjectMeta().GetGeneration() {
		return intctrlutil.Reconciled()
	}

	if ok, err := CheckConfigurationTemplate(r.Client, reqCtx, configTpl); !ok || err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "ValidateConfigurationTemplate")
	}

	// TODO(zt) update configmap Finalizer and set Immutable
	if err := UpdateConfigMapFinalizer(r.Client, reqCtx, configTpl); err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "UpdateConfigMapFinalizer")
	}

	statusPatch := client.MergeFrom(configTpl.DeepCopy())
	// configTpl.Spec.ConfigurationSchema.Schema = cfgcore.GenerateOpenAPISchema(configTpl.Spec.ConfigurationSchema.Cue)
	if err := UpdateConfigurationSchema(&configTpl.Spec); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "failed to generate configuration open api schema")
	}
	configTpl.Status.ObservedGeneration = configTpl.GetObjectMeta().GetGeneration()
	configTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, configTpl, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, configTpl)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigurationTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.ConfigurationTemplate{}).
		// for other resource
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *ConfigurationTemplateReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, configTpl *dbaasv1alpha1.ConfigurationTemplate) (*ctrl.Result, error) {
	// TODO(zt) delete configmap Finalizer

	// delete any external resources associated with the configuration template
	labels := client.MatchingLabels{
		CMConfigurationTplNameLabelKey: configTpl.GetName(),
	}
	ns := client.InNamespace(configTpl.Namespace)

	cmList := &corev1.ConfigMapList{}
	if err := r.Client.List(reqCtx.Ctx, cmList, ns, labels); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}

	for _, cm := range cmList.Items {
		if !controllerutil.ContainsFinalizer(&cm, ConfigurationTemplateFinalizerName) {
			continue
		}
		patch := client.MergeFrom(cm.DeepCopy())
		controllerutil.RemoveFinalizer(&cm, ConfigurationTemplateFinalizerName)
		if err := r.Patch(reqCtx.Ctx, &cm, patch); err != nil {
			res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			return &res, err
		}
	}

	return nil, nil
}
