/*
Copyright 2022.

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

package dbaas

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func init() {
	clusterDefUpdateHandlers["appVersion"] = appVersionUpdateHandler
}

func appVersionUpdateHandler(cli client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error {

	labelSelector, err := labels.Parse("clusterdefinition.infracreate.com/name=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &dbaasv1alpha1.AppVersionList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != clusterDef.GetObjectMeta().GetGeneration() {
			patch := client.MergeFrom(item.DeepCopy())
			notFoundComponentTypes, noContainersComponents := item.GetInconsistentComponentsInfo(clusterDef)
			var statusMsgs []string
			if len(notFoundComponentTypes) > 0 {
				statusMsgs = append(statusMsgs, fmt.Sprintf("component %v not found in clusterDefinition", notFoundComponentTypes))
			}
			if len(noContainersComponents) > 0 {
				statusMsgs = append(statusMsgs, fmt.Sprintf("component %v no containers in clusterDefinition and appVersion", notFoundComponentTypes))
			}
			if len(statusMsgs) > 0 {
				item.Status.Message = strings.Join(statusMsgs, ";")
			}
			if len(notFoundComponentTypes) > 0 || len(noContainersComponents) > 0 {
				item.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
				item.Status.Phase = dbaasv1alpha1.UnAvailablePhase
			} else {
				item.Status.ClusterDefSyncStatus = dbaasv1alpha1.InSyncStatus
				item.Status.Phase = dbaasv1alpha1.AvailablePhase
				item.Status.Message = ""
			}
			item.Status.ClusterDefGeneration = clusterDef.Generation
			if err = cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
}

// AppVersionReconciler reconciles a AppVersion object
type AppVersionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=appversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=appversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=appversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *AppVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
	}

	appVersion := &dbaasv1alpha1.AppVersion{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, appVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, appVersion, appVersionFinalizerName, func() (*ctrl.Result, error) {
		statusHandler := func() error {
			patch := client.MergeFrom(appVersion.DeepCopy())
			appVersion.Status.Phase = dbaasv1alpha1.DeletingPhase
			appVersion.Status.Message = "cannot be deleted because of existing referencing Cluster."
			return r.Client.Status().Patch(ctx, appVersion, patch)
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, appVersion,
			AppVersionLabelKey, statusHandler, &dbaasv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(reqCtx, appVersion)
	})
	if res != nil {
		return *res, err
	}

	if appVersion.Status.ObservedGeneration == appVersion.GetGeneration() {
		return intctrlutil.Reconciled()
	}

	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: appVersion.Namespace,
		Name:      appVersion.Spec.ClusterDefinitionRef,
	}, clusterdefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	patch := client.MergeFrom(appVersion.DeepCopy())
	if appVersion.ObjectMeta.Labels == nil {
		appVersion.ObjectMeta.Labels = map[string]string{}
	}
	appVersion.ObjectMeta.Labels[clusterDefLabelKey] = clusterdefinition.Name
	if err = r.Client.Patch(reqCtx.Ctx, appVersion, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	appVersion.Status.ClusterDefSyncStatus = dbaasv1alpha1.InSyncStatus
	appVersion.Status.Phase = dbaasv1alpha1.AvailablePhase
	appVersion.Status.ObservedGeneration = appVersion.GetGeneration()
	appVersion.Status.ClusterDefGeneration = clusterdefinition.GetGeneration()
	if err = r.Client.Status().Patch(ctx, appVersion, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.AppVersion{}).
		Complete(r)
}

func (r *AppVersionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, appVersion *dbaasv1alpha1.AppVersion) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return nil
}
