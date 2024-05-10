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
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type clusterComponentTransformer struct{}

var _ graph.Transformer = &clusterComponentTransformer{}

func (t *clusterComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if len(transCtx.ComponentSpecs) == 0 {
		return nil
	}

	allCompsReady, err := checkAllCompsReady(transCtx, transCtx.Cluster)
	if err != nil {
		return err
	}

	// if all component objects ready and cluster is not updating, skip reconciling components
	if !transCtx.OrigCluster.IsUpdating() && allCompsReady {
		return nil
	}

	return t.reconcileComponents(transCtx, dag)
}

func (t *clusterComponentTransformer) reconcileComponents(transCtx *clusterTransformContext, dag *graph.DAG) error {
	cluster := transCtx.Cluster

	protoCompSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, compSpec := range transCtx.ComponentSpecs {
		protoCompSpecMap[compSpec.Name] = compSpec
	}

	protoCompSet := sets.KeySet(protoCompSpecMap)
	runningCompSet, err := component.GetClusterComponentShortNameSet(transCtx.Context, transCtx.Client, cluster)
	if err != nil {
		return err
	}

	createCompSet := protoCompSet.Difference(runningCompSet)
	updateCompSet := protoCompSet.Intersection(runningCompSet)
	deleteCompSet := runningCompSet.Difference(protoCompSet)

	// component objects to be deleted
	if err := t.handleCompsDelete(transCtx, dag, protoCompSpecMap, deleteCompSet, transCtx.Labels, transCtx.Annotations); err != nil {
		return err
	}

	// component objects to be updated
	if err := t.handleCompsUpdate(transCtx, dag, protoCompSpecMap, updateCompSet, transCtx.Labels, transCtx.Annotations); err != nil {
		return err
	}

	// component objects to be created
	if err := t.handleCompsCreate(transCtx, dag, protoCompSpecMap, createCompSet, transCtx.Labels, transCtx.Annotations); err != nil {
		return err
	}

	return nil
}

func (t *clusterComponentTransformer) handleCompsCreate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, createCompSet sets.Set[string],
	protoCompLabelsMap, protoCompAnnotationsMap map[string]map[string]string) error {
	handler := newCompHandler(transCtx, protoCompSpecMap, protoCompLabelsMap, protoCompAnnotationsMap, createOp)
	return t.handleComps(transCtx, dag, createCompSet, handler)
}

func (t *clusterComponentTransformer) handleCompsDelete(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, deleteCompSet sets.Set[string],
	protoCompLabelsMap, protoCompAnnotationsMap map[string]map[string]string) error {
	handler := newCompHandler(transCtx, protoCompSpecMap, protoCompLabelsMap, protoCompAnnotationsMap, deleteOp)
	return t.handleComps(transCtx, dag, deleteCompSet, handler)
}

func (t *clusterComponentTransformer) handleCompsUpdate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, updateCompSet sets.Set[string],
	protoCompLabelsMap, protoCompAnnotationsMap map[string]map[string]string) error {
	handler := newCompHandler(transCtx, protoCompSpecMap, protoCompLabelsMap, protoCompAnnotationsMap, updateOp)
	return t.handleComps(transCtx, dag, updateCompSet, handler)
}

func (t *clusterComponentTransformer) handleComps(transCtx *clusterTransformContext, dag *graph.DAG,
	compNameSet sets.Set[string], handler compConditionalHandler) error {
	var unmatched []string
	for _, compName := range handler.ordered(sets.List(compNameSet)) {
		ok, err := handler.match(transCtx, dag, compName)
		if err != nil {
			return err
		}
		if ok {
			if err = handler.handle(transCtx, dag, compName); err != nil {
				return err
			}
		} else {
			unmatched = append(unmatched, compName)
		}
	}
	if len(unmatched) > 0 {
		return controllerutil.NewDelayedRequeueError(0, fmt.Sprintf("retry later: %s are not ready", strings.Join(unmatched, ",")))
	}
	return nil
}

func checkAllCompsReady(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) (bool, error) {
	compList := &appsv1alpha1.ComponentList{}
	labels := constant.GetClusterWellKnownLabels(cluster.Name)
	if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	if len(compList.Items) != len(transCtx.ComponentSpecs) {
		return false, nil
	}
	return true, nil
}

