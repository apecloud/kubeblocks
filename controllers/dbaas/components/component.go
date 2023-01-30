/*
Copyright ApeCloud Inc.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/types"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentContext wrapper for handling component status procedure context parameters.
type componentContext struct {
	reqCtx    intctrlutil.RequestCtx
	cli       client.Client
	recorder  record.EventRecorder
	component types.Component
	obj       client.Object
}

// newComponentContext creates a componentContext object.
func newComponentContext(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	component types.Component,
	obj client.Object) componentContext {
	return componentContext{
		reqCtx:    reqCtx,
		cli:       cli,
		recorder:  recorder,
		component: component,
		obj:       obj,
	}
}

// NewComponentByType creates a component object
func NewComponentByType(
	ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent,
	component *dbaasv1alpha1.ClusterComponent) types.Component {
	switch componentDef.ComponentType {
	case dbaasv1alpha1.Consensus:
		return consensusset.NewConsensusSet(ctx, cli, cluster, component, componentDef)
	case dbaasv1alpha1.Replication:
		return replicationset.NewReplicationSet(ctx, cli, cluster, component, componentDef)
	case dbaasv1alpha1.Stateful:
		return stateful.NewStateful(ctx, cli, cluster)
	case dbaasv1alpha1.Stateless:
		return stateless.NewStateless(ctx, cli, cluster)
	}
	return nil
}

// handleComponentStatusAndSyncCluster handles component status. if the component status changed, sync cluster.status.components
func handleComponentStatusAndSyncCluster(compCtx componentContext,
	workloadSpecIsUpdated bool,
	cluster *dbaasv1alpha1.Cluster) (time.Duration, error) {
	var (
		err                  error
		obj                  = compCtx.obj
		component            = compCtx.component
		requeueAfter         time.Duration
		podsReady            bool
		isRunning            bool
		requeueWhenPodsReady bool
	)

	if component == nil {
		return requeueAfter, nil
	}
	if podsReady, err = component.PodsReady(obj); err != nil {
		return requeueAfter, nil
	}
	if isRunning, err = component.IsRunning(obj); err != nil {
		return requeueAfter, nil
	}
	if podsReady {
		if requeueWhenPodsReady, err = component.HandleProbeTimeoutWhenPodsReady(compCtx.recorder); err != nil {
			return requeueAfter, nil
		}
	}

	if err = patchClusterComponentStatus(compCtx, cluster, workloadSpecIsUpdated,
		obj.GetLabels()[intctrlutil.AppComponentLabelKey], isRunning, podsReady); err != nil {
		return requeueAfter, err
	}

	if requeueWhenPodsReady {
		requeueAfter = time.Minute
	}

	return requeueAfter, opsutil.MarkRunningOpsRequestAnnotation(compCtx.reqCtx.Ctx, compCtx.cli, cluster)
}

// patchClusterComponentStatus patches Cluster.status.component status
func patchClusterComponentStatus(
	compCtx componentContext,
	cluster *dbaasv1alpha1.Cluster,
	workloadSpecIsUpdated bool,
	componentName string,
	componentIsRunning, podsAreReady bool) error {
	// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
	patch := client.MergeFrom(cluster.DeepCopy())
	if ok, err := NeedSyncStatusComponents(cluster, compCtx.component,
		componentName, workloadSpecIsUpdated, componentIsRunning, podsAreReady); err != nil || !ok {
		return err
	}
	compCtx.reqCtx.Log.Info("component status changed", "componentName", componentName, "phase", cluster.Status.Components[componentName].Phase, "componentIsRunning", componentIsRunning, "podsAreReady", podsAreReady)
	return compCtx.cli.Status().Patch(compCtx.reqCtx.Ctx, cluster, patch)
}

// NeedSyncStatusComponents Determines whether the component status needs to be modified
func NeedSyncStatusComponents(cluster *dbaasv1alpha1.Cluster,
	component types.Component,
	componentName string,
	workloadSpecIsUpdated,
	componentIsRunning,
	podsAreReady bool) (bool, error) {
	var (
		status          = &cluster.Status
		ok              bool
		statusComponent dbaasv1alpha1.ClusterStatusComponent
		podsReadyTime   *metav1.Time
	)
	if podsAreReady {
		podsReadyTime = &metav1.Time{Time: time.Now()}
	}
	if status.Components == nil {
		status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = status.Components[componentName]; !ok {
		componentType := util.GetComponentTypeName(*cluster, componentName)
		// TODO is it ok to set component status phase as cluster status phase
		status.Components[componentName] = dbaasv1alpha1.ClusterStatusComponent{Phase: cluster.Status.Phase,
			PodsReady: &podsAreReady, PodsReadyTime: podsReadyTime,
			Type: componentType,
		}
		return true, nil
	}
	var needSync bool
	// when the workload spec of the component is updated and cluster phase is Updating,
	// change the component phase to Updating.
	if workloadSpecIsUpdated && cluster.Status.Phase == dbaasv1alpha1.SpecUpdatingPhase {
		statusComponent.Phase = dbaasv1alpha1.SpecUpdatingPhase
		needSync = true
	}
	if !componentIsRunning {
		// if no operation is running in cluster and pods of component are not ready,
		// means the component is Failed or Abnormal.
		if util.IsCompleted(cluster.Status.Phase) {
			if phase, err := component.GetPhaseWhenPodsNotReady(componentName); err != nil {
				return false, err
			} else if phase != "" && statusComponent.Phase != phase {
				statusComponent.Phase = phase
				needSync = true
			}
		}
	} else {
		if statusComponent.Phase != dbaasv1alpha1.RunningPhase &&
			!clusterHandlingSpecForComponent(cluster, statusComponent.Phase) {
			// change component phase to Running when workloads of component are running.
			statusComponent.Phase = dbaasv1alpha1.RunningPhase
			statusComponent.SetMessage(nil)
			needSync = true
		}
	}
	if statusComponent.PodsReady == nil || *statusComponent.PodsReady != podsAreReady {
		statusComponent.PodsReadyTime = podsReadyTime
		needSync = true
	}
	statusComponent.PodsReady = &podsAreReady
	status.Components[componentName] = statusComponent
	return needSync, nil
}

// clusterHandlingSpecForComponent checks if the cluster is handling spec and this component is Updating or doing operation.
func clusterHandlingSpecForComponent(cluster *dbaasv1alpha1.Cluster, componentPhase dbaasv1alpha1.Phase) bool {
	return cluster.Generation != cluster.Status.ObservedGeneration &&
		!util.IsCompleted(componentPhase)
}
