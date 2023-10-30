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

package operations

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type reconfigureAction struct {
}

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1alpha1.GetReconfiguringRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase:                     appsv1alpha1.UpdatingClusterPhase,
		OpsHandler:                         &reAction,
		ProcessingReasonInClusterCondition: ProcessingReasonReconfiguring,
	}
	opsManager.RegisterOps(appsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func handleReconfigureStatusProgress(result *appsv1alpha1.ReconcileDetail, opsStatus *appsv1alpha1.OpsRequestStatus, phase appsv1alpha1.ConfigurationPhase) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationItemStatus) (err error) {
		// the Pending phase is waiting to be executed, and there is currently no valid ReconcileDetail information.
		if result != nil && phase != appsv1alpha1.CPendingPhase {
			cmStatus.LastAppliedStatus = result.ExecResult
			cmStatus.UpdatePolicy = appsv1alpha1.UpgradePolicy(result.Policy)
			cmStatus.SucceedCount = result.SucceedCount
			cmStatus.ExpectedCount = result.ExpectedCount
			cmStatus.Message = result.ErrMessage
			cmStatus.Status = string(phase)
		}
		if cmStatus.SucceedCount != core.Unconfirmed && cmStatus.ExpectedCount != core.Unconfirmed {
			opsStatus.Progress = getSlowestReconfiguringProgress(opsStatus.ReconfiguringStatus.ConfigurationStatus)
		}
		return
	}
}

func handleNewReconfigureRequest(configPatch *core.ConfigPatchInfo, lastAppliedConfigs map[string]string) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationItemStatus) (err error) {
		cmStatus.Status = appsv1alpha1.ReasonReconfigurePersisted
		cmStatus.LastAppliedConfiguration = lastAppliedConfigs
		if configPatch != nil {
			cmStatus.UpdatedParameters = appsv1alpha1.UpdatedParameters{
				AddedKeys:   i2sMap(configPatch.AddConfig),
				UpdatedKeys: b2sMap(configPatch.UpdateConfig),
				DeletedKeys: i2sMap(configPatch.DeleteConfig),
			}
		}
		return
	}
}

func (r *reconfigureAction) syncDependResources(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*intctrlutil.Fetcher, error) {
	ops := &opsRes.OpsRequest.Spec
	configSpec := ops.Reconfigure.Configurations[0]
	fetcher := intctrlutil.NewResourceFetcher(&intctrlutil.ResourceCtx{
		Context:       reqCtx.Ctx,
		Client:        cli,
		Namespace:     opsRes.Cluster.Namespace,
		ClusterName:   ops.ClusterRef,
		ComponentName: ops.Reconfigure.ComponentName,
	})

	err := fetcher.Cluster().
		ClusterDef().
		ClusterVer().
		Configuration().
		ConfigMap(configSpec.Name).
		Complete()
	if err != nil {
		return nil, err
	}
	return fetcher, nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	status := opsRes.OpsRequest.Status
	if len(status.Conditions) == 0 {
		return status.Phase, 30 * time.Second, nil
	}
	if isNoChange(status.Conditions) {
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	}

	ops := &opsRes.OpsRequest.Spec
	if ops.Reconfigure == nil || len(ops.Reconfigure.Configurations) == 0 {
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}

	resource, err := r.syncDependResources(reqCtx, cli, opsRes)
	if err != nil {
		return "", 30 * time.Second, err
	}
	configSpec := ops.Reconfigure.Configurations[0]
	item := resource.ConfigurationObj.Spec.GetConfigurationItem(configSpec.Name)
	itemStatus := resource.ConfigurationObj.Status.GetItemStatus(configSpec.Name)
	if item == nil || itemStatus == nil {
		return appsv1alpha1.OpsRunningPhase, 30 * time.Second, nil
	}

	switch phase := reconfiguringPhase(resource, *item, itemStatus); phase {
	case appsv1alpha1.CCreatingPhase, appsv1alpha1.CInitPhase:
		return appsv1alpha1.OpsFailedPhase, 0, core.MakeError("the configuration is creating or initializing, is not ready to reconfigure")
	case appsv1alpha1.CFailedAndPausePhase:
		return syncStatus(cli, reqCtx, opsRes, itemStatus, phase, appsv1alpha1.OpsFailedPhase, appsv1alpha1.ReasonReconfigureFailed)
	case appsv1alpha1.CFinishedPhase:
		return syncStatus(cli, reqCtx, opsRes, itemStatus, phase, appsv1alpha1.OpsSucceedPhase, appsv1alpha1.ReasonReconfigureSucceed)
	default:
		return syncStatus(cli, reqCtx, opsRes, itemStatus, phase, appsv1alpha1.OpsRunningPhase, appsv1alpha1.ReasonReconfigureRunning)
	}
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) error {
	var (
		opsRequest    = resource.OpsRequest
		spec          = &opsRequest.Spec
		clusterName   = spec.ClusterRef
		componentName = spec.Reconfigure.ComponentName
		reconfigure   = spec.Reconfigure
	)

	if !needReconfigure(opsRequest) {
		return nil
	}

	// TODO support multi tpl conditions merge
	item := reconfigure.Configurations[0]
	opsPipeline := newPipeline(reconfigureContext{
		cli:           cli,
		reqCtx:        reqCtx,
		resource:      resource,
		config:        item,
		clusterName:   clusterName,
		componentName: componentName,
	})

	result := opsPipeline.
		Configuration().
		Validate().
		ConfigMap(item.Name).
		ConfigConstraints().
		Merge().
		UpdateOpsLabel().
		Sync().
		Complete()

	if result.err != nil {
		return processMergedFailed(resource, result.failed, result.err)
	}

	reqCtx.Recorder.Eventf(resource.OpsRequest,
		corev1.EventTypeNormal,
		appsv1alpha1.ReasonReconfigurePersisted,
		"the reconfiguring operation of component[%s] in cluster[%s] merged successfully", componentName, clusterName)

	// merged successfully
	if err := patchReconfigureOpsStatus(resource, opsPipeline.configSpec.Name,
		handleNewReconfigureRequest(result.configPatch, result.lastAppliedConfigs)); err != nil {
		return err
	}
	condition := constructReconfiguringConditions(result, resource, opsPipeline.configSpec)
	resource.OpsRequest.SetStatusCondition(*condition)
	return nil
}