// getRunningCompObject gets the component object from cache snapshot
func getRunningCompObject(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster, compName string) (*appsv1alpha1.Component, error) {
	compKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      component.FullName(cluster.Name, compName),
	}
	comp := &appsv1alpha1.Component{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
		return nil, err
	}
	return comp, nil
}

// copyAndMergeComponent merges two component objects for updating:
// 1. new a component object targetCompObj by copying from oldCompObj
// 2. merge all fields can be updated from newCompObj into targetCompObj
func copyAndMergeComponent(oldCompObj, newCompObj *appsv1alpha1.Component) *appsv1alpha1.Component {
	compObjCopy := oldCompObj.DeepCopy()
	compProto := newCompObj

	// merge labels and annotations
	ictrlutil.MergeMetadataMapInplace(compObjCopy.Annotations, &compProto.Annotations)
	ictrlutil.MergeMetadataMapInplace(compObjCopy.Labels, &compProto.Labels)
	compObjCopy.Annotations = compProto.Annotations
	compObjCopy.Labels = compProto.Labels

	// merge spec
	compObjCopy.Spec.CompDef = compProto.Spec.CompDef
	compObjCopy.Spec.ServiceVersion = compProto.Spec.ServiceVersion
	compObjCopy.Spec.ClassDefRef = compProto.Spec.ClassDefRef
	compObjCopy.Spec.ServiceRefs = compProto.Spec.ServiceRefs
	compObjCopy.Spec.Resources = compProto.Spec.Resources
	compObjCopy.Spec.VolumeClaimTemplates = compProto.Spec.VolumeClaimTemplates
	compObjCopy.Spec.Services = compProto.Spec.Services
	compObjCopy.Spec.Replicas = compProto.Spec.Replicas
	compObjCopy.Spec.Configs = compProto.Spec.Configs
	// compObjCopy.Spec.Monitor = compProto.Spec.Monitor
	compObjCopy.Spec.EnabledLogs = compProto.Spec.EnabledLogs
	compObjCopy.Spec.ServiceAccountName = compProto.Spec.ServiceAccountName
	compObjCopy.Spec.Affinity = compProto.Spec.Affinity
	compObjCopy.Spec.Tolerations = compProto.Spec.Tolerations
	compObjCopy.Spec.TLSConfig = compProto.Spec.TLSConfig
	compObjCopy.Spec.Instances = compProto.Spec.Instances
	compObjCopy.Spec.OfflineInstances = compProto.Spec.OfflineInstances
	compObjCopy.Spec.RuntimeClassName = compProto.Spec.RuntimeClassName
	compObjCopy.Spec.Sidecars = compProto.Spec.Sidecars
	compObjCopy.Spec.MonitorEnabled = compProto.Spec.MonitorEnabled

	if reflect.DeepEqual(oldCompObj.Annotations, compObjCopy.Annotations) &&
		reflect.DeepEqual(oldCompObj.Labels, compObjCopy.Labels) &&
		reflect.DeepEqual(oldCompObj.Spec, compObjCopy.Spec) {
		return nil
	}
	return compObjCopy
}

const (
	createOp int = 0
	deleteOp int = 1
	updateOp int = 2
)

func newCompHandler(transCtx *clusterTransformContext, compSpecs map[string]*appsv1alpha1.ClusterComponentSpec,
	labels, annotations map[string]map[string]string, op int) compConditionalHandler {
	orders := definedOrders(transCtx, op)
	if len(orders) == 0 {
		return newParallelHandler(compSpecs, labels, annotations, op)
	}
	return newOrderedHandler(compSpecs, labels, annotations, orders, op)
}

func definedOrders(transCtx *clusterTransformContext, op int) []string {
	var (
		cluster    = transCtx.Cluster
		clusterDef = transCtx.ClusterDef
	)
	if len(cluster.Spec.Topology) != 0 && clusterDef != nil {
		for _, topology := range clusterDef.Spec.Topologies {
			if topology.Name == cluster.Spec.Topology {
				if topology.Orders != nil {
					switch op {
					case createOp:
						return topology.Orders.Provision
					case deleteOp:
						return topology.Orders.Terminate
					case updateOp:
						return topology.Orders.Update
					default:
						panic("runtime error: unknown component op: " + strconv.Itoa(op))
					}
				}
			}
		}
	}
	return nil
}

