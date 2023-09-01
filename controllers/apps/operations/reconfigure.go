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
	"fmt"
	"time"

	core2 "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		OpsHandler:                         &reAction,
		ProcessingReasonInClusterCondition: ProcessingReasonReconfiguring,
	}
	controllerutil.ConfigEventHandlerMap["ops_status_reconfigure"] = &reAction
	opsManager.RegisterOps(appsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx controllerutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx controllerutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r *reconfigureAction) Handle(eventContext controllerutil.ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.OpsPhase, cfgError error) error {
	var (
		opsRequest = &appsv1alpha1.OpsRequest{}
		cm         = eventContext.ConfigMap
		cli        = eventContext.Client
		ctx        = eventContext.ReqCtx.Ctx
	)

	opsRes := &OpsResource{
		OpsRequest: opsRequest,
		Recorder:   eventContext.ReqCtx.Recorder,
		Cluster:    eventContext.Cluster,
	}

	if len(lastOpsRequest) == 0 {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Name:      lastOpsRequest,
		Namespace: cm.Namespace,
	}, opsRequest); err != nil {
		return err
	}

	opsDeepCopy := opsRequest.DeepCopy()
	if err := patchReconfigureOpsStatus(opsRes, eventContext.ConfigSpecName,
		handleReconfigureStatusProgress(eventContext.PolicyStatus, phase, &opsRequest.Status)); err != nil {
		return err
	}

	switch phase {
	case appsv1alpha1.OpsSucceedPhase:
		// only update the condition of the opsRequest.
		eventContext.ReqCtx.Recorder.Eventf(opsRequest,
			corev1.EventTypeNormal,
			appsv1alpha1.ReasonReconfigureSucceed,
			"the reconfigure has been processed successfully")
		return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsDeepCopy, appsv1alpha1.OpsRunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureSucceed,
				eventContext.ConfigSpecName,
				formatConfigPatchToMessage(eventContext.ConfigPatch, &eventContext.PolicyStatus)),
			appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.OpsFailedPhase:
		eventContext.ReqCtx.Recorder.Eventf(opsRequest,
			corev1.EventTypeWarning,
			appsv1alpha1.ReasonReconfigureFailed,
			"failed to process the reconfigure, error: %v", cfgError)
		return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsDeepCopy, appsv1alpha1.OpsRunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureFailed,
				eventContext.ConfigSpecName,
				formatConfigPatchToMessage(eventContext.ConfigPatch, &eventContext.PolicyStatus)),
			appsv1alpha1.NewReconfigureFailedCondition(opsRequest, cfgError))
	default:
		return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsDeepCopy, appsv1alpha1.OpsRunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureRunning,
				eventContext.ConfigSpecName))
	}
}

func handleReconfigureStatusProgress(execStatus core2.PolicyExecStatus, phase appsv1alpha1.OpsPhase, opsStatus *appsv1alpha1.OpsRequestStatus) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationStatus) (err error) {
		cmStatus.LastAppliedStatus = execStatus.ExecStatus
		cmStatus.UpdatePolicy = appsv1alpha1.UpgradePolicy(execStatus.PolicyName)
		cmStatus.SucceedCount = execStatus.SucceedCount
		cmStatus.ExpectedCount = execStatus.ExpectedCount
		if cmStatus.SucceedCount != core2.Unconfirmed && cmStatus.ExpectedCount != core2.Unconfirmed {
			opsStatus.Progress = getSlowestReconfiguringProgress(opsStatus.ReconfiguringStatus.ConfigurationStatus)
		}
		switch phase {
		case appsv1alpha1.OpsSucceedPhase:
			cmStatus.Status = appsv1alpha1.ReasonReconfigureSucceed
		case appsv1alpha1.OpsFailedPhase:
			cmStatus.Status = appsv1alpha1.ReasonReconfigureFailed
		default:
			cmStatus.Status = appsv1alpha1.ReasonReconfigureRunning
		}
		return
	}
}

