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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ClusterTopologyReconciler reconciles a ClusterTopology object
type ClusterTopologyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clustertopologies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clustertopologies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clustertopologies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterTopology object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterTopologyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterTopology", req.NamespacedName),
		Recorder: r.Recorder,
	}

	rctx.Log.V(1).Info("reconcile", "clusterTopology", req.NamespacedName)

	clusterTopology := &appsv1alpha1.ClusterTopology{}
	if err := r.Client.Get(rctx.Ctx, rctx.Req.NamespacedName, clusterTopology); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	return r.reconcile(rctx, clusterTopology)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterTopologyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ClusterTopology{}).
		Complete(r)
}

func (r *ClusterTopologyReconciler) reconcile(rctx intctrlutil.RequestCtx,
	clusterTopology *appsv1alpha1.ClusterTopology) (ctrl.Result, error) {
	res, err := intctrlutil.HandleCRDeletion(rctx, r, clusterTopology, clusterTopologyFinalizerName, r.deletionHandler(rctx, clusterTopology))
	if res != nil {
		return *res, err
	}

	if clusterTopology.Status.ObservedGeneration == clusterTopology.Generation &&
		slices.Contains([]appsv1alpha1.Phase{appsv1alpha1.AvailablePhase}, clusterTopology.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err = r.validate(rctx, clusterTopology); err != nil {
		if err1 := r.unavailable(r.Client, rctx, clusterTopology, err); err1 != nil {
			return intctrlutil.CheckedRequeueWithError(err1, rctx.Log, "")
		}
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	err = r.available(r.Client, rctx, clusterTopology)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, rctx.Log, "")
	}

	intctrlutil.RecordCreatedEvent(r.Recorder, clusterTopology)

	return intctrlutil.Reconciled()
}

func (r *ClusterTopologyReconciler) deletionHandler(rctx intctrlutil.RequestCtx,
	clusterTopology *appsv1alpha1.ClusterTopology) func() (*ctrl.Result, error) {
	return func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(clusterTopology, corev1.EventTypeWarning, constant.ReasonRefCRUnavailable,
				"cannot be deleted because of existing referencing Cluster.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(rctx, r.Client, clusterTopology, constant.ClusterTopologyLabelKey,
			recordEvent, &appsv1alpha1.ClusterList{}); res != nil || err != nil {
			return res, err
		}
		return nil, nil
	}
}

func (r *ClusterTopologyReconciler) available(cli client.Client, rctx intctrlutil.RequestCtx,
	clusterTopology *appsv1alpha1.ClusterTopology) error {
	return r.status(cli, rctx, clusterTopology, appsv1alpha1.AvailablePhase, "")
}

func (r *ClusterTopologyReconciler) unavailable(cli client.Client, rctx intctrlutil.RequestCtx,
	clusterTopology *appsv1alpha1.ClusterTopology, err error) error {
	return r.status(cli, rctx, clusterTopology, appsv1alpha1.UnavailablePhase, err.Error())
}

func (r *ClusterTopologyReconciler) status(cli client.Client, rctx intctrlutil.RequestCtx,
	clusterTopology *appsv1alpha1.ClusterTopology, phase appsv1alpha1.Phase, message string) error {
	patch := client.MergeFrom(clusterTopology.DeepCopy())
	clusterTopology.Status.ObservedGeneration = clusterTopology.Generation
	clusterTopology.Status.Phase = phase
	clusterTopology.Status.Message = message
	clusterTopology.Status.Topologies = r.supportedTopologies(clusterTopology)
	clusterTopology.Status.ExternalServices = r.referredExternalServices(clusterTopology)
	return cli.Status().Patch(rctx.Ctx, clusterTopology, patch)
}

func (r *ClusterTopologyReconciler) supportedTopologies(clusterTopology *appsv1alpha1.ClusterTopology) string {
	topologies := make([]string, 0)
	for _, topology := range clusterTopology.Spec.Topologies {
		topologies = append(topologies, topology.Name)
	}
	slices.Sort(topologies)
	return strings.Join(topologies, ",") // TODO
}

func (r *ClusterTopologyReconciler) referredExternalServices(clusterTopology *appsv1alpha1.ClusterTopology) string {
	return ""
}

