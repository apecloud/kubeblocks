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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
	intctrlutil.ConfigEventHandlerMap["ops_status_reconfigure"] = &reAction
	opsManager.RegisterOps(appsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r *reconfigureAction) Handle(eventContext intctrlutil.ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.OpsPhase, cfgError error) error {
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

func handleReconfigureStatusProgress(execStatus cfgcore.PolicyExecStatus, phase appsv1alpha1.OpsPhase, opsStatus *appsv1alpha1.OpsRequestStatus) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationStatus) error {
		cmStatus.LastAppliedStatus = execStatus.ExecStatus
		cmStatus.UpdatePolicy = appsv1alpha1.UpgradePolicy(execStatus.PolicyName)
		cmStatus.SucceedCount = execStatus.SucceedCount
		cmStatus.ExpectedCount = execStatus.ExpectedCount
		if cmStatus.SucceedCount != cfgcore.Unconfirmed && cmStatus.ExpectedCount != cfgcore.Unconfirmed {
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
		return nil
	}
}

func handleNewReconfigureRequest(configPatch *cfgcore.ConfigPatchInfo, lastAppliedConfigs map[string]string) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationStatus) error {
		cmStatus.Status = appsv1alpha1.ReasonReconfigureMerged
		cmStatus.LastAppliedConfiguration = lastAppliedConfigs
		cmStatus.UpdatedParameters = appsv1alpha1.UpdatedParameters{
			AddedKeys:   i2sMap(configPatch.AddConfig),
			UpdatedKeys: b2sMap(configPatch.UpdateConfig),
			DeletedKeys: i2sMap(configPatch.DeleteConfig),
		}
		return nil
	}
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
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

func (r *reconfigureAction) syncReconfigureOperatorStatus(ctx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, error) {
	var (
		ops        = &opsRes.OpsRequest.Spec
		ns         = opsRes.Cluster.Namespace
		name       = opsRes.OpsRequest.Name
		configSpec = ops.Reconfigure.Configurations[0]
	)

	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(ops.ClusterRef, ops.Reconfigure.ComponentName, configSpec.Name),
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

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) error {
	var (
		opsRequest        = resource.OpsRequest
		spec              = &opsRequest.Spec
		clusterName       = spec.ClusterRef
		componentName     = spec.Reconfigure.ComponentName
		cluster           = resource.Cluster
		clusterDefinition = &appsv1alpha1.ClusterDefinition{}
		clusterVersion    = &appsv1alpha1.ClusterVersion{}
	)

	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterDefRef,
		Namespace: cluster.Namespace,
	}, clusterDefinition); err != nil {
		return cfgcore.WrapError(err, "failed to get clusterdefinition[%s]", cluster.Spec.ClusterDefRef)
	}

	if err := getClusterVersionResource(cluster.Spec.ClusterVersionRef, clusterVersion, cli, reqCtx.Ctx); err != nil {
		return err
	}

	configSpecs, err := cfgcore.GetConfigTemplatesFromComponent(
		cluster.Spec.ComponentSpecs,
		clusterDefinition.Spec.ComponentDefs,
		clusterVersion.Spec.ComponentVersions,
		componentName)
	if err != nil {
		return processMergedFailed(resource, true,
			cfgcore.WrapError(err, "failed to get config template in the component[%s]", componentName))
	}
	return r.sync(reqCtx, cli, clusterName, componentName, spec.Reconfigure, resource, configSpecs)
}

func (r *reconfigureAction) sync(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	clusterName, componentName string,
	reconfigure *appsv1alpha1.Reconfigure,
	resource *OpsResource,
	configSpecs []appsv1alpha1.ComponentConfigSpec) error {
	foundConfigSpec := func(configSpecName string) *appsv1alpha1.ComponentConfigSpec {
		if len(configSpecName) == 0 && len(configSpecs) == 1 {
			return &configSpecs[0]
		}
		for _, configSpec := range configSpecs {
			if configSpec.Name == configSpecName {
				return &configSpec
			}
		}
		return nil
	}

	// Update params to configmap
	// TODO support multi tpl conditions merge
	for _, config := range reconfigure.Configurations {
		configSpec := foundConfigSpec(config.Name)
		if configSpec == nil {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("failed to reconfigure, not existed config[%s], all configs: %v", config.Name, getConfigSpecName(configSpecs)))
		}
		if len(configSpec.ConfigConstraintRef) == 0 {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("current configSpec not support reconfigure, configSpec: %v", configSpec.Name))
		}
		result := updateConfigConfigmapResource(config, *configSpec, client.ObjectKey{
			Name:      cfgcore.GetComponentCfgName(clusterName, componentName, configSpec.Name),
			Namespace: resource.Cluster.Namespace,
		}, reqCtx.Ctx, cli, resource.OpsRequest.Name)
		if result.err != nil {
			return processMergedFailed(resource, result.failed, result.err)
		} else {
			reqCtx.Recorder.Eventf(resource.OpsRequest,
				corev1.EventTypeNormal,
				appsv1alpha1.ReasonReconfigureMerged,
				"the reconfiguring operation of component[%s] in cluster[%s] merged successfully", componentName, clusterName)
		}

		// merged successfully
		if err := patchReconfigureOpsStatus(resource, configSpec.Name,
			handleNewReconfigureRequest(result.configPatch, result.lastAppliedConfigs)); err != nil {
			return err
		}
		condition := constructReconfiguringConditions(result, resource, configSpec)
		resource.OpsRequest.SetStatusCondition(*condition)
	}
	return nil
}