func handleNewReconfigureRequest(configPatch *core2.ConfigPatchInfo, lastAppliedConfigs map[string]string) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationStatus) (err error) {
		cmStatus.Status = appsv1alpha1.ReasonReconfigureMerged
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

func (r *reconfigureAction) ReconcileAction(reqCtx controllerutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	status := opsRes.OpsRequest.Status
	if len(status.Conditions) == 0 {
		return status.Phase, 30 * time.Second, nil
	}
	condition := status.Conditions[len(status.Conditions)-1]
	isNoChanged := isNoChange(condition)
	if isSucceedPhase(condition) || isNoChanged {
		// TODO Sync reload progress from config manager.
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	}
	if isFailedPhase(condition) {
		// TODO Sync reload progress from config manager.
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	if !isRunningPhase(condition) {
		return appsv1alpha1.OpsRunningPhase, 30 * time.Second, nil
	}

	ops := &opsRes.OpsRequest.Spec
	if ops.Reconfigure == nil || len(ops.Reconfigure.Configurations) == 0 {
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	phase, err := r.syncReconfigureOperatorStatus(reqCtx, cli, opsRes)
	switch {
	default:
		return appsv1alpha1.OpsRunningPhase, 30 * time.Second, nil
	case err != nil:
		return "", 30 * time.Second, err
	case phase == appsv1alpha1.OpsSucceedPhase:
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	}
}

func (r *reconfigureAction) syncReconfigureOperatorStatus(ctx controllerutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, error) {
	var (
		ops        = &opsRes.OpsRequest.Spec
		ns         = opsRes.Cluster.Namespace
		name       = opsRes.OpsRequest.Name
		configSpec = ops.Reconfigure.Configurations[0]
	)

	cmKey := client.ObjectKey{
		Name:      core2.GetComponentCfgName(ops.ClusterRef, ops.Reconfigure.ComponentName, configSpec.Name),
		Namespace: ns,
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(ctx.Ctx, cmKey, cm); err != nil {
		return appsv1alpha1.OpsRunningPhase, err
	}

	if checkFinishedReconfigure(cm, name, ctx.Log) {
		ctx.Recorder.Eventf(opsRes.OpsRequest,
			corev1.EventTypeNormal,
			appsv1alpha1.ReasonReconfigureSucceed,
			"sync reconfiguring operation phase succeed")
		return appsv1alpha1.OpsSucceedPhase, nil
	}
	return appsv1alpha1.OpsRunningPhase, nil
}

func checkFinishedReconfigure(cm *corev1.ConfigMap, opsRequestName string, logger logr.Logger) bool {
	labels := cm.GetLabels()
	annotations := cm.GetAnnotations()
	if len(annotations) == 0 || len(labels) == 0 {
		return false
	}

	hash, _ := util.ComputeHash(cm.Data)
	logger.V(1).Info(fmt.Sprintf("current opsrqeust: %s, current version: %s, last applied: %s, finish version: %s",
		opsRequestName, hash,
		annotations[constant.LastAppliedOpsCRAnnotationKey],
		labels[constant.CMInsConfigurationHashLabelKey]))
	return labels[constant.CMInsConfigurationHashLabelKey] == hash
}

func isExpectedPhase(condition metav1.Condition, expectedTypes []string, expectedStatus metav1.ConditionStatus) bool {
	for _, t := range expectedTypes {
		if t == condition.Type && condition.Status == expectedStatus {
			return true
		}
	}
	return false
}

func isSucceedPhase(condition metav1.Condition) bool {
	return isExpectedPhase(condition, []string{appsv1alpha1.ConditionTypeSucceed, appsv1alpha1.ReasonReconfigureSucceed}, metav1.ConditionTrue)
}

func isNoChange(condition metav1.Condition) bool {
	return isExpectedPhase(condition, []string{appsv1alpha1.ReasonReconfigureNoChanged}, metav1.ConditionTrue)
}

func isFailedPhase(condition metav1.Condition) bool {
	return isExpectedPhase(condition, []string{appsv1alpha1.ConditionTypeFailed, appsv1alpha1.ReasonReconfigureFailed}, metav1.ConditionFalse)
}

func isRunningPhase(condition metav1.Condition) bool {
	return isExpectedPhase(condition, []string{appsv1alpha1.ReasonReconfigureRunning, appsv1alpha1.ReasonReconfigureMerged},
		metav1.ConditionTrue)
}

func (r *reconfigureAction) Action(reqCtx controllerutil.RequestCtx, cli client.Client, resource *OpsResource) error {
	var (
		opsRequest    = resource.OpsRequest
		spec          = &opsRequest.Spec
		clusterName   = spec.ClusterRef
		componentName = spec.Reconfigure.ComponentName
		reconfigure   = spec.Reconfigure
	)

	// Update params to configmap
	if len(reconfigure.Configurations) == 0 {
		return nil
	}

	// TODO support multi tpl conditions merge
	pipeline := newPipeline(reconfigureContext{
		cli:           cli,
		reqCtx:        reqCtx,
		resource:      resource,
		config:        reconfigure.Configurations[0],
		clusterName:   clusterName,
		componentName: componentName,
	})

	result := pipeline.
		ClusterDefinition().
		ClusterVersion().
		Validate().
		ConfigMap().
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
		appsv1alpha1.ReasonReconfigureMerged,
		"the reconfiguring operation of component[%s] in cluster[%s] merged successfully", componentName, clusterName)

	// merged successfully
	if err := patchReconfigureOpsStatus(resource, pipeline.configSpec.Name,
		handleNewReconfigureRequest(result.configPatch, result.lastAppliedConfigs)); err != nil {
		return err
	}
	condition := constructReconfiguringConditions(result, resource, pipeline.configSpec)
	resource.OpsRequest.SetStatusCondition(*condition)
	return nil
}
