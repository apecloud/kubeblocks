/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions/finalizers,verbs=update

// ClusterDefinitionReconciler reconciles a ClusterDefinition object
type ClusterDefinitionReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	clusterDef := &appsv1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if res, err := intctrlutil.HandleCRDeletion(reqCtx, r, clusterDef,
		clusterDefinitionFinalizerName, r.deletionHandler(reqCtx, clusterDef)); res != nil {
		return *res, err
	}

	if clusterDef.Status.ObservedGeneration == clusterDef.Generation &&
		clusterDef.Status.Phase == appsv1.AvailablePhase {
		return intctrlutil.Reconciled()
	}

	if res, err := r.reconcile(reqCtx, clusterDef); res != nil {
		if err1 := r.unavailable(reqCtx, clusterDef, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, reqCtx.Log, "")
		}
		return *res, err
	}

	if err := r.available(reqCtx, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, clusterDef)

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&appsv1.ClusterDefinition{}).
		Complete(r)
}

func (r *ClusterDefinitionReconciler) deletionHandler(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(clusterDef, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing Cluster")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, clusterDef, constant.ClusterDefLabelKey,
			recordEvent, &appsv1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(rctx, clusterDef)
	}
}

func (r *ClusterDefinitionReconciler) deleteExternalResources(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition) error {
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return appsconfig.DeleteConfigMapFinalizer(r.Client, rctx, clusterDef)
}

func (r *ClusterDefinitionReconciler) available(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition) error {
	return r.status(rctx, clusterDef, appsv1.AvailablePhase, "")
}

func (r *ClusterDefinitionReconciler) unavailable(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return r.status(rctx, clusterDef, appsv1.UnavailablePhase, message)
}

func (r *ClusterDefinitionReconciler) status(rctx intctrlutil.RequestCtx,
	clusterDef *appsv1.ClusterDefinition, phase appsv1.Phase, message string) error {
	patch := client.MergeFrom(clusterDef.DeepCopy())
	clusterDef.Status.ObservedGeneration = clusterDef.Generation
	clusterDef.Status.Phase = phase
	clusterDef.Status.Message = message
	clusterDef.Status.Topologies = r.supportedTopologies(clusterDef)
	return r.Client.Status().Patch(rctx.Ctx, clusterDef, patch)
}

func (r *ClusterDefinitionReconciler) supportedTopologies(clusterDef *appsv1.ClusterDefinition) string {
	topologies := make([]string, 0)
	for _, topology := range clusterDef.Spec.Topologies {
		topologies = append(topologies, topology.Name)
	}
	slices.Sort(topologies)
	return strings.Join(topologies, ",") // TODO(API): topologies length
}

func (r *ClusterDefinitionReconciler) reconcile(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition) (*ctrl.Result, error) {
	if err := r.reconcileTopologies(rctx, clusterDef); err != nil {
		res, err1 := intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
		return &res, err1
	}
	return nil, nil
}

