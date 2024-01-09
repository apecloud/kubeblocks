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
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, clusterVersion, clusterVersionFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(clusterVersion, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, clusterVersion,
			constant.ClusterVerLabelKey, recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(reqCtx, clusterVersion)
	})
	if res != nil {
		return *res, err
	}

	if clusterVersion.Status.ObservedGeneration == clusterVersion.Generation &&
		slices.Contains(clusterVersion.Status.GetTerminalPhases(), clusterVersion.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	clusterdefinition := &appsv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Name: clusterVersion.Spec.ClusterDefinitionRef,
	}, clusterdefinition); err != nil {
		if apierrors.IsNotFound(err) {
			if res, patchErr := r.patchClusterDefLabel(reqCtx, clusterVersion); res != nil {
				return *res, patchErr
			}
			if err = r.handleClusterDefNotFound(reqCtx, clusterVersion, err.Error()); err != nil {
				return intctrlutil.RequeueWithErrorAndRecordEvent(clusterVersion, r.Recorder, err, reqCtx.Log)
			}
			return intctrlutil.Reconciled()
		}
		return intctrlutil.RequeueWithErrorAndRecordEvent(clusterVersion, r.Recorder, err, reqCtx.Log)
	}

	patchStatus := func(phase appsv1alpha1.Phase, message string) error {
		patch := client.MergeFrom(clusterVersion.DeepCopy())
		clusterVersion.Status.Phase = phase
		clusterVersion.Status.Message = message
		clusterVersion.Status.ObservedGeneration = clusterVersion.Generation
		clusterVersion.Status.ClusterDefGeneration = clusterdefinition.Generation
		return r.Client.Status().Patch(ctx, clusterVersion, patch)
	}

	if statusMsg := validateClusterVersion(clusterVersion, clusterdefinition); statusMsg != "" {
		if err := patchStatus(appsv1alpha1.UnavailablePhase, statusMsg); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	if err = appsconfig.ReconcileConfigSpecsForReferencedCR(r.Client, reqCtx, clusterVersion); err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	}

	if res, err = r.patchClusterDefLabel(reqCtx, clusterVersion); res != nil {
		return *res, err
	}

	if err = patchStatus(appsv1alpha1.AvailablePhase, ""); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, clusterVersion)
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ClusterVersion{}).
		Complete(r)
}

func (r *ClusterVersionReconciler) patchClusterDefLabel(reqCtx intctrlutil.RequestCtx,
	clusterVersion *appsv1alpha1.ClusterVersion) (*ctrl.Result, error) {
	if v, ok := clusterVersion.ObjectMeta.Labels[constant.ClusterDefLabelKey]; !ok || v != clusterVersion.Spec.ClusterDefinitionRef {
		patch := client.MergeFrom(clusterVersion.DeepCopy())
		if clusterVersion.ObjectMeta.Labels == nil {
			clusterVersion.ObjectMeta.Labels = map[string]string{}
		}
		clusterVersion.ObjectMeta.Labels[constant.ClusterDefLabelKey] = clusterVersion.Spec.ClusterDefinitionRef
		if err := r.Client.Patch(reqCtx.Ctx, clusterVersion, patch); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	return nil, nil
}

// handleClusterDefNotFound handles clusterVersion status when clusterDefinition not found.
func (r *ClusterVersionReconciler) handleClusterDefNotFound(reqCtx intctrlutil.RequestCtx,
	clusterVersion *appsv1alpha1.ClusterVersion, message string) error {
	if clusterVersion.Status.Message == message {
		return nil
	}
	patch := client.MergeFrom(clusterVersion.DeepCopy())
	clusterVersion.Status.Phase = appsv1alpha1.UnavailablePhase
	clusterVersion.Status.Message = message
	return r.Client.Status().Patch(reqCtx.Ctx, clusterVersion, patch)
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

func (r *ClusterVersionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, clusterVersion *appsv1alpha1.ClusterVersion) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return appsconfig.DeleteConfigMapFinalizer(r.Client, reqCtx, clusterVersion)
}

func clusterVersionUpdateHandler(cli client.Client, ctx context.Context, clusterDef *appsv1alpha1.ClusterDefinition) error {
	labelSelector, err := labels.Parse(constant.ClusterDefLabelKey + "=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &appsv1alpha1.ClusterVersionList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != clusterDef.Generation {
			patch := client.MergeFrom(item.DeepCopy())
			if statusMsg := validateClusterVersion(&item, clusterDef); statusMsg != "" {
				item.Status.Phase = appsv1alpha1.UnavailablePhase
				item.Status.Message = statusMsg
			} else {
				item.Status.Phase = appsv1alpha1.AvailablePhase
				item.Status.Message = ""
				item.Status.ClusterDefGeneration = clusterDef.Generation
			}
			if err = cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
}
