/*
Copyright ApeCloud, Inc.

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

package apps

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterversions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterversions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterversions/finalizers,verbs=update

// ClusterVersionReconciler reconciles a ClusterVersion object
type ClusterVersionReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

func init() {
	viper.SetDefault(maxConcurReconClusterVersionKey, runtime.NumCPU()*2)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ClusterVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("ClusterVersion", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	deletionHandler := func() (*ctrl.Result, error) {
		return r.deleteExternalResources(reqCtx, clusterVersion)
	}
	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, clusterVersion, clusterVersionFinalizerName, deletionHandler); res != nil {
		return *res, err
	}

	clusterDefinition, res, err := r.getReferencedClusterDefinition(reqCtx, clusterVersion)
	if res != nil {
		return *res, err
	}

	if clusterVersion.Status.ObservedGeneration == clusterVersion.Generation &&
		clusterVersion.Status.ClusterDefGeneration == clusterDefinition.Generation {
		return intctrlutil.Reconciled()
	}

	if res, err := r.patchLabelIfNotExist(reqCtx, clusterVersion, clusterDefinition); res != nil {
		return *res, err
	}

	if err := appsconfig.ReconcileConfigurationForReferencedCR(r.Client, reqCtx, clusterVersion); err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	}

	patch := client.MergeFrom(clusterVersion.DeepCopy())
	if statusMsg := validateClusterVersion(clusterVersion, clusterDefinition); statusMsg != "" {
		clusterVersion.Status.Phase = appsv1alpha1.UnavailablePhase
		clusterVersion.Status.Message = statusMsg
	} else {
		clusterVersion.Status.Phase = appsv1alpha1.AvailablePhase
		clusterVersion.Status.Message = ""
	}
	clusterVersion.Status.ObservedGeneration = clusterVersion.Generation
	clusterVersion.Status.ClusterDefGeneration = clusterDefinition.Generation

	if err = r.Client.Status().Patch(ctx, clusterVersion, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, clusterVersion)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ClusterVersion{}).
		Watches(
			&source.Kind{Type: &appsv1alpha1.ClusterDefinition{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForClusterDefinition),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurReconClusterVersionKey),
		}).
		Complete(r)
}

func (r *ClusterVersionReconciler) findObjectsForClusterDefinition(obj client.Object) []reconcile.Request {
	labelSelector, err := labels.Parse(clusterDefLabelKey + "=" + obj.GetName())
	if err != nil {
		return []reconcile.Request{}
	}

	clusterVersions := &appsv1alpha1.ClusterVersionList{}
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     obj.GetNamespace(),
	}
	err = r.List(context.TODO(), clusterVersions, opts)
	if err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(clusterVersions.Items))
	for i, item := range clusterVersions.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}

func (r *ClusterVersionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx,
	clusterVersion *appsv1alpha1.ClusterVersion) (*ctrl.Result, error) {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	recordEvent := func() {
		r.Recorder.Event(clusterVersion, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
			"cannot be deleted because of existing referencing Cluster.")
	}
	if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, clusterVersion,
		clusterVersionLabelKey, recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
		return res, err
	}

	return nil, appsconfig.DeleteConfigMapFinalizer(r.Client, reqCtx, clusterVersion)
}

func (r *ClusterVersionReconciler) getReferencedClusterDefinition(reqCtx intctrlutil.RequestCtx,
	cv *appsv1alpha1.ClusterVersion) (*appsv1alpha1.ClusterDefinition, *ctrl.Result, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: cv.Spec.ClusterDefinitionRef}, cd); err != nil {
		if apierrors.IsNotFound(err) {
			_ = r.handleClusterDefinitionNotFound(reqCtx, cv, err.Error())
		}
		res, err := intctrlutil.RequeueWithErrorAndRecordEvent(cv, r.Recorder, err, reqCtx.Log)
		return nil, &res, err
	}
	return cd, nil, nil
}

// handleClusterDefinitionNotFound handles clusterVersion status when clusterDefinition not found.
func (r *ClusterVersionReconciler) handleClusterDefinitionNotFound(reqCtx intctrlutil.RequestCtx,
	clusterVersion *appsv1alpha1.ClusterVersion, message string) error {
	if clusterVersion.Status.Message == message {
		return nil
	}

	patch := client.MergeFrom(clusterVersion.DeepCopy())
	clusterVersion.Status.Phase = appsv1alpha1.UnavailablePhase
	clusterVersion.Status.Message = message
	return r.Client.Status().Patch(reqCtx.Ctx, clusterVersion, patch)
}

func (r *ClusterVersionReconciler) patchLabelIfNotExist(reqCtx intctrlutil.RequestCtx,
	clusterVersion *appsv1alpha1.ClusterVersion, clusterDefinition *appsv1alpha1.ClusterDefinition) (*ctrl.Result, error) {
	if clusterVersion.Labels != nil {
		if label, ok := clusterVersion.Labels[clusterDefLabelKey]; ok && label == clusterDefinition.Name {
			return nil, nil
		}
	}

	patch := client.MergeFrom(clusterVersion.DeepCopy())
	if clusterVersion.Labels == nil {
		clusterVersion.Labels = map[string]string{}
	}
	clusterVersion.Labels[clusterDefLabelKey] = clusterDefinition.Name
	if err := r.Client.Patch(reqCtx.Ctx, clusterVersion, patch); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}
	return nil, nil
}

func validateClusterVersion(clusterVersion *appsv1alpha1.ClusterVersion, clusterDef *appsv1alpha1.ClusterDefinition) string {
	notFoundComponentDefNames, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	var statusMsgs []string
	if len(notFoundComponentDefNames) > 0 {
		statusMsgs = append(statusMsgs, fmt.Sprintf("spec.componentSpecs[*].componentDefRef %v not found in ClusterDefinition.spec.componentDefs[*].name", notFoundComponentDefNames))
	} else if len(noContainersComponents) > 0 {
		statusMsgs = append(statusMsgs, fmt.Sprintf("spec.componentSpecs[*].componentDefRef %v missing spec.componentSpecs[*].containers in ClusterDefinition.spec.componentDefs[*] and ClusterVersion.spec.componentVersions[*]", noContainersComponents))
	}
	return strings.Join(statusMsgs, ";")
}
