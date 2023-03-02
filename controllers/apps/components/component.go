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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
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

// NewComponentByType creates a component object
func NewComponentByType(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	component *appsv1alpha1.ClusterComponentSpec) types.Component {
	if componentDef == nil {
		return nil
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensusset.NewConsensusSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Replication:
		return replicationset.NewReplicationSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStateful(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStateless(ctx, cli, cluster, component, componentDef)
	}
	return nil
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
