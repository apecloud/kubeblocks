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
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/operations"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type handleComponentAndCheckStatus func(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, object client.Object) (bool, error)

// NeedSyncStatusComponents Determine whether the component status needs to be modified
func NeedSyncStatusComponents(cluster *dbaasv1alpha1.Cluster, componentName string, componentIsRunning bool) bool {
	var (
		status          = &cluster.Status
		ok              bool
		statusComponent dbaasv1alpha1.ClusterStatusComponent
	)
	if status.Components == nil {
		status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = status.Components[componentName]; !ok {
		status.Components[componentName] = dbaasv1alpha1.ClusterStatusComponent{Phase: cluster.Status.Phase}
		return true
	}

	if !componentIsRunning {
		// if cluster.status.phase is Updating, means the cluster has an operation running.
		// so we sync the cluster phase to component phase.
		if cluster.Status.Phase == dbaasv1alpha1.UpdatingPhase && statusComponent.Phase == dbaasv1alpha1.RunningPhase {
			statusComponent.Phase = cluster.Status.Phase
			status.Components[componentName] = statusComponent
			return true
		}
		// TODO handle when the deployment/statefulSet/pod is deleted by k8s controller or users.
	} else {
		// if componentIsRunning is true and component status is not Running.
		// we should change component phase to Running
		if statusComponent.Phase != dbaasv1alpha1.RunningPhase {
			statusComponent.Phase = dbaasv1alpha1.RunningPhase
			statusComponent.Message = ""
			status.Components[componentName] = statusComponent
			return true
		}
	}
	return false
}

// patchClusterComponentStatus patch Cluster.status.component status
func patchClusterComponentStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	componentName string,
	componentIsRunning bool) error {
	// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
	patch := client.MergeFrom(cluster.DeepCopy())
	if ok := NeedSyncStatusComponents(cluster, componentName, componentIsRunning); !ok {
		return nil
	}
	reqCtx.Log.Info("component phase changed", "componentName", componentName, "phase", cluster.Status.Components[componentName].Phase)
	return cli.Status().Patch(reqCtx.Ctx, cluster, patch)
}

// MarkRunningOpsRequestAnnotation mark reconcile annotation to the OpsRequest which is running in the cluster.
// then the related OpsRequest can reconcile
func MarkRunningOpsRequestAnnotation(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster) error {
	var (
		opsRequestMap   map[dbaasv1alpha1.Phase]string
		opsRequestValue string
		ok              bool
		err             error
	)
	if cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = cluster.Annotations[intctrlutil.OpsRequestAnnotationKey]; !ok {
		return nil
	}
	if err = json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return err
	}
	// mark annotation for operations
	for _, v := range opsRequestMap {
		if err = operations.PatchOpsRequestAnnotation(ctx, cli, cluster, v); err != nil {
			return err
		}
	}
	return nil
}

// checkComponentStatusAndSyncCluster check component status. if the component status changed, sync cluster.status.components
func checkComponentStatusAndSyncCluster(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	object client.Object,
	customFunc handleComponentAndCheckStatus) error {
	var (
		componentIsRunning bool
		err                error
		cluster            = &dbaasv1alpha1.Cluster{}
		labels             = object.GetLabels()
	)

	if labels == nil {
		return nil
	}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: labels[intctrlutil.AppInstanceLabelKey], Namespace: object.GetNamespace()}, cluster); err != nil {
		return err
	}
	if customFunc == nil {
		return nil
	}
	if componentIsRunning, err = customFunc(reqCtx, cli, cluster, object); err != nil {
		return err
	}
	if err = patchClusterComponentStatus(reqCtx, cli, cluster, labels[intctrlutil.AppComponentLabelKey], componentIsRunning); err != nil {
		return err
	}
	if componentIsRunning {
		return MarkRunningOpsRequestAnnotation(reqCtx.Ctx, cli, cluster)
	}
	return nil
}

// DeploymentIsReady check deployment is ready
func DeploymentIsReady(deploy *appsv1.Deployment) bool {
	var (
		targetReplicas     = *deploy.Spec.Replicas
		componentIsRunning = true
	)
	if deploy.Status.AvailableReplicas != targetReplicas ||
		deploy.Status.Replicas != targetReplicas ||
		deploy.Status.ObservedGeneration != deploy.GetGeneration() {
		componentIsRunning = false
	}
	return componentIsRunning
}
