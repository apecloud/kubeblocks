/*
Copyright ApeCloud, Inc.

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

package components

import (
	"context"
	"time"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentContext wrapper for handling component status procedure context parameters.
type componentContext struct {
	reqCtx        intctrlutil.RequestCtx
	cli           client.Client
	recorder      record.EventRecorder
	component     types.Component
	obj           client.Object
	componentSpec *appsv1alpha1.ClusterComponentSpec
}

// newComponentContext creates a componentContext object.
func newComponentContext(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	component types.Component,
	obj client.Object,
	componentSpec *appsv1alpha1.ClusterComponentSpec) componentContext {

	return componentContext{
		reqCtx:        reqCtx,
		cli:           cli,
		recorder:      recorder,
		component:     component,
		obj:           obj,
		componentSpec: componentSpec,
	}
}

func workloadCompClusterReconcile(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	operand client.Object,
	processor func(*appsv1alpha1.ClusterComponentSpec, types.Component) (ctrl.Result, error),
) (ctrl.Result, error) {
	var err error
	var cluster *appsv1alpha1.Cluster

	if cluster, err = util.GetClusterByObject(reqCtx.Ctx, cli, operand); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	} else if cluster == nil {
		return intctrlutil.Reconciled()
	}

	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// create a component object
	componentName := operand.GetLabels()[constant.KBAppComponentLabelKey]
	componentSpec := cluster.GetComponentByName(componentName)
	if componentSpec == nil {
		return intctrlutil.Reconciled()
	}
	componentDef := clusterDef.GetComponentDefByName(componentSpec.ComponentDefRef)
	component := NewComponentByType(reqCtx.Ctx, cli, *cluster, *componentDef, *componentSpec)
	return processor(componentSpec, component)
}

// NewComponentByType creates a component object
func NewComponentByType(
	ctx context.Context,
	cli client.Client,
	cluster appsv1alpha1.Cluster,
	componentDef appsv1alpha1.ClusterComponentDefinition,
	component appsv1alpha1.ClusterComponentSpec) types.Component {
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensusset.NewConsensusSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Replication:
		return replicationset.NewReplicationSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStateful(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStateless(ctx, cli, cluster, component, componentDef)
	default:
		panic("unknown workload type")
	}
}

// updateComponentStatusInClusterStatus updates cluster.Status.Components if the component status changed
func updateComponentStatusInClusterStatus(compCtx componentContext,
	cluster *appsv1alpha1.Cluster) (time.Duration, error) {
	componentStatusSynchronizer := NewClusterStatusSynchronizer(compCtx.reqCtx.Ctx, compCtx.cli, cluster, compCtx.component, compCtx.componentSpec)
	if componentStatusSynchronizer == nil {
		return 0, nil
	}

	wait, err := componentStatusSynchronizer.Update(compCtx.obj, &compCtx.reqCtx.Log, compCtx.recorder)
	if err != nil {
		return 0, err
	}

	var requeueAfter time.Duration
	if wait {
		requeueAfter = time.Minute
	}
	return requeueAfter, opsutil.MarkRunningOpsRequestAnnotation(compCtx.reqCtx.Ctx, compCtx.cli, cluster)
}
