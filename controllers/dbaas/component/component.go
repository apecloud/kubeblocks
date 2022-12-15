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

package component

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/consensusset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/stateful"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/stateless"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// Component is the interface to use for component status
type Component interface {
	// IsRunning when relevant k8s workloads changes, check whether the cluster is running.
	// you can also reconcile the pods of component util the component is Running here.
	IsRunning(obj client.Object) (bool, error)

	// PodsReady check whether all pods of the component are ready.
	PodsReady(obj client.Object) (bool, error)

	// HandleProbeTimeoutWhenPodsReady if the component need role probe and the pods of component are ready,
	// we should handle the component phase when the role probe timeout and return a bool.
	// if return true, means probe has not timed out and need to requeue after an interval time to handle probe timeout again.
	// else return false, means probe has timed out and need to update the component phase to Failed or Abnormal.
	HandleProbeTimeoutWhenPodsReady() (bool, error)

	// CalculatePhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error)
}

// NewComponentByType new a component object by cluster, clusterDefinition and componentName
func NewComponentByType(
	ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent,
	componentName string) Component {
	switch componentDef.ComponentType {
	case dbaasv1alpha1.Consensus:
		component := util.GetComponentByName(cluster, componentName)
		return consensusset.NewConsensusSet(ctx, cli, cluster, component, componentDef)
	case dbaasv1alpha1.Stateful:
		return stateful.NewStateful(ctx, cli, cluster)
	case dbaasv1alpha1.Stateless:
		return stateless.NewStateless(ctx, cli, cluster)
	}
	return nil
}

// handleComponentStatusAndSyncCluster handle component status. if the component status changed, sync cluster.status.components
func handleComponentStatusAndSyncCluster(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	obj client.Object,
	cluster *dbaasv1alpha1.Cluster,
	component Component) (time.Duration, error) {
	var (
		err                  error
		labels               = obj.GetLabels()
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
	if requeueWhenPodsReady, err = component.HandleProbeTimeoutWhenPodsReady(); err != nil {
		return requeueAfter, nil
	}
	if err = patchClusterComponentStatus(reqCtx, cli, cluster, component,
		labels[intctrlutil.AppComponentLabelKey], isRunning, podsReady); err != nil {
		return requeueAfter, err
	}

	if isRunning {
		return requeueAfter, util.MarkRunningOpsRequestAnnotation(reqCtx.Ctx, cli, cluster)
	}
	if requeueWhenPodsReady {
		requeueAfter = time.Minute
	}
	return requeueAfter, nil
}

// patchClusterComponentStatus patch Cluster.status.component status
func patchClusterComponentStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component Component,
	componentName string,
	componentIsRunning, podsIsReady bool) error {
	// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
	patch := client.MergeFrom(cluster.DeepCopy())
	if ok, err := NeedSyncStatusComponents(cluster, component,
		componentName, componentIsRunning, podsIsReady); err != nil || !ok {
		return err
	}
	reqCtx.Log.Info("component status changed", "componentName", componentName, "phase", cluster.Status.Components[componentName].Phase)
	return cli.Status().Patch(reqCtx.Ctx, cluster, patch)
}

// NeedSyncStatusComponents Determine whether the component status needs to be modified
func NeedSyncStatusComponents(cluster *dbaasv1alpha1.Cluster,
	component Component,
	componentName string,
	componentIsRunning, podsIsReady bool) (bool, error) {
	var (
		status          = &cluster.Status
		ok              bool
		statusComponent dbaasv1alpha1.ClusterStatusComponent
		podsReadyTime   *metav1.Time
	)
	if podsIsReady {
		podsReadyTime = &metav1.Time{Time: time.Now()}
	}
	if status.Components == nil {
		status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = status.Components[componentName]; !ok {
		status.Components[componentName] = dbaasv1alpha1.ClusterStatusComponent{Phase: cluster.Status.Phase,
			PodsReady: &podsIsReady, PodsReadyTime: podsReadyTime}
		return true, nil
	}
	var needSync bool
	if !componentIsRunning {
		if statusComponent.Phase == dbaasv1alpha1.RunningPhase {
			// if cluster.status.phase is Updating or OpsRequest of cluster scope is Running.
			// so we sync the cluster phase to component phase.
			// TODO check cluster status what means cluster scope OpsRequests are running
			if cluster.Status.Phase == dbaasv1alpha1.UpdatingPhase {
				statusComponent.Phase = cluster.Status.Phase
				needSync = true
			}

			// if no operations are running in cluster, means the component is Failed or Abnormal.
			if util.IsCompleted(cluster.Status.Phase) {
				if phase, err := component.CalculatePhaseWhenPodsNotReady(componentName); err != nil {
					return false, err
				} else if phase != "" {
					statusComponent.Phase = cluster.Status.Phase
					needSync = true
				}
			}
		}
	} else {
		if statusComponent.Phase != dbaasv1alpha1.RunningPhase {
			// if componentIsRunning is true and component status is not Running.
			// we should change component phase to Running
			statusComponent.Phase = dbaasv1alpha1.RunningPhase
			statusComponent.Message = nil
			needSync = true
		}
	}
	if statusComponent.PodsReady != &podsIsReady {
		statusComponent.PodsReadyTime = podsReadyTime
		needSync = true
	}
	statusComponent.PodsReady = &podsIsReady
	status.Components[componentName] = statusComponent
	return needSync, nil
}
