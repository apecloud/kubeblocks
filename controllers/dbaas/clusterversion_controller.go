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

package dbaas

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusterversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusterversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.kubeblocks.io,resources=clusterversions/finalizers,verbs=update

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

func clusterVersionUpdateHandler(cli client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error {

	labelSelector, err := labels.Parse(clusterDefLabelKey + "=" + clusterDef.GetName())
	if err != nil {
		return err
	}
	o := &client.ListOptions{LabelSelector: labelSelector}

	list := &dbaasv1alpha1.ClusterVersionList{}
	if err := cli.List(ctx, list, o); err != nil {
		return err
	}
	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != clusterDef.GetObjectMeta().GetGeneration() {
			patch := client.MergeFrom(item.DeepCopy())
			if statusMsg := validateClusterVersion(&item, clusterDef); statusMsg != "" {
				item.Status.Phase = dbaasv1alpha1.UnavailablePhase
				item.Status.Message = statusMsg
			} else {
				item.Status.Phase = dbaasv1alpha1.AvailablePhase
				item.Status.Message = ""
			}
			item.Status.ClusterDefGeneration = clusterDef.Generation
			item.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
			if err = cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ClusterVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, clusterVersion, clusterVersionFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(clusterVersion, corev1.EventTypeWarning, intctrlutil.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, clusterVersion,
			clusterVersionLabelKey, recordEvent, &dbaasv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(reqCtx, clusterVersion)
	})
	if res != nil {
		// when clusterVersion deleted, sync cluster.status.operations.upgradable
		if err := r.syncClusterStatusOperationsWithUpgrade(ctx, clusterVersion); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return *res, err
	}

	if clusterVersion.Status.ObservedGeneration == clusterVersion.GetGeneration() {
		return intctrlutil.Reconciled()
	}

	if ok, err := checkClusterVersionTemplate(r.Client, reqCtx, clusterVersion); !ok || err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "configMapIsReady")
	}

	clusterdefinition := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Name: clusterVersion.Spec.ClusterDefinitionRef,
	}, clusterdefinition); err != nil {
		return intctrlutil.RequeueWithErrorAndRecordEvent(clusterVersion, r.Recorder, err, reqCtx.Log)
	}

	patch := client.MergeFrom(clusterVersion.DeepCopy())
	if clusterVersion.ObjectMeta.Labels == nil {
		clusterVersion.ObjectMeta.Labels = map[string]string{}
	}
	clusterVersion.ObjectMeta.Labels[clusterDefLabelKey] = clusterdefinition.Name
	if err = r.Client.Patch(reqCtx.Ctx, clusterVersion, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// when clusterVersion created, sync cluster.status.operations.upgradable
	if err = r.syncClusterStatusOperationsWithUpgrade(ctx, clusterVersion); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if statusMsg := validateClusterVersion(clusterVersion, clusterdefinition); statusMsg != "" {
		clusterVersion.Status.Phase = dbaasv1alpha1.UnavailablePhase
		clusterVersion.Status.Message = statusMsg
	} else {
		clusterVersion.Status.Phase = dbaasv1alpha1.AvailablePhase
		clusterVersion.Status.Message = ""
	}
	clusterVersion.Status.ClusterDefSyncStatus = dbaasv1alpha1.InSyncStatus
	clusterVersion.Status.ObservedGeneration = clusterVersion.GetGeneration()
	clusterVersion.Status.ClusterDefGeneration = clusterdefinition.GetGeneration()
	if err = r.Client.Status().Patch(ctx, clusterVersion, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, clusterVersion)
	return ctrl.Result{}, nil
}

func checkClusterVersionTemplate(client client.Client, ctx intctrlutil.RequestCtx, clusterVersion *dbaasv1alpha1.ClusterVersion) (bool, error) {
	for _, component := range clusterVersion.Spec.Components {
		if len(component.ConfigTemplateRefs) == 0 {
			continue
		}
		if ok, err := checkValidConfTpls(client, ctx, component.ConfigTemplateRefs); !ok || err != nil {
			return ok, err
		}
	}
	return true, nil
}

func validateClusterVersion(clusterVersion *dbaasv1alpha1.ClusterVersion, clusterDef *dbaasv1alpha1.ClusterDefinition) string {
	notFoundComponentTypes, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	var statusMsgs []string
	if len(notFoundComponentTypes) > 0 {
		statusMsgs = append(statusMsgs, fmt.Sprintf("spec.components[*].type %v not found in ClusterDefinition.spec.components[*].typeName", notFoundComponentTypes))
	} else if len(noContainersComponents) > 0 {
		statusMsgs = append(statusMsgs, fmt.Sprintf("spec.components[*].type %v missing spec.components[*].containers in ClusterDefinition.spec.components[*] and ClusterVersion.spec.components[*]", noContainersComponents))
	}
	return strings.Join(statusMsgs, ";")
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.ClusterVersion{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurReconClusterVersionKey),
		}).
		Complete(r)
}

func (r *ClusterVersionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, clusterVersion *dbaasv1alpha1.ClusterVersion) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return nil
}

// SyncClusterStatusOperationsWithUpgrade sync cluster status.operations.upgradable when delete or create ClusterVersion
func (r *ClusterVersionReconciler) syncClusterStatusOperationsWithUpgrade(ctx context.Context, clusterVersion *dbaasv1alpha1.ClusterVersion) error {
	var (
		clusterList        = &dbaasv1alpha1.ClusterList{}
		clusterVersionList = &dbaasv1alpha1.ClusterVersionList{}
		upgradable         bool
		err                error
	)
	// if not delete or create ClusterVersion, return
	if clusterVersion.Status.ObservedGeneration != 0 && clusterVersion.GetDeletionTimestamp().IsZero() {
		return nil
	}
	if err = r.Client.List(ctx, clusterList, client.MatchingLabels{clusterDefLabelKey: clusterVersion.Spec.ClusterDefinitionRef}); err != nil {
		return err
	}
	if err = r.Client.List(ctx, clusterVersionList, client.MatchingLabels{clusterDefLabelKey: clusterVersion.Spec.ClusterDefinitionRef}); err != nil {
		return err
	}
	if len(clusterVersionList.Items) > 1 {
		upgradable = true
	}
	for _, v := range clusterList.Items {
		var patch client.Patch
		if v.Status.Operations != nil {
			if v.Status.Operations.Upgradable == upgradable {
				continue
			}
			patch = client.MergeFrom(v.DeepCopy())
		} else {
			patch = client.MergeFrom(v.DeepCopy())
			v.Status.Operations = &dbaasv1alpha1.Operations{}
		}
		v.Status.Operations.Upgradable = upgradable
		if err = r.Client.Status().Patch(ctx, &v, patch); err != nil {
			return err
		}
	}
	return nil
}
