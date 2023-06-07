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

package lifecycle

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO: cluster plan builder can be abstracted as a common flow

// ClusterTransformContext a graph.TransformContext implementation for Cluster reconciliation
type ClusterTransformContext struct {
	context.Context
	Client roclient.ReadonlyClient
	record.EventRecorder
	logr.Logger
	Cluster     *appsv1alpha1.Cluster
	OrigCluster *appsv1alpha1.Cluster
	ClusterDef  *appsv1alpha1.ClusterDefinition
	ClusterVer  *appsv1alpha1.ClusterVersion
}

// clusterPlanBuilder a graph.PlanBuilder implementation for Cluster reconciliation
type clusterPlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *ClusterTransformContext
	transformers graph.TransformerChain
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *ClusterTransformContext
}

var _ graph.TransformContext = &ClusterTransformContext{}
var _ graph.PlanBuilder = &clusterPlanBuilder{}
var _ graph.Plan = &clusterPlan{}

// TransformContext implementation

func (c *ClusterTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *ClusterTransformContext) GetClient() roclient.ReadonlyClient {
	return c.Client
}

func (c *ClusterTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *ClusterTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

// PlanBuilder implementation

func (c *clusterPlanBuilder) Init() error {
	cluster := &appsv1alpha1.Cluster{}
	if err := c.cli.Get(c.transCtx.Context, c.req.NamespacedName, cluster); err != nil {
		return err
	}

	c.transCtx.Cluster = cluster
	c.transCtx.OrigCluster = cluster.DeepCopy()
	c.transformers = append(c.transformers, &initTransformer{
		cluster:       c.transCtx.Cluster,
		originCluster: c.transCtx.OrigCluster,
	})
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
		preCheckCondition := meta.FindStatusCondition(c.transCtx.Cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
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
	err := p.dag.WalkReverseTopoOrder(p.walkFunc)
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
	sendWarningEventWithError(p.transCtx.GetRecorder(), clusterCopy, ReasonApplyResourcesFailed, err)
	return p.cli.Status().Patch(p.transCtx.Context, clusterCopy, client.MergeFrom(p.transCtx.OrigCluster))
}

// Do the real works

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &clusterPlanBuilder{
		req: req,
		cli: cli,
		transCtx: &ClusterTransformContext{
			Context:       ctx.Ctx,
			Client:        cli,
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

func (c *clusterPlanBuilder) defaultWalkFuncWithLogging(vertex graph.Vertex) error {
	node, ok := vertex.(*ictrltypes.LifecycleVertex)
	err := c.defaultWalkFunc(vertex)
	if err != nil {
		if !ok {
			c.transCtx.Logger.Error(err, "")
		} else {
			if node.Action == nil {
				c.transCtx.Logger.Error(err, fmt.Sprintf("%T", node))
			} else {
				c.transCtx.Logger.Error(err, fmt.Sprintf("%s %T error", *node.Action, node.Obj))
			}
		}
	}
	return err
}

// TODO: retry strategy on error
func (c *clusterPlanBuilder) defaultWalkFunc(vertex graph.Vertex) error {
	node, ok := vertex.(*ictrltypes.LifecycleVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", vertex)
	}
	if node.Action == nil {
		return errors.New("node action can't be nil")
	}

	// cluster object has more business to do, handle them here
	if _, ok = node.Obj.(*appsv1alpha1.Cluster); ok {
		if err := c.reconcileCluster(node); err != nil {
			return err
		}
	}
	return c.reconcileObject(node)
}

func (c *clusterPlanBuilder) reconcileObject(node *ictrltypes.LifecycleVertex) error {
	switch *node.Action {
	case ictrltypes.CREATE:
		err := c.cli.Create(c.transCtx.Context, node.Obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case ictrltypes.UPDATE:
		if node.Immutable {
			return nil
		}
		err := c.cli.Update(c.transCtx.Context, node.Obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case ictrltypes.PATCH:
		patch := client.MergeFrom(node.ObjCopy)
		if err := c.cli.Patch(c.transCtx.Context, node.Obj, patch); !apierrors.IsNotFound(err) {
			c.transCtx.Logger.Error(err, fmt.Sprintf("patch %T error", node.ObjCopy))
			return err
		}
	case ictrltypes.DELETE:
		if controllerutil.RemoveFinalizer(node.Obj, constant.DBClusterFinalizerName) {
			err := c.cli.Update(c.transCtx.Context, node.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		// delete secondary objects
		if _, ok := node.Obj.(*appsv1alpha1.Cluster); !ok {
			// retain backup for data protection even if the cluster is wiped out.
			if strings.EqualFold(node.Obj.GetLabels()[constant.BackupProtectionLabelKey], constant.BackupRetain) {
				return nil
			}
			err := intctrlutil.BackgroundDeleteObject(c.cli, c.transCtx.Context, node.Obj)
			// err := c.cli.Delete(c.transCtx.Context, node.obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case ictrltypes.STATUS:
		patch := client.MergeFrom(node.ObjCopy)
		if err := c.cli.Status().Patch(c.transCtx.Context, node.Obj, patch); err != nil {
			return err
		}
		// handle condition and phase changing triggered events
		if newCluster, ok := node.Obj.(*appsv1alpha1.Cluster); ok {
			oldCluster, _ := node.ObjCopy.(*appsv1alpha1.Cluster)
			c.emitConditionUpdatingEvent(oldCluster.Status.Conditions, newCluster.Status.Conditions)
			c.emitStatusUpdatingEvent(oldCluster.Status, newCluster.Status)
		}
	case ictrltypes.NOOP:
		// nothing
	}
	return nil
}

func (c *clusterPlanBuilder) reconcileCluster(node *ictrltypes.LifecycleVertex) error {
	cluster := node.Obj.(*appsv1alpha1.Cluster).DeepCopy()
	origCluster := node.ObjCopy.(*appsv1alpha1.Cluster)
	switch *node.Action {
	// cluster.meta and cluster.spec might change
	case ictrltypes.STATUS:
		if !reflect.DeepEqual(cluster.ObjectMeta, origCluster.ObjectMeta) || !reflect.DeepEqual(cluster.Spec, origCluster.Spec) {
			// TODO: we should Update instead of Patch cluster object,
			// TODO: but Update failure happens too frequently as other controllers are updating cluster object too.
			// TODO: use Patch here, revert to Update after refactoring done
			// if err := c.cli.Update(c.ctx.Ctx, cluster); err != nil {
			//	tmpCluster := &appsv1alpha1.Cluster{}
			//	err = c.cli.Get(c.ctx.Ctx,client.ObjectKeyFromObject(origCluster), tmpCluster)
			//	c.ctx.Log.Error(err, fmt.Sprintf("update %T error, orig: %v, curr: %v, api-server: %v", origCluster, origCluster, cluster, tmpCluster))
			//	return err
			// }
			patch := client.MergeFrom(origCluster.DeepCopy())
			if err := c.cli.Patch(c.transCtx.Context, cluster, patch); err != nil {
				// log for debug
				// TODO:(free6om) make error message smaller when refactor done.
				c.transCtx.Logger.Error(err, fmt.Sprintf("patch %T error, orig: %v, curr: %v", origCluster, origCluster, cluster))
				return err
			}
		}
	case ictrltypes.CREATE, ictrltypes.UPDATE:
		return fmt.Errorf("cluster can't be created or updated: %s", cluster.Name)
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

func (c *clusterPlanBuilder) emitStatusUpdatingEvent(oldStatus, newStatus appsv1alpha1.ClusterStatus) {
	cluster := c.transCtx.Cluster
	if !reflect.DeepEqual(oldStatus, newStatus) {
		_ = opsutil.MarkRunningOpsRequestAnnotation(c.transCtx.Context, c.cli, cluster)
	}
	newPhase := newStatus.Phase
	if newPhase == oldStatus.Phase {
		return
	}
	eType := corev1.EventTypeNormal
	message := ""
	switch newPhase {
	case appsv1alpha1.RunningClusterPhase:
		message = fmt.Sprintf("Cluster: %s is ready, current phase is %s", cluster.Name, newPhase)
	case appsv1alpha1.StoppedClusterPhase:
		message = fmt.Sprintf("Cluster: %s stopped successfully.", cluster.Name)
	case appsv1alpha1.FailedClusterPhase, appsv1alpha1.AbnormalClusterPhase:
		message = fmt.Sprintf("Cluster: %s is %s, check according to the components message", cluster.Name, newPhase)
		eType = corev1.EventTypeWarning
	}
	if len(message) > 0 {
		c.transCtx.EventRecorder.Event(cluster, eType, string(newPhase), message)
	}
}