func needReconfigure(request *appsv1alpha1.OpsRequest) bool {
	// Update params to configmap
	if request.Spec.Type != appsv1alpha1.ReconfiguringType ||
		request.Spec.Reconfigure == nil ||
		len(request.Spec.Reconfigure.Configurations) == 0 {
		return false
	}

	// Check if the reconfiguring operation has been processed.
	for _, condition := range request.Status.Conditions {
		if isExpectedPhase(condition, []string{appsv1alpha1.ReasonReconfigurePersisted, appsv1alpha1.ReasonReconfigureNoChanged}, metav1.ConditionTrue) {
			return false
		}
	}
	return true
}

func syncStatus(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	opsRes *OpsResource,
	status *appsv1alpha1.ConfigurationItemDetailStatus,
	phase appsv1alpha1.ConfigurationPhase,
	opsPhase appsv1alpha1.OpsPhase,
	reconfigurePhase string) (appsv1alpha1.OpsPhase, time.Duration, error) {
	opsDeepCopy := opsRes.OpsRequest.DeepCopy()
	if err := patchReconfigureOpsStatus(opsRes, status.Name,
		handleReconfigureStatusProgress(status.ReconcileDetail, &opsRes.OpsRequest.Status, phase)); err != nil {
		return "", 30 * time.Second, err
	}
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, opsRes, opsDeepCopy, appsv1alpha1.OpsRunningPhase,
		appsv1alpha1.NewReconfigureRunningCondition(opsRes.OpsRequest, reconfigurePhase, status.Name)); err != nil {
		return "", 30 * time.Second, err
	}
	return opsPhase, 30 * time.Second, nil
}

func reconfiguringPhase(resource *intctrlutil.Fetcher,
	detail appsv1alpha1.ConfigurationItemDetail,
	status *appsv1alpha1.ConfigurationItemDetailStatus) appsv1alpha1.ConfigurationPhase {
	if status.ReconcileDetail == nil || status.ReconcileDetail.CurrentRevision != status.UpdateRevision {
		return appsv1alpha1.CPendingPhase
	}
	return intctrlutil.GetConfigSpecReconcilePhase(resource.ConfigMapObj, detail, status)
}

func isExpectedPhase(condition metav1.Condition, expectedTypes []string, expectedStatus metav1.ConditionStatus) bool {
	for _, t := range expectedTypes {
		if t == condition.Type && condition.Status == expectedStatus {
			return true
		}
	}
	return false
}

func isNoChange(conditions []metav1.Condition) bool {
	for i := len(conditions); i > 0; i-- {
		if isExpectedPhase(conditions[i-1], []string{appsv1alpha1.ReasonReconfigureNoChanged}, metav1.ConditionTrue) {
			return true
		}
	}
	return false
}
