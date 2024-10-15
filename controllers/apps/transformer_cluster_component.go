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
	"reflect"
	"slices"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterComponentTransformer transforms components and shardings to mapping Component objects
type clusterComponentTransformer struct{}

var _ graph.Transformer = &clusterComponentTransformer{}

func (t *clusterComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if len(transCtx.allComps) == 0 {
		return nil
	}

	updateToDate, err := checkAllCompsUpToDate(transCtx, transCtx.Cluster)
	if err != nil {
		return err
	}

	// if the cluster is not updating and all components are up-to-date, skip the reconciliation
	if !transCtx.OrigCluster.IsUpdating() && updateToDate {
		return nil
	}

	return t.transform(transCtx, dag)
}

func (t *clusterComponentTransformer) transform(transCtx *clusterTransformContext, dag *graph.DAG) error {
	runningSet, err := t.runningSet(transCtx)
	if err != nil {
		return err
	}
	protoSet := t.protoSet(transCtx)

	createSet, deleteSet, updateSet := setDiff(runningSet, protoSet)

	if err := deleteCompNShardingInOrder(transCtx, dag, deleteSet, pointer.Bool(true)); err != nil {
		return err
	}

	var delayedErr error
	if err := t.handleUpdate(transCtx, dag, updateSet); err != nil {
		if !ictrlutil.IsDelayedRequeueError(err) {
			return err
		}
		delayedErr = err
	}

	if err := t.handleCreate(transCtx, dag, createSet); err != nil {
		return err
	}

	return delayedErr
}

func (t *clusterComponentTransformer) runningSet(transCtx *clusterTransformContext) (sets.Set[string], error) {
	return clusterRunningCompNShardingSet(transCtx.Context, transCtx.Client, transCtx.Cluster)
}

func (t *clusterComponentTransformer) protoSet(transCtx *clusterTransformContext) sets.Set[string] {
	names := sets.Set[string]{}
	for _, comp := range transCtx.components {
		names.Insert(comp.Name)
	}
	for _, sharding := range transCtx.shardings {
		names.Insert(sharding.Name)
	}
	return names
}

func (t *clusterComponentTransformer) handleCreate(transCtx *clusterTransformContext, dag *graph.DAG, createSet sets.Set[string]) error {
	handler := newCompNShardingHandler(transCtx, createOp)
	return handleCompNShardingInOrder(transCtx, dag, createSet, handler)
}

func (t *clusterComponentTransformer) handleUpdate(transCtx *clusterTransformContext, dag *graph.DAG, updateSet sets.Set[string]) error {
	handler := newCompNShardingHandler(transCtx, updateOp)
	return handleCompNShardingInOrder(transCtx, dag, updateSet, handler)
}

func deleteCompNShardingInOrder(transCtx *clusterTransformContext, dag *graph.DAG, deleteSet sets.Set[string], scaleIn *bool) error {
	handler := newCompNShardingHandler(transCtx, deleteOp)
	if h, ok := handler.(*clusterParallelHandler); ok {
		h.scaleIn = scaleIn
	}
	if h, ok := handler.(*orderedDeleteHandler); ok {
		h.scaleIn = scaleIn
	}
	return handleCompNShardingInOrder(transCtx, dag, deleteSet, handler)
}

func handleCompNShardingInOrder(transCtx *clusterTransformContext, dag *graph.DAG, nameSet sets.Set[string], handler clusterConditionalHandler) error {
	unmatched := ""
	for _, name := range handler.ordered(sets.List(nameSet)) {
		ok, err := handler.match(transCtx, dag, name)
		if err != nil {
			return err
		}
		if !ok {
			unmatched = name
			break
		}
		if err = handler.handle(transCtx, dag, name); err != nil {
			return err
		}
	}
	if len(unmatched) > 0 {
		return ictrlutil.NewDelayedRequeueError(0, fmt.Sprintf("retry later: %s are not ready", unmatched))
	}
	return nil
}

