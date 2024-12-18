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

	"github.com/go-logr/logr"
	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterTransformContext a graph.TransformContext implementation for Cluster reconciliation
type clusterTransformContext struct {
	context.Context
	Client client.Reader
	record.EventRecorder
	logr.Logger

	Cluster     *appsv1.Cluster
	OrigCluster *appsv1.Cluster

	clusterDef    *appsv1.ClusterDefinition
	shardingDefs  map[string]*appsv1.ShardingDefinition
	componentDefs map[string]*appsv1.ComponentDefinition

	// consolidated components and shardings from topology and/or user-specified
	components []*appsv1.ClusterComponentSpec
	shardings  []*appsv1.ClusterSharding

	shardingComps map[string][]*appsv1.ClusterComponentSpec // comp specs for each sharding

	// TODO: remove this, annotations to be added to components for sharding, mapping with @allComps.
	annotations map[string]map[string]string
}

// clusterPlanBuilder a graph.PlanBuilder implementation for Cluster reconciliation
type clusterPlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *clusterTransformContext
	transformers graph.TransformerChain
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *clusterTransformContext
}

var _ graph.TransformContext = &clusterTransformContext{}
var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

// TransformContext implementation

func (c *clusterTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *clusterTransformContext) GetClient() client.Reader {
	return c.Client
}

func (c *clusterTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *clusterTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

// sharding use to determine if the entity is sharding or component
func (c *clusterTransformContext) sharding(name string) (bool, error) {
	// fast return when the object has already been initialized
	if len(c.components) > 0 || len(c.shardings) > 0 {
		_, ok := c.shardingComps[name]
		return ok, nil
	}

	// sharding defined in cd topology
	if len(c.Cluster.Spec.ClusterDef) > 0 && len(c.Cluster.Spec.Topology) > 0 {
		if c.clusterDef == nil {
			return false, fmt.Errorf("clusterDefinition is not initialized")
		}
		for _, topo := range c.clusterDef.Spec.Topologies {
			if c.Cluster.Spec.Topology != topo.Name {
				continue
			}
			for _, sharding := range topo.Shardings {
				if sharding.Name == name {
					return true, nil
				}
			}
			return false, nil
		}
		return false, fmt.Errorf("topology %s not found in ClusterDefinition %s", c.Cluster.Spec.Topology, c.Cluster.Spec.ClusterDef)
	}

	// sharding defined in cluster.spec.shardings
	for _, sharding := range c.Cluster.Spec.Shardings {
		if sharding.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (c *clusterTransformContext) total() int {
	cnt := len(c.components)
	for _, comps := range c.shardingComps {
		cnt += len(comps)
	}
	return cnt
}

func (c *clusterTransformContext) traverse(f func(spec *appsv1.ClusterComponentSpec)) {
	if f != nil {
		for _, comp := range c.components {
			f(comp)
		}
		for _, comps := range c.shardingComps {
			for _, comp := range comps {
				f(comp)
			}
		}
	}
}

func init() {
	model.AddScheme(appsv1alpha1.AddToScheme)
	model.AddScheme(appsv1beta1.AddToScheme)
	model.AddScheme(appsv1.AddToScheme)
	model.AddScheme(dpv1alpha1.AddToScheme)
	model.AddScheme(snapshotv1.AddToScheme)
	model.AddScheme(snapshotv1beta1.AddToScheme)
	model.AddScheme(extensionsv1alpha1.AddToScheme)
	model.AddScheme(workloadsv1.AddToScheme)
}

// PlanBuilder implementation

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1.Cluster{}
	if err := c.cli.Get(c.transCtx.Context, c.req.NamespacedName, cluster); err != nil {
		return err
	}
	c.AddTransformer(&clusterInitTransformer{cluster: cluster})
	return nil
}

func (c *clusterPlanBuilder) AddTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, transformer...)
	return c
}

func (c *clusterPlanBuilder) AddParallelTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, &ParallelTransformers{transformers: transformer})
	return c
}

