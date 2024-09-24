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

package operations

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type reconfigureAction struct {
}

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1.GetReconfiguringRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase: appsv1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     &reAction,
	}
	opsManager.RegisterOps(opsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func handleReconfigureStatusProgress(result *appsv1alpha1.ReconcileDetail, opsStatus *opsv1alpha1.OpsRequestStatus, phase appsv1alpha1.ConfigurationPhase) handleReconfigureOpsStatus {
	return func(cmStatus *opsv1alpha1.ConfigurationItemStatus) (err error) {
		// the Pending phase is waiting to be executed, and there is currently no valid ReconcileDetail information.
		if result != nil && phase != appsv1alpha1.CPendingPhase {
			cmStatus.LastAppliedStatus = result.ExecResult
			cmStatus.UpdatePolicy = appsv1alpha1.UpgradePolicy(result.Policy)
			cmStatus.SucceedCount = result.SucceedCount
			cmStatus.ExpectedCount = result.ExpectedCount
			cmStatus.Message = result.ErrMessage
			cmStatus.Status = string(phase)
		}
		return
	}
}

func handleNewReconfigureRequest(configPatch *core.ConfigPatchInfo, lastAppliedConfigs map[string]string) handleReconfigureOpsStatus {
	return func(cmStatus *opsv1alpha1.ConfigurationItemStatus) (err error) {
		cmStatus.Status = appsv1alpha1.ReasonReconfigurePersisted
		cmStatus.LastAppliedConfiguration = lastAppliedConfigs
		if configPatch != nil {
			cmStatus.UpdatedParameters = opsv1alpha1.UpdatedParameters{
				AddedKeys:   i2sMap(configPatch.AddConfig),
				UpdatedKeys: b2sMap(configPatch.UpdateConfig),
				DeletedKeys: i2sMap(configPatch.DeleteConfig),
			}
		}
		return
	}
}

func (r *reconfigureAction) syncDependResources(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, configSpec opsv1alpha1.ConfigurationItem, componentName string) (*configctrl.Fetcher, error) {
	fetcher := configctrl.NewResourceFetcher(&configctrl.ResourceCtx{
		Context:       reqCtx.Ctx,
		Client:        cli,
		Namespace:     opsRes.Cluster.Namespace,
		ClusterName:   opsRes.Cluster.Name,
		ComponentName: componentName,
	})

	fetcher.ClusterObj = opsRes.Cluster
	err := fetcher.
		Configuration().
		ConfigMap(configSpec.Name).
		Complete()
	if err != nil {
		return nil, err
	}
	return fetcher, nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		isFinished = true
		opsRequest = resource.OpsRequest.Spec
	)

	// Node: support multiple component
	opsDeepCopy := resource.OpsRequest.DeepCopy()
	statusAsComponents := make([]opsv1alpha1.ConfigurationItemStatus, 0)
	for _, reconfigureParams := range fromReconfigureOperations(opsRequest, reqCtx, cli, resource) {
		phase, err := r.doSyncReconfigureStatus(reconfigureParams)
		switch {
		case err != nil:
			return "", 30 * time.Second, err
		case phase == opsv1alpha1.OpsFailedPhase:
			return opsv1alpha1.OpsFailedPhase, 0, nil
		case phase != opsv1alpha1.OpsSucceedPhase:
			isFinished = false
		}
		statusAsComponents = append(statusAsComponents, reconfigureParams.configurationStatus.ConfigurationStatus[0])
	}

	phase := opsv1alpha1.OpsRunningPhase
	if isFinished {
		phase = opsv1alpha1.OpsSucceedPhase
	}
	return syncReconfigureForOps(reqCtx, cli, resource, statusAsComponents, opsDeepCopy, phase)
}

func fromReconfigureOperations(request opsv1alpha1.OpsRequestSpec, reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (reconfigures []reconfigureParams) {
	var operations []opsv1alpha1.Reconfigure
	operations = append(operations, request.Reconfigures...)

	for _, reconfigure := range operations {
		if len(reconfigure.Configurations) == 0 {
			continue
		}
		reconfigures = append(reconfigures, reconfigureParams{
			resource:            resource,
			reqCtx:              reqCtx,
			cli:                 cli,
			clusterName:         resource.Cluster.Name,
			componentName:       reconfigure.ComponentName,
			opsRequest:          resource.OpsRequest,
			configurationItem:   reconfigure.Configurations[0],
			configurationStatus: initReconfigureStatus(resource.OpsRequest, reconfigure.ComponentName),
		})
	}
	return reconfigures
}

func syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, statusAsComponents []opsv1alpha1.ConfigurationItemStatus, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	succeedCount := 0
	expectedCount := 0
	opsRequest := resource.OpsRequest
	invalidProgress := false
	for _, status := range statusAsComponents {
		if status.SucceedCount < 0 || status.ExpectedCount < 0 {
			invalidProgress = true
			break
		}
		succeedCount += int(status.SucceedCount)
		expectedCount += int(status.ExpectedCount)
	}
	if !invalidProgress {
		opsRequest.Status.Progress = fmt.Sprintf("%d/%d", succeedCount, expectedCount)
	}
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", 30 * time.Second, err
	}
	return phase, 30 * time.Second, nil
}