func checkAllCompsUpToDate(transCtx *clusterTransformContext, cluster *appsv1.Cluster) (bool, error) {
	compList := &appsv1.ComponentList{}
	labels := constant.GetClusterLabels(cluster.Name)
	if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	if len(compList.Items) != len(transCtx.allComps) {
		return false, nil
	}
	for _, comp := range compList.Items {
		generation, ok := comp.Annotations[constant.KubeBlocksGenerationKey]
		if !ok {
			return false, nil
		}
		if comp.Generation != comp.Status.ObservedGeneration || generation != strconv.FormatInt(cluster.Generation, 10) {
			return false, nil
		}
	}
	return true, nil
}

// copyAndMergeComponent merges two component objects for updating:
// 1. new a component object targetCompObj by copying from oldCompObj
// 2. merge all fields can be updated from newCompObj into targetCompObj
func copyAndMergeComponent(oldCompObj, newCompObj *appsv1.Component) *appsv1.Component {
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
	compObjCopy.Spec.ServiceRefs = compProto.Spec.ServiceRefs
	compObjCopy.Spec.Labels = compProto.Spec.Labels
	compObjCopy.Spec.Annotations = compProto.Spec.Annotations
	compObjCopy.Spec.Env = compProto.Spec.Env
	compObjCopy.Spec.Resources = compProto.Spec.Resources
	compObjCopy.Spec.VolumeClaimTemplates = compProto.Spec.VolumeClaimTemplates
	compObjCopy.Spec.Volumes = compProto.Spec.Volumes
	compObjCopy.Spec.Services = compProto.Spec.Services
	compObjCopy.Spec.Replicas = compProto.Spec.Replicas
	compObjCopy.Spec.Configs = compProto.Spec.Configs
	compObjCopy.Spec.ServiceAccountName = compProto.Spec.ServiceAccountName
	compObjCopy.Spec.ParallelPodManagementConcurrency = compProto.Spec.ParallelPodManagementConcurrency
	compObjCopy.Spec.PodUpdatePolicy = compProto.Spec.PodUpdatePolicy
	compObjCopy.Spec.SchedulingPolicy = compProto.Spec.SchedulingPolicy
	compObjCopy.Spec.TLSConfig = compProto.Spec.TLSConfig
	compObjCopy.Spec.Instances = compProto.Spec.Instances
	compObjCopy.Spec.OfflineInstances = compProto.Spec.OfflineInstances
	compObjCopy.Spec.RuntimeClassName = compProto.Spec.RuntimeClassName
	compObjCopy.Spec.DisableExporter = compProto.Spec.DisableExporter
	compObjCopy.Spec.Stop = compProto.Spec.Stop

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

func newCompNShardingHandler(transCtx *clusterTransformContext, op int) clusterConditionalHandler {
	orders := definedOrders(transCtx, op)
	if len(orders) == 0 {
		return newParallelHandler(op)
	}
	return newOrderedHandler(orders, op)
}

func definedOrders(transCtx *clusterTransformContext, op int) []string {
	var (
		cluster    = transCtx.Cluster
		clusterDef = transCtx.clusterDef
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
						panic("runtime error: unknown operation: " + strconv.Itoa(op))
					}
				}
			}
		}
	}
	return nil
}

func newParallelHandler(op int) clusterConditionalHandler {
	if op == createOp || op == deleteOp || op == updateOp {
		return &clusterParallelHandler{
			clusterCompNShardingHandler: clusterCompNShardingHandler{op: op},
		}
	}
	panic("runtime error: unknown operation: " + strconv.Itoa(op))
}