// Build runs all transformers to generate a plan
func (c *clusterPlanBuilder) Build() (graph.Plan, error) {
	var err error
	defer func() {
		// set apply resource condition
		// if cluster is being deleted, no need to set apply resource condition
		if c.transCtx.Cluster.IsDeleting() {
			return
		}
		preCheckCondition := meta.FindStatusCondition(c.transCtx.Cluster.Status.Conditions, appsv1.ConditionTypeProvisioningStarted)
		if preCheckCondition == nil {
			// this should not happen
			return
		}
		// if pre-check failed, this is a fast return, no need to set apply resource condition
		if preCheckCondition.Status != metav1.ConditionTrue {
			sendWarningEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonPreCheckFailed, err)
			return
		}
		setApplyResourceCondition(&c.transCtx.Cluster.Status.Conditions, c.transCtx.Cluster.Generation, err)
		sendWarningEventWithError(c.transCtx.GetRecorder(), c.transCtx.Cluster, ReasonApplyResourcesFailed, err)
	}()

	// new a DAG and apply chain on it
	dag := graph.NewDAG()
	err = c.transformers.ApplyTo(c.transCtx, dag)
	c.transCtx.Logger.V(1).Info(fmt.Sprintf("DAG: %s", dag))

	// construct execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFuncWithLogging,
		cli:      c.cli,
		transCtx: c.transCtx,
	}
	return plan, err
}

// Plan implementation

func (p *clusterPlan) Execute() error {
	err := p.dag.WalkReverseTopoOrder(p.walkFunc, nil)
	if err != nil {
		if hErr := p.handlePlanExecutionError(err); hErr != nil {
			return hErr
		}
	}
	return err
}

func (p *clusterPlan) handlePlanExecutionError(err error) error {
	clusterCopy := p.transCtx.OrigCluster.DeepCopy()
	condition := newFailedApplyResourcesCondition(err)
	meta.SetStatusCondition(&clusterCopy.Status.Conditions, condition)
	return p.cli.Status().Patch(p.transCtx.Context, clusterCopy, client.MergeFrom(p.transCtx.OrigCluster))
}

// Do the real works

// newClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
func newClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client) graph.PlanBuilder {
	return &clusterPlanBuilder{
		req: ctx.Req,
		cli: cli,
		transCtx: &clusterTransformContext{
			Context:       ctx.Ctx,
			Client:        model.NewGraphClient(cli),
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

func (c *clusterPlanBuilder) defaultWalkFuncWithLogging(vertex graph.Vertex) error {
	node, ok := vertex.(*model.ObjectVertex)
	err := c.defaultWalkFunc(vertex)
	switch {
	case err == nil:
		return err
	case !ok:
		c.transCtx.Logger.Error(err, "")
	case node.Action == nil:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%T", node))
	case apierrors.IsConflict(err):
		return err
	default:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%s %T error", *node.Action, node.Obj))
	}
	return err
}

func (c *clusterPlanBuilder) defaultWalkFunc(vertex graph.Vertex) error {
	node, ok := vertex.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", vertex)
	}
	if node.Action == nil {
		return fmt.Errorf("node action can't be nil")
	}

	// cluster object has more business to do, handle them here
	if _, ok = node.Obj.(*appsv1.Cluster); ok {
		if err := c.reconcileCluster(node); err != nil {
			return err
		}
	}
	return c.reconcileObject(node)
}

func (c *clusterPlanBuilder) reconcileCluster(node *model.ObjectVertex) error {
	cluster := node.Obj.(*appsv1.Cluster).DeepCopy()
	origCluster := node.OriObj.(*appsv1.Cluster)
	switch *node.Action {
	// cluster.meta and cluster.spec might change
	case model.STATUS:
		if !reflect.DeepEqual(cluster.ObjectMeta, origCluster.ObjectMeta) || !reflect.DeepEqual(cluster.Spec, origCluster.Spec) {
			patch := client.MergeFrom(origCluster.DeepCopy())
			if err := c.cli.Patch(c.transCtx.Context, cluster, patch); err != nil {
				return err
			}
		}
	case model.CREATE, model.UPDATE:
		return fmt.Errorf("cluster can't be created or updated: %s", cluster.Name)
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileObject(node *model.ObjectVertex) error {
	ctx := c.transCtx.Context
	switch *node.Action {
	case model.CREATE:
		return c.reconcileCreateObject(ctx, node)
	case model.UPDATE:
		return c.reconcileUpdateObject(ctx, node)
	case model.PATCH:
		return c.reconcilePatchObject(ctx, node)
	case model.DELETE:
		return c.reconcileDeleteObject(ctx, node)
	case model.STATUS:
		return c.reconcileStatusObject(ctx, node)
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileCreateObject(ctx context.Context, node *model.ObjectVertex) error {
	err := c.cli.Create(ctx, node.Obj, clientOption(node))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileUpdateObject(ctx context.Context, node *model.ObjectVertex) error {
	err := c.cli.Update(ctx, node.Obj, clientOption(node))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *clusterPlanBuilder) reconcilePatchObject(ctx context.Context, node *model.ObjectVertex) error {
	patch := client.MergeFrom(node.OriObj)
	err := c.cli.Patch(ctx, node.Obj, patch, clientOption(node))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileDeleteObject(ctx context.Context, node *model.ObjectVertex) error {
	if controllerutil.RemoveFinalizer(node.Obj, constant.DBClusterFinalizerName) {
		err := c.cli.Update(ctx, node.Obj, clientOption(node))
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	backgroundDeleteObject := func() error {
		deletePropagation := metav1.DeletePropagationBackground
		deleteOptions := &client.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		}
		if err := c.cli.Delete(ctx, node.Obj, deleteOptions, clientOption(node)); err != nil {
			return client.IgnoreNotFound(err)
		}
		return nil
	}
	// delete secondary objects
	if _, ok := node.Obj.(*appsv1.Cluster); !ok {
		err := backgroundDeleteObject()
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileStatusObject(ctx context.Context, node *model.ObjectVertex) error {
	patch := client.MergeFrom(node.OriObj)
	if err := c.cli.Status().Patch(ctx, node.Obj, patch, clientOption(node)); err != nil {
		return err
	}
	// handle condition and phase changing triggered events
	if newCluster, ok := node.Obj.(*appsv1.Cluster); ok {
		oldCluster, _ := node.OriObj.(*appsv1.Cluster)
		c.emitConditionUpdatingEvent(oldCluster.Status.Conditions, newCluster.Status.Conditions)
		c.emitStatusUpdatingEvent(oldCluster.Status, newCluster.Status)
	}
	return nil
}

func (c *clusterPlanBuilder) emitConditionUpdatingEvent(oldConditions, newConditions []metav1.Condition) {
	for _, newCondition := range newConditions {
		oldCondition := meta.FindStatusCondition(oldConditions, newCondition.Type)
		// filtered in cluster creation
		if oldCondition == nil && newCondition.Status == metav1.ConditionFalse {
			return
		}
		if !reflect.DeepEqual(oldCondition, &newCondition) {
			eType := corev1.EventTypeNormal
			if newCondition.Status == metav1.ConditionFalse {
				eType = corev1.EventTypeWarning
			}
			c.transCtx.EventRecorder.Event(c.transCtx.Cluster, eType, newCondition.Reason, newCondition.Message)
		}
	}
}

func (c *clusterPlanBuilder) emitStatusUpdatingEvent(oldStatus, newStatus appsv1.ClusterStatus) {
	cluster := c.transCtx.Cluster
	newPhase := newStatus.Phase
	if newPhase == oldStatus.Phase {
		return
	}
	eType := corev1.EventTypeNormal
	message := ""
	switch newPhase {
	case appsv1.RunningClusterPhase:
		message = fmt.Sprintf("Cluster: %s is ready, current phase is %s", cluster.Name, newPhase)
	case appsv1.StoppedClusterPhase:
		message = fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
	case appsv1.FailedClusterPhase, appsv1.AbnormalClusterPhase:
		message = fmt.Sprintf("Cluster: %s is %s, check according to the components message", cluster.Name, newPhase)
		eType = corev1.EventTypeWarning
	}
	if len(message) > 0 {
		c.transCtx.EventRecorder.Event(cluster, eType, string(newPhase), message)
	}
}
