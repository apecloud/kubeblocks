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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	// ReasonCVReady indicates the cluster version is ready for use.
	ReasonCVReady = "Ready"
	// ReasonCVInconsistent indicates the components are inconsistent between cluster version and referenced cluster definition.
	ReasonCVInconsistent = "Inconsistent"
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
	clusterDefUpdateHandlers["clusterVersion"] = clusterVersionUpdateHandler
	viper.SetDefault(maxConcurReconClusterVersionKey, runtime.NumCPU()*2)
}

func clusterVersionUpdateHandler(cli client.Client, ctx context.Context, cd *appsv1alpha1.ClusterDefinition) error {
	labelSelector, err := labels.Parse(clusterDefLabelKey + "=" + cd.GetName())
	if err != nil {
		return err
	}

	list := &appsv1alpha1.ClusterVersionList{}
	opts := &client.ListOptions{LabelSelector: labelSelector}
	if err := cli.List(ctx, list, opts); err != nil {
		return err
	}

	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != cd.Generation {
			patch := client.MergeFrom(item.DeepCopy())
			if message := validateClusterVersion(&item, cd); message != "" {
				updateCVReadyCondition(&item, metav1.ConditionFalse, ReasonCVInconsistent, message)
			} else {
				// TODO: it's not reasonable to set status as ready since there may be other failures.
				updateCVReadyCondition(&item, metav1.ConditionTrue, ReasonCVReady, "")
				item.Status.ClusterDefGeneration = cd.Generation
			}
			if err = cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
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
		Log:      log.FromContext(ctx).WithValues("clusterVersion", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	handler := func() (*ctrl.Result, error) { return r.deletionHandler(reqCtx, clusterVersion) }
	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, clusterVersion, clusterVersionFinalizerName, handler); res != nil {
		return *res, err
	}

	if clusterVersion.Status.ObservedGeneration == clusterVersion.Generation {
		return intctrlutil.Reconciled()
	}

	clusterDefinition, res, err := r.getAReconcileReferencedCR(reqCtx, clusterVersion)
	if res != nil {
		return *res, err
	}

	// TODO: errors about config template are not exposed in status.
	if err := appsconfig.ReconcileConfigurationForReferencedCR(r.Client, reqCtx, clusterVersion); err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	}

	if res, err := r.reconcileVersionContext(reqCtx, clusterVersion, clusterDefinition); res != nil {
		return *res, err
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, clusterVersion)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ClusterVersion{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurReconClusterVersionKey),
		}).
		Complete(r)
}

func (r *ClusterVersionReconciler) deletionHandler(reqCtx intctrlutil.RequestCtx,
	cv *appsv1alpha1.ClusterVersion) (*ctrl.Result, error) {
	recordEvent := func() {
		r.Recorder.Event(cv, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
			"cannot be deleted because of existing referencing Cluster.")
	}
	if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, cv, clusterVersionLabelKey, recordEvent,
		&appsv1alpha1.ClusterList{}); res != nil || err != nil {
		return res, err
	}
	return nil, appsconfig.DeleteConfigMapFinalizer(r.Client, reqCtx, cv)
}

func (r *ClusterVersionReconciler) getAReconcileReferencedCR(reqCtx intctrlutil.RequestCtx,
	cv *appsv1alpha1.ClusterVersion) (*appsv1alpha1.ClusterDefinition, *ctrl.Result, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: cv.Spec.ClusterDefinitionRef}, cd); err != nil {
		if apierrors.IsNotFound(err) {
			_ = patchCVReadyCondition(reqCtx, r.Client, cv, metav1.ConditionFalse, constant.ReasonRefCRUnavailable, err.Error())
		}
		res, err := intctrlutil.RequeueWithErrorAndRecordEvent(cv, r.Recorder, err, reqCtx.Log)
		return nil, &res, err
	}

	// the label already exists.
	if cv.ObjectMeta.Labels != nil && cv.ObjectMeta.Labels[clusterDefLabelKey] == cd.Name {
		return cd, nil, nil
	}

	patch := client.MergeFrom(cv.DeepCopy())
	if cv.ObjectMeta.Labels == nil {
		cv.ObjectMeta.Labels = map[string]string{}
	}
	cv.ObjectMeta.Labels[clusterDefLabelKey] = cd.Name
	if err := r.Client.Patch(reqCtx.Ctx, cv, patch); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return nil, &res, err
	}

	return cd, nil, nil
}

func (r *ClusterVersionReconciler) reconcileVersionContext(reqCtx intctrlutil.RequestCtx,
	cv *appsv1alpha1.ClusterVersion, cd *appsv1alpha1.ClusterDefinition) (*ctrl.Result, error) {
	patch := client.MergeFrom(cv.DeepCopy())
	if message := validateClusterVersion(cv, cd); message != "" {
		updateCVReadyCondition(cv, metav1.ConditionFalse, ReasonCVInconsistent, message)
	} else {
		updateCVReadyCondition(cv, metav1.ConditionTrue, ReasonCVReady, "")
	}

	// TODO: if the validation failed, is it reasonable to update generations?
	cv.Status.ObservedGeneration = cv.Generation
	cv.Status.ClusterDefGeneration = cd.Generation
	if err := r.Client.Status().Patch(reqCtx.Ctx, cv, patch); err != nil {
		res, err := intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		return &res, err
	}

	return nil, nil
}

func validateClusterVersion(cv *appsv1alpha1.ClusterVersion, cd *appsv1alpha1.ClusterDefinition) string {
	msgs := make([]string, 0)
	notFound, noContainers := cv.GetInconsistentComponentsInfo(cd)
	if len(notFound) > 0 {
		msgs = append(msgs, fmt.Sprintf("spec.componentSpecs[*].componentDefRef %v not found in ClusterDefinition.spec.componentDefs[*].name", notFound))
	} else if len(noContainers) > 0 {
		msgs = append(msgs, fmt.Sprintf("spec.componentSpecs[*].componentDefRef %v missing spec.componentSpecs[*].containers in ClusterDefinition.spec.componentDefs[*] and ClusterVersion.spec.componentVersions[*]", noContainers))
	}
	return strings.Join(msgs, ";")
}

func updateCVReadyCondition(cv *appsv1alpha1.ClusterVersion, status metav1.ConditionStatus, reason string, message string) bool {
	if len(cv.Status.Conditions) > 0 {
		cond := cv.Status.Conditions[0]
		if cond.Status == status && cond.Reason == reason && cond.Message == message {
			return false
		}
	}

	cond := metav1.Condition{
		Type:               appsv1alpha1.ClusterVersionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
	if len(cv.Status.Conditions) == 0 {
		cv.Status.Conditions = append(cv.Status.Conditions, cond)
	} else {
		cv.Status.Conditions[0] = cond
	}
	return true
}

func patchCVReadyCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, cv *appsv1alpha1.ClusterVersion,
	status metav1.ConditionStatus, reason string, message string) error {
	patch := client.MergeFrom(cv.DeepCopy())
	if updateCVReadyCondition(cv, status, reason, message) {
		return cli.Status().Patch(reqCtx.Ctx, cv, patch)
	}
	return nil
}