func newOrderedHandler(orders []string, op int) clusterConditionalHandler {
	switch op {
	case createOp, updateOp:
		return &orderedCreateNUpdateHandler{
			clusterOrderedOrder:         clusterOrderedOrder{orders: orders},
			phasePrecondition:           phasePrecondition{orders: orders},
			clusterCompNShardingHandler: clusterCompNShardingHandler{op: op},
		}
	case deleteOp:
		return &orderedDeleteHandler{
			clusterOrderedOrder:         clusterOrderedOrder{orders: orders},
			notExistPrecondition:        notExistPrecondition{orders: orders},
			clusterCompNShardingHandler: clusterCompNShardingHandler{op: op},
		}
	default:
		panic("runtime error: unknown operation: " + strconv.Itoa(op))
	}
}

type clusterConditionalHandler interface {
	ordered([]string) []string
	match(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error)
	handle(transCtx *clusterTransformContext, dag *graph.DAG, name string) error
}

type clusterParallelOrder struct{}

func (o *clusterParallelOrder) ordered(names []string) []string {
	return names
}

type clusterOrderedOrder struct {
	orders []string
}

func (o *clusterOrderedOrder) ordered(names []string) []string {
	result := make([]string, 0)
	for _, order := range o.orders {
		comps := strings.Split(order, ",")
		for _, comp := range names {
			if slices.Index(comps, comp) >= 0 {
				result = append(result, comp)
			}
		}
	}
	if len(result) != len(names) {
		panic("runtime error: cannot find order for components and shardings " + strings.Join(names, ","))
	}
	return result
}

type dummyPrecondition struct{}

func (c *dummyPrecondition) match(*clusterTransformContext, *graph.DAG, string) (bool, error) {
	return true, nil
}

type notExistPrecondition struct {
	orders []string
}

func (c *notExistPrecondition) match(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	for _, predecessor := range predecessors(c.orders, name) {
		exist, err := c.predecessorExist(transCtx, dag, predecessor)
		if err != nil {
			return false, err
		}
		if exist {
			return false, nil
		}
	}
	return true, nil
}

func (c *notExistPrecondition) predecessorExist(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	if transCtx.sharding(name) {
		return c.shardingExist(transCtx, dag, name)
	}
	return c.compExist(transCtx, dag, name)
}

func (c *notExistPrecondition) compExist(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	var (
		compKey = types.NamespacedName{
			Namespace: transCtx.Cluster.Namespace,
			Name:      component.FullName(transCtx.Cluster.Name, name),
		}
	)
	get := func() (bool, error) {
		comp := &appsv1.Component{}
		err := transCtx.Client.Get(transCtx.Context, compKey, comp)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return err == nil, nil
	}
	dagCreate := func() bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: compKey.Namespace,
				Name:      compKey.Name,
			},
		}
		return graphCli.IsAction(dag, comp, model.ActionCreatePtr())
	}

	exist, err := get()
	if err != nil {
		return false, err
	}
	if exist {
		return true, nil
	}
	if dagCreate() {
		return true, nil
	}
	return false, nil
}

func (c *notExistPrecondition) shardingExist(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	list := func() (bool, error) {
		comps, err := ictrlutil.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, name)
		if err != nil {
			return false, err
		}
		return len(comps) > 0, nil
	}
	dagCreate := func() bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		for _, obj := range graphCli.FindAll(dag, &appsv1.Component{}) {
			if shardingCompWithName(obj.(*appsv1.Component), name) &&
				graphCli.IsAction(dag, obj, model.ActionCreatePtr()) {
				return true
			}
		}
		return false
	}

	exist, err := list()
	if err != nil {
		return false, err
	}
	if exist {
		return true, nil
	}
	if dagCreate() {
		return true, nil
	}
	return false, nil
}

type phasePrecondition struct {
	orders []string
}