func (r *ClusterDefinitionReconciler) reconcileTopologies(rctx intctrlutil.RequestCtx, clusterDef *appsv1.ClusterDefinition) error {
	if !checkUniqueItemWithValue(clusterDef.Spec.Topologies, "Name", nil) {
		return fmt.Errorf("duplicate topology names")
	}
	if !checkUniqueItemWithValue(clusterDef.Spec.Topologies, "Default", true) {
		return fmt.Errorf("multiple default topologies")
	}
	for _, topology := range clusterDef.Spec.Topologies {
		if err := r.validateTopology(rctx, topology); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterDefinitionReconciler) validateTopology(rctx intctrlutil.RequestCtx, topology appsv1.ClusterTopology) error {
	if err := r.validateTopologyComponents(rctx, topology); err != nil {
		return err
	}
	if err := r.validateTopologyShardings(rctx, topology); err != nil {
		return err
	}
	if err := r.globalUniqueNameCheck(topology); err != nil {
		return err
	}
	if topology.Orders != nil {
		if err := r.validateTopologyOrders(topology); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterDefinitionReconciler) validateTopologyComponents(rctx intctrlutil.RequestCtx, topology appsv1.ClusterTopology) error {
	if !checkUniqueItemWithValue(topology.Components, "Name", nil) {
		return fmt.Errorf("duplicate topology component names")
	}

	// validate topology reference component definitions name pattern
	for _, comp := range topology.Components {
		if err := component.ValidateDefNameRegexp(comp.CompDef); err != nil {
			return fmt.Errorf("invalid component definition reference: %s", comp.CompDef)
		}
	}

	compDefs, err := r.loadTopologyCompDefs(rctx.Ctx, topology)
	if err != nil {
		return err
	}
	for _, comp := range topology.Components {
		if err := r.validateTopologyComponent(compDefs, comp); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterDefinitionReconciler) loadTopologyCompDefs(ctx context.Context,
	topology appsv1.ClusterTopology) (map[string][]*appsv1.ComponentDefinition, error) {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := r.Client.List(ctx, compDefList); err != nil {
		return nil, err
	}

	compDefs := map[string]*appsv1.ComponentDefinition{}
	for i, item := range compDefList.Items {
		compDefs[item.Name] = &compDefList.Items[i]
	}

	result := make(map[string][]*appsv1.ComponentDefinition)
	for _, comp := range topology.Components {
		defs := make([]*appsv1.ComponentDefinition, 0)
		for compDefName := range compDefs {
			if component.PrefixOrRegexMatched(compDefName, comp.CompDef) {
				defs = append(defs, compDefs[compDefName])
			}
		}
		result[comp.Name] = defs
	}
	return result, nil
}

func (r *ClusterDefinitionReconciler) validateTopologyComponent(compDefs map[string][]*appsv1.ComponentDefinition,
	comp appsv1.ClusterTopologyComponent) error {
	defs, ok := compDefs[comp.Name]
	if !ok || len(defs) == 0 {
		return fmt.Errorf("there is no matched definitions found for the component %s", comp.Name)
	}
	return nil
}

func (r *ClusterDefinitionReconciler) validateTopologyShardings(rctx intctrlutil.RequestCtx, topology appsv1.ClusterTopology) error {
	if !checkUniqueItemWithValue(topology.Shardings, "Name", nil) {
		return fmt.Errorf("duplicate topology sharding names")
	}

	for _, sharding := range topology.Shardings {
		if err := component.ValidateDefNameRegexp(sharding.ShardingDef); err != nil {
			return fmt.Errorf("invalid sharding definition reference: %s", sharding.ShardingDef)
		}
	}

	shardingDefs, err := r.loadTopologyShardingDefs(rctx.Ctx, topology)
	if err != nil {
		return err
	}
	for _, sharding := range topology.Shardings {
		if err := r.validateTopologySharding(shardingDefs, sharding); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterDefinitionReconciler) loadTopologyShardingDefs(ctx context.Context,
	topology appsv1.ClusterTopology) (map[string][]*appsv1.ShardingDefinition, error) {
	shardingDefList := &appsv1.ShardingDefinitionList{}
	if err := r.Client.List(ctx, shardingDefList); err != nil {
		return nil, err
	}

	shardingDefs := map[string]*appsv1.ShardingDefinition{}
	for i, item := range shardingDefList.Items {
		shardingDefs[item.Name] = &shardingDefList.Items[i]
	}

	result := make(map[string][]*appsv1.ShardingDefinition)
	for _, sharding := range topology.Shardings {
		defs := make([]*appsv1.ShardingDefinition, 0)
		for shardingDefName := range shardingDefs {
			if component.PrefixOrRegexMatched(shardingDefName, sharding.ShardingDef) {
				defs = append(defs, shardingDefs[shardingDefName])
			}
		}
		result[sharding.Name] = defs
	}
	return result, nil
}

func (r *ClusterDefinitionReconciler) validateTopologySharding(shardingDefs map[string][]*appsv1.ShardingDefinition,
	sharding appsv1.ClusterTopologySharding) error {
	defs, ok := shardingDefs[sharding.Name]
	if !ok || len(defs) == 0 {
		return fmt.Errorf("there is no matched definitions found for the sharding %s", sharding.Name)
	}
	return nil
}

func (r *ClusterDefinitionReconciler) globalUniqueNameCheck(topology appsv1.ClusterTopology) error {
	if len(topology.Components) == 0 || len(topology.Shardings) == 0 {
		return nil
	}
	names := sets.New[string]()
	for _, comp := range topology.Components {
		names.Insert(comp.Name)
	}
	for _, sharding := range topology.Shardings {
		if names.Has(sharding.Name) {
			return fmt.Errorf("duplicate topology component and sharding names: %s", sharding.Name)
		}
		names.Insert(sharding.Name)
	}
	return nil
}

func (r *ClusterDefinitionReconciler) validateTopologyOrders(topology appsv1.ClusterTopology) error {
	entities := make([]string, 0)
	for _, comp := range topology.Components {
		entities = append(entities, comp.Name)
	}
	for _, sharding := range topology.Shardings {
		entities = append(entities, sharding.Name)
	}
	slices.Sort(entities)

	validate := func(order []string) bool {
		if len(order) == 0 {
			return true
		}
		items := strings.Split(strings.Join(order, ","), ",")
		slices.Sort(items)
		return slices.Equal(items, entities)
	}

	if !validate(topology.Orders.Provision) {
		return fmt.Errorf("the components and shardings in provision orders are different from those in definition")
	}
	if !validate(topology.Orders.Terminate) {
		return fmt.Errorf("the components and shardings in terminate orders are different from those in definition")
	}
	if !validate(topology.Orders.Update) {
		return fmt.Errorf("the components and shardings in update orders are different from those in definition")
	}
	return nil
}

// defaultClusterTopology returns the default cluster topology in specified cluster definition.
func defaultClusterTopology(clusterDef *appsv1.ClusterDefinition) *appsv1.ClusterTopology {
	for i, topology := range clusterDef.Spec.Topologies {
		if topology.Default {
			return &clusterDef.Spec.Topologies[i]
		}
	}
	return nil
}

// referredClusterTopology returns the cluster topology which has name @name.
func referredClusterTopology(clusterDef *appsv1.ClusterDefinition, name string) *appsv1.ClusterTopology {
	if len(name) == 0 {
		return defaultClusterTopology(clusterDef)
	}
	for i, topology := range clusterDef.Spec.Topologies {
		if topology.Name == name {
			return &clusterDef.Spec.Topologies[i]
		}
	}
	return nil
}

func clusterTopologyCompMatched(comp appsv1.ClusterTopologyComponent, compName string) bool {
	if comp.Name == compName {
		return true
	}
	if comp.Template != nil && *comp.Template {
		return strings.HasPrefix(compName, comp.Name)
	}
	return false
}