func newParallelHandler(compSpecs map[string]*appsv1alpha1.ClusterComponentSpec,
	labels, annotations map[string]map[string]string, op int) compConditionalHandler {
	switch op {
	case createOp:
		return &parallelCreateCompHandler{
			createCompHandler: createCompHandler{
				compSpecs:   compSpecs,
				labels:      labels,
				annotations: annotations,
			},
		}
	case deleteOp:
		return &parallelDeleteCompHandler{}
	case updateOp:
		return &parallelUpdateCompHandler{
			updateCompHandler: updateCompHandler{
				compSpecs:   compSpecs,
				labels:      labels,
				annotations: annotations,
			},
		}
	default:
		panic("runtime error: unknown component op: " + strconv.Itoa(op))
	}
}

func newOrderedHandler(compSpecs map[string]*appsv1alpha1.ClusterComponentSpec,
	labels, annotations map[string]map[string]string, orders []string, op int) compConditionalHandler {
	switch op {
	case createOp:
		return &orderedCreateCompHandler{
			compOrderedOrder: compOrderedOrder{
				orders: orders,
			},
			compPhasePrecondition: compPhasePrecondition{
				orders:         orders,
				expectedPhases: []appsv1alpha1.ClusterComponentPhase{appsv1alpha1.RunningClusterCompPhase},
			},
			createCompHandler: createCompHandler{
				compSpecs:   compSpecs,
				labels:      labels,
				annotations: annotations,
			},
		}
	case deleteOp:
		return &orderedDeleteCompHandler{
			compOrderedOrder: compOrderedOrder{
				orders: orders,
			},
			compNotExistPrecondition: compNotExistPrecondition{
				orders: orders,
			},
			deleteCompHandler: deleteCompHandler{},
		}
	case updateOp:
		return &orderedUpdateCompHandler{
			compOrderedOrder: compOrderedOrder{
				orders: orders,
			},
			compPhasePrecondition: compPhasePrecondition{
				orders:         orders,
				expectedPhases: []appsv1alpha1.ClusterComponentPhase{appsv1alpha1.RunningClusterCompPhase},
			},
			updateCompHandler: updateCompHandler{
				compSpecs:   compSpecs,
				labels:      labels,
				annotations: annotations,
			},
		}
	default:
		panic("runtime error: unknown component op: " + strconv.Itoa(op))
	}
}

type compConditionalHandler interface {
	ordered([]string) []string
	match(transCtx *clusterTransformContext, dag *graph.DAG, compName string) (bool, error)
	handle(transCtx *clusterTransformContext, dag *graph.DAG, compName string) error
}

type compParallelOrder struct{}

func (o *compParallelOrder) ordered(compNames []string) []string {
	return compNames
}

type compOrderedOrder struct {
	orders []string
}

func (o *compOrderedOrder) ordered(compNames []string) []string {
	result := make([]string, 0)
	for _, order := range o.orders {
		comps := strings.Split(order, ",")
		for _, comp := range compNames {
			if slices.Index(comps, comp) >= 0 {
				result = append(result, comp)
			}
		}
	}
	if len(result) != len(compNames) {
		panic("runtime error: cannot find order for components " + strings.Join(compNames, ","))
	}
	return result
}

type compDummyPrecondition struct{}

func (c *compDummyPrecondition) match(*clusterTransformContext, *graph.DAG, string) (bool, error) {
	return true, nil
}

type compNotExistPrecondition struct {
	orders []string
}

func (c *compNotExistPrecondition) match(transCtx *clusterTransformContext, dag *graph.DAG, compName string) (bool, error) {
	get := func(compKey types.NamespacedName) (bool, error) {
		comp := &appsv1alpha1.Component{}
		err := transCtx.Client.Get(transCtx.Context, compKey, comp)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return err == nil, nil
	}
	dagCreate := func(compKey types.NamespacedName) bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		comp := &appsv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: compKey.Namespace,
				Name:      compKey.Name,
			},
		}
		return graphCli.IsAction(dag, comp, model.ActionCreatePtr())
	}
	for _, predecessor := range predecessors(c.orders, compName) {
		compKey := types.NamespacedName{
			Namespace: transCtx.Cluster.Namespace,
			Name:      component.FullName(transCtx.Cluster.Name, predecessor),
		}
		exist, err := get(compKey)
		if err != nil {
			return false, err
		}
		if exist {
			return false, nil
		}
		if dagCreate(compKey) {
			return false, nil
		}
	}
	return true, nil
}

type compPhasePrecondition struct {
	orders         []string
	expectedPhases []appsv1alpha1.ClusterComponentPhase
}