func (r *ClusterTopologyReconciler) validate(rctx intctrlutil.RequestCtx, clusterTopology *appsv1alpha1.ClusterTopology) error {
	if !checkUniqueItemWithValue(clusterTopology.Spec.Topologies, "Name", nil) {
		return fmt.Errorf("duplicate topology names")
	}
	if !checkUniqueItemWithValue(clusterTopology.Spec.Topologies, "Default", true) {
		return fmt.Errorf("multiple default topologies")
	}
	for _, topology := range clusterTopology.Spec.Topologies {
		if err := r.validateTopology(rctx, topology); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterTopologyReconciler) validateTopology(rctx intctrlutil.RequestCtx, topology appsv1alpha1.ClusterTopologyDefinition) error {
	if !checkUniqueItemWithValue(topology.Components, "Name", nil) {
		return fmt.Errorf("duplicate topology component names")
	}
	if topology.Orders != nil {
		if err := r.validateTopologyOrders(topology); err != nil {
			return err
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

func (r *ClusterTopologyReconciler) validateTopologyOrders(topology appsv1alpha1.ClusterTopologyDefinition) error {
	comps := make([]string, 0)
	for _, comp := range topology.Components {
		comps = append(comps, comp.Name)
	}
	slices.Sort(comps)

	validate := func(order []string) bool {
		if len(order) == 0 {
			return true
		}
		items := strings.Split(strings.Join(order, ","), ",")
		slices.Sort(items)
		return slices.Equal(items, comps)
	}

	if !validate(topology.Orders.StartupOrder) {
		return fmt.Errorf("there are components not defined in startup order")
	}
	if !validate(topology.Orders.ShutdownOrder) {
		return fmt.Errorf("there are components not defined in shutdown order")
	}
	if !validate(topology.Orders.UpdateOrder) {
		return fmt.Errorf("there are components not defined in update order")
	}
	return nil
}

func (r *ClusterTopologyReconciler) loadTopologyCompDefs(ctx context.Context,
	topology appsv1alpha1.ClusterTopologyDefinition) (map[string][]*appsv1alpha1.ComponentDefinition, error) {
	compDefList := &appsv1alpha1.ComponentDefinitionList{}
	if err := r.Client.List(ctx, compDefList); err != nil {
		return nil, err
	}

	compDefs := map[string]*appsv1alpha1.ComponentDefinition{}
	for i, item := range compDefList.Items {
		compDefs[item.Name] = &compDefList.Items[i]
	}

	result := make(map[string][]*appsv1alpha1.ComponentDefinition)
	for _, comp := range topology.Components {
		defs := make([]*appsv1alpha1.ComponentDefinition, 0)
		for compDefName := range compDefs {
			if strings.HasPrefix(compDefName, comp.CompDef) {
				defs = append(defs, compDefs[compDefName])
			}
		}
		result[comp.Name] = defs
	}
	return result, nil
}

func (r *ClusterTopologyReconciler) validateTopologyComponent(compDefs map[string][]*appsv1alpha1.ComponentDefinition,
	comp appsv1alpha1.ClusterTopologyComponent) error {
	// TODO: service version
	defs, ok := compDefs[comp.Name]
	if !ok {
		return fmt.Errorf("there is no matched definitions found for the topology component %s", comp.Name)
	}

	if !checkUniqueItemWithValue(comp.ServiceRefs, "Name", nil) {
		return fmt.Errorf("duplicate topology component serviceRef")
	}
	for _, serviceRef := range comp.ServiceRefs {
		if err := r.validateTopologyCompServiceRefs(defs, comp, serviceRef); err != nil {
			return err
		}
	}
	if len(comp.RequiredVersion) > 0 {
		if err := r.validateTopologyCompRequiredVersion(defs, comp); err != nil {
			return err
		}
	}
	if comp.Replicas != nil {
		if err := r.validateTopologyCompReplicas(defs, comp, *comp.Replicas); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterTopologyReconciler) validateTopologyCompServiceRefs(compDefs []*appsv1alpha1.ComponentDefinition,
	comp appsv1alpha1.ClusterTopologyComponent, serviceRef appsv1alpha1.ServiceRef) error {
	match := func(d appsv1alpha1.ServiceRefDeclaration) bool {
		return d.Name == serviceRef.Name
	}
	for _, compDef := range compDefs {
		if slices.IndexFunc(compDef.Spec.ServiceRefDeclarations, match) == -1 {
			return fmt.Errorf("service ref %s in topology component %s not declared in matched definition %s",
				serviceRef.Name, comp.Name, compDef.Name)
		}
	}
	return nil
}

func (r *ClusterTopologyReconciler) validateTopologyCompRequiredVersion(compDefs []*appsv1alpha1.ComponentDefinition,
	comp appsv1alpha1.ClusterTopologyComponent) error {
	// TODO
	return nil
}

func (r *ClusterTopologyReconciler) validateTopologyCompReplicas(compDefs []*appsv1alpha1.ComponentDefinition,
	comp appsv1alpha1.ClusterTopologyComponent, replicas int32) error {
	for _, compDef := range compDefs {
		limit := compDef.Spec.ReplicasLimit
		if limit != nil {
			if replicas < limit.MinReplicas || replicas > limit.MaxReplicas {
				return fmt.Errorf("topology component %s default replicas %d out-of-limit", comp.Name, replicas)
			}
		}
	}
	return nil
}
