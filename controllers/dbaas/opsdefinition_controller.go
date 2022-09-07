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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func init() {
	clusterDefUpdateHandlers["opsDefinition"] = opsDefinitionUpdateHandler
}

func opsDefinitionUpdateHandler(cli client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	list := &dbaasv1alpha1.OpsDefinitionList{}
	if err := cli.List(ctx, list, client.MatchingLabels{clusterDefLabelKey: clusterDef.GetName()}); err != nil {
		return err
	}
	// clusterDefinition components to map
	componentMap := map[string]*dbaasv1alpha1.ClusterDefinitionComponent{}
	for _, v := range clusterDef.Spec.Components {
		componentMap[v.TypeName] = &v
	}
	// Automatically determine whether the opsDefinition complies with the modified constraint of clusterDefinition
	for _, item := range list.Items {
		if item.Status.ClusterDefGeneration != clusterDef.GetGeneration() {
			patch := client.MergeFrom(item.DeepCopy())
			// Determine whether to synchronize automatically
			statusMsg := make([]string, 0)
			if item.Spec.Strategy != nil && item.Spec.Strategy.Components != nil {
				for _, v := range item.Spec.Strategy.Components {
					if _, ok := componentMap[v.Type]; !ok {
						statusMsg = append(statusMsg, fmt.Sprintf("spec.strategy.components[*].type %v not found in ClusterDefinition.spec.components[*].typeName", v.Type))
						break
					}
				}
			}
			if len(statusMsg) > 0 {
				item.Status.Phase = dbaasv1alpha1.UnavailablePhase
				item.Status.Message = strings.Join(statusMsg, ";")
			} else {
				item.Status.Phase = dbaasv1alpha1.AvailablePhase
				item.Status.Message = ""
			}
			item.Status.ClusterDefSyncStatus = dbaasv1alpha1.OutOfSyncStatus
			item.Status.ClusterDefGeneration = clusterDef.GetGeneration()
			if err := cli.Status().Patch(ctx, &item, patch); err != nil {
				return err
			}
		}
	}

	return nil
}

// OpsDefinitionReconciler reconciles a OpsDefinition object
type OpsDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=opsdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=opsdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=opsdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpsDefinition object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *OpsDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("opsDefinition", req.NamespacedName),
	}

	opsDef := &dbaasv1alpha1.OpsDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, opsDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, opsDef, opsDefinitionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, opsDef)
	})
	if res != nil {
		return *res, err
	}

	if opsDef.Status.ObservedGeneration == opsDef.GetGeneration() {
		return intctrlutil.Reconciled()
	}

	patch := client.MergeFrom(opsDef.DeepCopy())
	// add label of clusterDefinitionRef
	if opsDef.Labels == nil {
		opsDef.Labels = map[string]string{}
	}
	clusterDefinitionName := opsDef.ObjectMeta.Labels[clusterDefLabelKey]
	if clusterDefinitionName != opsDef.Spec.ClusterDefinitionRef {
		opsDef.Labels[clusterDefLabelKey] = opsDef.Spec.ClusterDefinitionRef
		if err = r.Client.Patch(reqCtx.Ctx, opsDef, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	// sync observedGeneration and update status to available
	opsDef.Status.Phase = dbaasv1alpha1.AvailablePhase
	opsDef.Status.Message = ""
	opsDef.Status.ObservedGeneration = opsDef.GetGeneration()
	if err = r.Client.Status().Patch(reqCtx.Ctx, opsDef, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.OpsDefinition{}).
		Complete(r)
}

func (r *OpsDefinitionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, opsDefinition *dbaasv1alpha1.OpsDefinition) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return nil
}