func (r *reconfigureAction) doSyncReconfigureStatus(params reconfigureParams) (opsv1alpha1.OpsPhase, error) {
	configSpec := params.configurationItem
	resource, err := r.syncDependResources(params.reqCtx,
		params.cli, params.resource, configSpec, params.componentName)
	if err != nil {
		return "", err
	}

	item := resource.ConfigurationObj.Spec.GetConfigurationItem(configSpec.Name)
	itemStatus := resource.ConfigurationObj.Status.GetItemStatus(configSpec.Name)
	if item == nil || itemStatus == nil {
		return opsv1alpha1.OpsRunningPhase, nil
	}

	switch phase := reconfiguringPhase(resource, *item, itemStatus); phase {
	case appsv1alpha1.CCreatingPhase, appsv1alpha1.CInitPhase:
		return opsv1alpha1.OpsFailedPhase, core.MakeError("the configuration is creating or initializing, is not ready to reconfigure")
	case appsv1alpha1.CFailedAndPausePhase:
		return opsv1alpha1.OpsFailedPhase,
			syncStatus(params.configurationStatus, params.resource, itemStatus, phase)
	case appsv1alpha1.CFinishedPhase:
		return opsv1alpha1.OpsSucceedPhase,
			syncStatus(params.configurationStatus, params.resource, itemStatus, phase)
	default:
		return opsv1alpha1.OpsRunningPhase,
			syncStatus(params.configurationStatus, params.resource, itemStatus, phase)
	}
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) error {
	opsRequest := resource.OpsRequest.Spec
	// Node: support multiple component
	for _, reconfigureParams := range fromReconfigureOperations(opsRequest, reqCtx, cli, resource) {
		if err := r.doReconfiguring(reconfigureParams); err != nil {
			return err
		}
	}
	return nil
}

func (r *reconfigureAction) doReconfiguring(params reconfigureParams) error {
	if !needReconfigure(params.opsRequest, params.configurationStatus) {
		return nil
	}

	item := params.configurationItem
	opsPipeline := newPipeline(reconfigureContext{
		cli:           params.cli,
		reqCtx:        params.reqCtx,
		resource:      params.resource,
		config:        item,
		clusterName:   params.clusterName,
		componentName: params.componentName,
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
		return processMergedFailed(params.resource, result.failed, result.err)
	}

	params.reqCtx.Recorder.Eventf(params.resource.OpsRequest,
		corev1.EventTypeNormal,
		appsv1alpha1.ReasonReconfigurePersisted,
		"the reconfiguring operation of component[%s] in cluster[%s] merged successfully", params.componentName, params.clusterName)

	// merged successfully
	if err := updateReconfigureStatusByCM(params.configurationStatus, opsPipeline.configSpec.Name,
		handleNewReconfigureRequest(result.configPatch, result.lastAppliedConfigs)); err != nil {
		return err
	}
	condition := constructReconfiguringConditions(result, params.resource, opsPipeline.configSpec)
	meta.SetStatusCondition(&params.configurationStatus.Conditions, *condition)
	return nil
}

func needReconfigure(request *opsv1alpha1.OpsRequest, status *opsv1alpha1.ReconfiguringStatus) bool {
	// Update params to configmap
	if request.Spec.Type != opsv1alpha1.ReconfiguringType {
		return false
	}

	// Check if the reconfiguring operation has been processed.
	for _, condition := range status.Conditions {
		if isExpectedPhase(condition, []string{appsv1alpha1.ReasonReconfigurePersisted, appsv1alpha1.ReasonReconfigureNoChanged}, metav1.ConditionTrue) {
			return false
		}
	}
	return true
}

func syncStatus(reconfiguringStatus *opsv1alpha1.ReconfiguringStatus,
	opsRes *OpsResource,
	status *appsv1alpha1.ConfigurationItemDetailStatus,
	phase appsv1alpha1.ConfigurationPhase) error {
	err := updateReconfigureStatusByCM(reconfiguringStatus, status.Name,
		handleReconfigureStatusProgress(status.ReconcileDetail, &opsRes.OpsRequest.Status, phase))
	meta.SetStatusCondition(&reconfiguringStatus.Conditions, *opsv1alpha1.NewReconfigureRunningCondition(
		opsRes.OpsRequest, string(phase), status.Name))
	return err
}

func reconfiguringPhase(resource *configctrl.Fetcher,
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

func initReconfigureStatus(opsRequest *opsv1alpha1.OpsRequest, componentName string) *opsv1alpha1.ReconfiguringStatus {
	status := &opsRequest.Status
	if status.ReconfiguringStatusAsComponent == nil {
		status.ReconfiguringStatusAsComponent = make(map[string]*opsv1alpha1.ReconfiguringStatus)
	}
	if _, ok := status.ReconfiguringStatusAsComponent[componentName]; !ok {
		status.ReconfiguringStatusAsComponent[componentName] = &opsv1alpha1.ReconfiguringStatus{
			ConfigurationStatus: make([]opsv1alpha1.ConfigurationItemStatus, 0),
		}
	}
	return status.ReconfiguringStatusAsComponent[componentName]
}