func (c *phasePrecondition) match(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	for _, predecessor := range predecessors(c.orders, name) {
		match, err := c.predecessorMatch(transCtx, dag, predecessor)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

func (c *phasePrecondition) predecessorMatch(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	if transCtx.sharding(name) {
		return c.shardingMatch(transCtx, dag, name)
	}
	return c.compMatch(transCtx, dag, name)
}

func (c *phasePrecondition) compMatch(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	var (
		compKey = types.NamespacedName{
			Namespace: transCtx.Cluster.Namespace,
			Name:      component.FullName(transCtx.Cluster.Name, name),
		}
	)
	dagGet := func() bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		for _, obj := range graphCli.FindAll(dag, &appsv1.Component{}) {
			if client.ObjectKeyFromObject(obj) == compKey {
				return true // TODO: should check the action?
			}
		}
		return false
	}

	comp := &appsv1.Component{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	if !c.expected(comp) {
		return false, nil
	}
	// create or update in DAG?
	if dagGet() {
		return false, nil
	}
	return true, nil
}

func (c *phasePrecondition) shardingMatch(transCtx *clusterTransformContext, dag *graph.DAG, name string) (bool, error) {
	dagList := func() bool {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		for _, obj := range graphCli.FindAll(dag, &appsv1.Component{}) {
			if shardingCompWithName(obj.(*appsv1.Component), name) {
				return true // TODO: should check the action?
			}
		}
		return false
	}

	comps, err := ictrlutil.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, name)
	if err != nil {
		return false, err
	}
	for _, comp := range comps {
		if !c.expected(&comp) {
			return false, nil
		}
	}
	// create or update in DAG?
	if dagList() {
		return false, nil
	}
	return true, nil
}

func (c *phasePrecondition) expected(comp *appsv1.Component) bool {
	if comp.Generation == comp.Status.ObservedGeneration {
		expect := appsv1.RunningClusterCompPhase
		if comp.Spec.Stop != nil && *comp.Spec.Stop {
			expect = appsv1.StoppedClusterCompPhase
		}
		return comp.Status.Phase == expect
	}
	return false
}

type clusterCompNShardingHandler struct {
	op      int
	scaleIn *bool
}

func (h *clusterCompNShardingHandler) handle(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	if transCtx.sharding(name) {
		handler := &clusterShardingHandler{scaleIn: h.scaleIn}
		switch h.op {
		case createOp:
			return handler.create(transCtx, dag, name)
		case deleteOp:
			return handler.delete(transCtx, dag, name)
		default:
			return handler.update(transCtx, dag, name)
		}
	} else {
		handler := &clusterComponentHandler{}
		switch h.op {
		case createOp:
			return handler.create(transCtx, dag, name)
		case deleteOp:
			return handler.delete(transCtx, dag, name)
		default:
			return handler.update(transCtx, dag, name)
		}
	}
}

func predecessors(orders []string, name string) []string {
	var previous []string
	for _, order := range orders {
		names := strings.Split(order, ",")
		if index := slices.Index(names, name); index >= 0 {
			return previous
		}
		previous = names
	}
	panic("runtime error: cannot find predecessor for component or sharding " + name)
}

type clusterParallelHandler struct {
	clusterParallelOrder
	dummyPrecondition
	clusterCompNShardingHandler
}

type orderedCreateNUpdateHandler struct {
	clusterOrderedOrder
	phasePrecondition
	clusterCompNShardingHandler
}

type orderedDeleteHandler struct {
	clusterOrderedOrder
	notExistPrecondition
	clusterCompNShardingHandler
}

func setDiff(s1, s2 sets.Set[string]) (sets.Set[string], sets.Set[string], sets.Set[string]) {
	return s2.Difference(s1), s1.Difference(s2), s1.Intersection(s2)
}

func mapDiff[T interface{}](m1, m2 map[string]T) (sets.Set[string], sets.Set[string], sets.Set[string]) {
	s1, s2 := sets.KeySet(m1), sets.KeySet(m2)
	return setDiff(s1, s2)
}

type clusterComponentHandler struct{}

func (h *clusterComponentHandler) create(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	proto, err := h.protoComp(transCtx, name)
	if err != nil {
		return err
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Create(dag, proto)

	initClusterCompNShardingStatus(transCtx, name)

	return nil
}

func (h *clusterComponentHandler) delete(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	comp, err := h.runningComp(transCtx, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) || model.IsObjectDeleting(comp) {
		return nil
	}
	transCtx.Logger.Info(fmt.Sprintf("deleting component %s", comp.Name))
	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Delete(dag, comp)
	return nil
}

func (h *clusterComponentHandler) update(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	running, err1 := h.runningComp(transCtx, name)
	if err1 != nil {
		return err1
	}
	proto, err2 := h.protoComp(transCtx, name)
	if err2 != nil {
		return err2
	}

	if obj := copyAndMergeComponent(running, proto); obj != nil {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		graphCli.Update(dag, running, obj)
	}
	return nil
}

func (h *clusterComponentHandler) runningComp(transCtx *clusterTransformContext, name string) (*appsv1.Component, error) {
	compKey := types.NamespacedName{
		Namespace: transCtx.Cluster.Namespace,
		Name:      component.FullName(transCtx.Cluster.Name, name),
	}
	comp := &appsv1.Component{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
		return nil, err
	}
	return comp, nil
}

func (h *clusterComponentHandler) protoComp(transCtx *clusterTransformContext, name string) (*appsv1.Component, error) {
	for _, comp := range transCtx.components {
		if comp.Name == name {
			return component.BuildComponent(transCtx.Cluster, comp, nil, nil)
		}
	}
	return nil, fmt.Errorf("cluster component %s not found", name)
}

type clusterShardingHandler struct {
	scaleIn *bool
}

func (h *clusterShardingHandler) create(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	protoComps, err := h.protoComps(transCtx, name)
	if err != nil {
		return err
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i := range protoComps {
		graphCli.Create(dag, protoComps[i])
	}

	initClusterCompNShardingStatus(transCtx, name)

	// TODO:
	//  1. sharding post-provision
	//  2. provision strategy

	return nil
}

func (h *clusterShardingHandler) delete(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	runningComps, err := ictrlutil.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, name)
	if err != nil {
		return err
	}

	// TODO: sharding pre-terminate

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i := range runningComps {
		h.deleteComp(transCtx, graphCli, dag, &runningComps[i], nil)
	}
	return nil
}

func (h *clusterShardingHandler) deleteComp(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, comp *appsv1.Component, scaleIn *bool) {
	if !model.IsObjectDeleting(comp) {
		transCtx.Logger.Info(fmt.Sprintf("deleting sharding component %s", comp.Name))

		vertex := graphCli.Do(dag, nil, comp, model.ActionDeletePtr(), nil)
		if scaleIn != nil && *scaleIn {
			compCopy := comp.DeepCopy()
			if comp.Annotations == nil {
				compCopy.Annotations = make(map[string]string)
			}
			compCopy.Annotations[constant.ComponentScaleInAnnotationKey] = trueVal
			graphCli.Do(dag, compCopy, comp, model.ActionUpdatePtr(), vertex)
		}
	}
}

func (h *clusterShardingHandler) update(transCtx *clusterTransformContext, dag *graph.DAG, name string) error {
	runningComps, err1 := ictrlutil.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, name)
	if err1 != nil {
		return err1
	}

	runningCompsMap := make(map[string]*appsv1.Component)
	for i, comp := range runningComps {
		runningCompsMap[comp.Name] = &runningComps[i]
	}

	protoComps, err2 := h.protoComps(transCtx, name)
	if err2 != nil {
		return err2
	}
	protoCompsMap := make(map[string]*appsv1.Component)
	for i, comp := range protoComps {
		protoCompsMap[comp.Name] = protoComps[i]
	}

	toCreate, toDelete, toUpdate := mapDiff(runningCompsMap, protoCompsMap)

	// TODO: update strategy

	h.deleteComps(transCtx, dag, runningCompsMap, toDelete)
	h.updateComps(transCtx, dag, runningCompsMap, protoCompsMap, toUpdate)
	h.createComps(transCtx, dag, protoCompsMap, toCreate)

	return nil
}

func (h *clusterShardingHandler) createComps(transCtx *clusterTransformContext, dag *graph.DAG,
	protoComps map[string]*appsv1.Component, createSet sets.Set[string]) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for name := range createSet {
		graphCli.Create(dag, protoComps[name])
		// TODO: shard post-provision
	}
}

