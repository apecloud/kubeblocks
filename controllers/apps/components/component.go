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
	"reflect"
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

// podsOfComponentAreReady checks if the pods of component are ready.
func podsOfComponentAreReady(compCtx componentContext) (*bool, error) {
	if compCtx.componentSpec.Replicas == 0 {
		// if replicas number of component is 0, ignore it and return nil.
		return nil, nil
	}
	podsReadyForComponent, err := compCtx.component.PodsReady(compCtx.obj)
	if err != nil {
		return nil, err
	}
	return &podsReadyForComponent, nil
}

// updateComponentStatusInClusterStatus updates cluster.Status.Components if the component status changed
func updateComponentStatusInClusterStatus(compCtx componentContext,
	cluster *appsv1alpha1.Cluster) (time.Duration, error) {
	var (
		obj          = compCtx.obj
		component    = compCtx.component
		requeueAfter time.Duration
	)

	componentStatusSynchronizer := NewClusterStatusSynchronizer(compCtx.reqCtx.Ctx, compCtx.cli, cluster, compCtx.component, compCtx.componentSpec)
	if componentStatusSynchronizer == nil {
		return 0, nil
	}
	if component == nil {
		return 0, nil
	}
	// handle the components changes
	err := component.HandleUpdate(obj)
	if err != nil {
		return 0, err
	}
	isRunning, err := component.IsRunning(obj)
	if err != nil {
		return 0, err
	}
	podsReady, err := podsOfComponentAreReady(compCtx)
	if err != nil {
		return 0, err
	}

	hasFailedAndTimedOutPod := false
	clusterDeepCopy := cluster.DeepCopy()
	if !isRunning {
		if podsReady != nil && *podsReady {
			// check if the role probe timed out when component phase is not Running but all pods of component are ready.
			if requeueWhenPodsReady, err := component.HandleProbeTimeoutWhenPodsReady(compCtx.recorder); err != nil {
				return 0, err
			} else if requeueWhenPodsReady {
				requeueAfter = time.Minute
			}
		} else {
			// check whether there is a failed pod of component that has timed out
			var hasFailedPod bool
			var message appsv1alpha1.ComponentMessageMap
			hasFailedAndTimedOutPod, hasFailedPod, message = componentStatusSynchronizer.HasFailedAndTimedOutPod()
			if hasFailedAndTimedOutPod {
				componentStatusSynchronizer.UpdateMessage(message)
			} else if hasFailedPod {
				requeueAfter = time.Minute
			}
		}
	}

	if err = componentStatusSynchronizer.UpdateComponentsPhase(isRunning,
		podsReady, hasFailedAndTimedOutPod); err != nil {
		return 0, err
	}

	componentName := compCtx.componentSpec.Name
	oldComponentStatus := clusterDeepCopy.Status.Components[componentName]
	componentStatus := cluster.Status.Components[componentName]
	if !reflect.DeepEqual(oldComponentStatus, componentStatus) {
		compCtx.reqCtx.Log.Info("component status changed", "componentName", componentName, "phase",
			cluster.Status.Components[componentName].Phase, "componentIsRunning", isRunning, "podsAreReady", podsReady)
		patch := client.MergeFrom(clusterDeepCopy)
		if err = compCtx.cli.Status().Patch(compCtx.reqCtx.Ctx, cluster, patch); err != nil {
			return 0, err
		}
	}

	return requeueAfter, opsutil.MarkRunningOpsRequestAnnotation(compCtx.reqCtx.Ctx, compCtx.cli, cluster)
}
