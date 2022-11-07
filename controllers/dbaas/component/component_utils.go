package component

import (
	"encoding/json"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	// OpsRequestReconcileAnnotationKey Notify OpsRequest to reconcile
	OpsRequestReconcileAnnotationKey = "kubeblocks.io/reconcile"
)

type handleComponentAndCheckStatus func(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, object client.Object) (bool, error)

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
	return markRunningOpsRequestAnnotation(reqCtx, cli, cluster)
}

// needSyncStatusComponents Determine whether the component status needs to be modified
func needSyncStatusComponents(cluster *dbaasv1alpha1.Cluster, componentName string, componentIsRunning bool) bool {
	var (
		ok              bool
		statusComponent *dbaasv1alpha1.ClusterStatusComponent
	)
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = cluster.Status.Components[componentName]; !ok {
		cluster.Status.Components[componentName] = &dbaasv1alpha1.ClusterStatusComponent{Phase: cluster.Status.Phase}
		return true
	}
	// if componentIsRunning is false, means the cluster has an operation running.
	// so we sync the cluster phase to component phase when cluster phase is not Running.
	if cluster.Status.Phase != dbaasv1alpha1.RunningPhase && !componentIsRunning && statusComponent.Phase == dbaasv1alpha1.RunningPhase {
		statusComponent.Phase = cluster.Status.Phase
		return true
	}
	// if componentIsRunning is true and component status is not Running.
	// we should change component phase to Running
	if statusComponent.Phase != dbaasv1alpha1.RunningPhase && componentIsRunning {
		statusComponent.Phase = dbaasv1alpha1.RunningPhase
		return true
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
	if ok := needSyncStatusComponents(cluster, componentName, componentIsRunning); !ok {
		return nil
	}
	reqCtx.Log.Info("component phase changed", "componentName", componentName, "phase", cluster.Status.Components[componentName].Phase)
	return cli.Status().Patch(reqCtx.Ctx, cluster, patch)
}

// patchOpsRequestAnnotation patch the reconcile annotation to OpsRequest
func patchOpsRequestAnnotation(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, opsRequestName string) error {
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: opsRequestName, Namespace: cluster.Namespace}, opsRequest); err != nil {
		return err
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Annotations == nil {
		opsRequest.Annotations = map[string]string{}
	}
	opsRequest.Annotations[OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339)
	return cli.Patch(reqCtx.Ctx, opsRequest, patch)
}

// MarkRunningOpsRequestAnnotation mark reconcile annotation to the OpsRequest running in the current cluster.
// then the related OpsRequest can reconcile
func markRunningOpsRequestAnnotation(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster) error {
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
	// mark annotation for updating operations
	for k, v := range opsRequestMap {
		if k != dbaasv1alpha1.UpdatingPhase {
			continue
		}
		if err = patchOpsRequestAnnotation(reqCtx, cli, cluster, v); err != nil {
			return err
		}
	}
	return nil
}

// filterLabels filter the resources according to labels
func filterLabels(object client.Object) bool {
	matchLabels := []string{intctrlutil.AppInstanceLabelKey, intctrlutil.AppComponentLabelKey}
	objLabels := object.GetLabels()
	if objLabels == nil {
		return false
	}
	for _, l := range matchLabels {
		if _, ok := objLabels[l]; !ok {
			return false
		}
	}
	return true
}