func (h *clusterShardingHandler) deleteComps(transCtx *clusterTransformContext, dag *graph.DAG,
	runningComps map[string]*appsv1.Component, deleteSet sets.Set[string]) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for name := range deleteSet {
		// TODO: shard pre-terminate
		h.deleteComp(transCtx, graphCli, dag, runningComps[name], h.scaleIn)
	}
}

func (h *clusterShardingHandler) updateComps(transCtx *clusterTransformContext, dag *graph.DAG,
	runningComps map[string]*appsv1.Component, protoComps map[string]*appsv1.Component, updateSet sets.Set[string]) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for name := range updateSet {
		running, proto := runningComps[name], protoComps[name]
		if obj := copyAndMergeComponent(running, proto); obj != nil {
			graphCli.Update(dag, running, obj)
		}
	}
}

func (h *clusterShardingHandler) protoComps(transCtx *clusterTransformContext, name string) ([]*appsv1.Component, error) {
	build := func(sharding *appsv1.ClusterSharding) ([]*appsv1.Component, error) {
		labels := map[string]string{
			constant.KBAppShardingNameLabelKey: sharding.Name,
		}
		if len(sharding.ShardingDef) > 0 {
			labels[constant.ShardingDefLabelKey] = sharding.ShardingDef
		}

		objs := make([]*appsv1.Component, 0)

		shardingComps := transCtx.shardingComps[sharding.Name]
		for i := range shardingComps {
			spec := shardingComps[i]
			var annotations map[string]string
			if transCtx.annotations != nil {
				annotations = transCtx.annotations[spec.Name]
			}
			obj, err := component.BuildComponent(transCtx.Cluster, spec, labels, annotations)
			if err != nil {
				return nil, err
			}
			objs = append(objs, obj)
		}
		return objs, nil
	}

	for _, sharding := range transCtx.shardings {
		if sharding.Name == name {
			return build(sharding)
		}
	}
	return nil, fmt.Errorf("cluster sharding %s not found", name)
}