func (c *compPhasePrecondition) match(transCtx *clusterTransformContext, dag *graph.DAG, compName string) (bool, error) {
	dagGet := func(compKey types.NamespacedName) bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		for _, obj := range graphCli.FindAll(dag, &appsv1alpha1.Component{}) {
			if client.ObjectKeyFromObject(obj) == compKey {
				return true
			}
		}
		return false
	}
	for _, predecessor := range predecessors(c.orders, compName) {
		comp := &appsv1alpha1.Component{}
		compKey := types.NamespacedName{
			Namespace: transCtx.Cluster.Namespace,
			Name:      component.FullName(transCtx.Cluster.Name, predecessor),
		}
		if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if comp.Generation != comp.Status.ObservedGeneration || slices.Index(c.expectedPhases, comp.Status.Phase) < 0 {
			return false, nil
		}
		// create or update if exists in DAG
		if dagGet(compKey) {
			return false, nil
		}
	}
	return true, nil
}

func predecessors(orders []string, compName string) []string {
	var previous []string
	for _, comps := range orders {
		compNames := strings.Split(comps, ",")
		if index := slices.Index(compNames, compName); index >= 0 {
			return previous
		}
		previous = compNames
	}
	panic("runtime error: cannot find predecessor for component " + compName)
}

type createCompHandler struct {
	compSpecs   map[string]*appsv1alpha1.ClusterComponentSpec
	labels      map[string]map[string]string
	annotations map[string]map[string]string
}

func (h *createCompHandler) handle(transCtx *clusterTransformContext, dag *graph.DAG, compName string) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp, err := component.BuildComponent(cluster, h.compSpecs[compName], h.labels[compName], h.annotations[compName])
	if err != nil {
		return err
	}
	graphCli.Create(dag, comp)
	h.initClusterCompStatus(cluster, compName)
	return nil
}

func (h *createCompHandler) initClusterCompStatus(cluster *appsv1alpha1.Cluster, compName string) {
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	cluster.Status.Components[compName] = appsv1alpha1.ClusterComponentStatus{}
}

type deleteCompHandler struct{}

func (h *deleteCompHandler) handle(transCtx *clusterTransformContext, dag *graph.DAG, compName string) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp, err := getRunningCompObject(transCtx, cluster, compName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) || model.IsObjectDeleting(comp) {
		return nil
	}
	transCtx.Logger.Info(fmt.Sprintf("deleting component %s", comp.Name))
	compCopy := comp.DeepCopy()
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	// update the scale-in annotation to component before deleting
	comp.Annotations[constant.ComponentScaleInAnnotationKey] = trueVal
	deleteCompVertex := graphCli.Do(dag, nil, comp, model.ActionDeletePtr(), nil)
	graphCli.Do(dag, compCopy, comp, model.ActionUpdatePtr(), deleteCompVertex)
	return nil
}

type updateCompHandler struct {
	compSpecs   map[string]*appsv1alpha1.ClusterComponentSpec
	labels      map[string]map[string]string
	annotations map[string]map[string]string
}

func (h *updateCompHandler) handle(transCtx *clusterTransformContext, dag *graph.DAG, compName string) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	runningComp, getErr := getRunningCompObject(transCtx, cluster, compName)
	if getErr != nil {
		return getErr
	}
	comp, buildErr := component.BuildComponent(cluster, h.compSpecs[compName], h.labels[compName], h.annotations[compName])
	if buildErr != nil {
		return buildErr
	}
	if newCompObj := copyAndMergeComponent(runningComp, comp); newCompObj != nil {
		graphCli.Update(dag, runningComp, newCompObj)
	}
	return nil
}

type parallelCreateCompHandler struct {
	compParallelOrder
	compDummyPrecondition
	createCompHandler
}

type parallelDeleteCompHandler struct {
	compParallelOrder
	compDummyPrecondition
	deleteCompHandler
}

type parallelUpdateCompHandler struct {
	compParallelOrder
	compDummyPrecondition
	updateCompHandler
}

type orderedCreateCompHandler struct {
	compOrderedOrder
	compPhasePrecondition
	createCompHandler
}

type orderedDeleteCompHandler struct {
	compOrderedOrder
	compNotExistPrecondition
	deleteCompHandler
}

type orderedUpdateCompHandler struct {
	compOrderedOrder
	compPhasePrecondition
	updateCompHandler
}