func initClusterCompNShardingStatus(transCtx *clusterTransformContext, name string) {
	var (
		cluster = transCtx.Cluster
	)
	m := &cluster.Status.Components
	if transCtx.sharding(name) {
		m = &cluster.Status.Shardings
	}
	if *m == nil {
		*m = make(map[string]appsv1.ClusterComponentStatus)
	}
	(*m)[name] = appsv1.ClusterComponentStatus{}
}

func clusterRunningCompNShardingSet(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) (sets.Set[string], error) {
	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return nil, err
	}

	names := sets.Set[string]{}
	for _, comp := range compList.Items {
		if shardingName := shardingCompNName(&comp); len(shardingName) > 0 {
			names.Insert(shardingName)
		} else {
			name, err := component.ShortName(cluster.Name, comp.Name)
			if err != nil {
				return nil, err
			}
			names.Insert(name)
		}
	}
	return names, nil
}

func shardingCompWithName(comp *appsv1.Component, shardingName string) bool {
	if comp == nil || comp.Labels == nil {
		return false
	}
	name, ok := comp.Labels[constant.KBAppShardingNameLabelKey]
	return ok && name == shardingName
}

func shardingCompNName(comp *appsv1.Component) string {
	if comp != nil && comp.Labels != nil {
		name, ok := comp.Labels[constant.KBAppShardingNameLabelKey]
		if ok {
			return name
		}
	}
	return ""
}
