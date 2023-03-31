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

package operations

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type reconfigureAction struct {
}

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		MaintainClusterPhaseBySelf:         true,
		OpsHandler:                         &reAction,
		ProcessingReasonInClusterCondition: ProcessingReasonReconfiguring,
	}
	cfgcore.ConfigEventHandlerMap["ops_status_reconfigure"] = &reAction
	opsManager.RegisterOps(appsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewReconfigureCondition(opsRequest)
}

// SaveLastConfiguration this operation can not change in Cluster.spec.
func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (r *reconfigureAction) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return opsRequest.GetReconfiguringComponentNameMap()
}

func (r *reconfigureAction) Handle(eventContext cfgcore.ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.OpsPhase, cfgError error) error {
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
		return PatchOpsStatusWithOpsDeepCopy(ctx, cli, opsRes, opsDeepCopy, appsv1alpha1.OpsRunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureSucceed,
				eventContext.ConfigSpecName,
				formatConfigPatchToMessage(eventContext.ConfigPatch, &eventContext.PolicyStatus)),
			appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.OpsFailedPhase:
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
		if err := r.syncReconfigureComponentStatus(reqCtx, cli, opsRes, isNoChanged); err != nil {
			return "", time.Second, err
		}
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	}
	if isFailedPhase(condition) {
		// TODO Sync reload progress from config manager.
		if err := r.syncReconfigureComponentStatus(reqCtx, cli, opsRes, true); err != nil {
			return "", time.Second, err
		}
		return appsv1alpha1.OpsFailedPhase, 0, nil
	}
	return appsv1alpha1.OpsRunningPhase, 30 * time.Second, nil
}

func (r *reconfigureAction) syncReconfigureComponentStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	res *OpsResource,
	skipCheckReload bool) error {
	cluster := res.Cluster
	opsRequest := res.OpsRequest

	if opsRequest.Spec.Reconfigure == nil {
		return nil
	}

	if !isReloadPolicy(opsRequest.Status.ReconfiguringStatus) && !skipCheckReload {
		return nil
	}

	componentName := opsRequest.Spec.Reconfigure.ComponentName
	c, ok := cluster.Status.Components[componentName]
	if !ok || c.Phase != appsv1alpha1.SpecReconcilingClusterCompPhase {
		return nil
	}

	clusterPatch := client.MergeFrom(cluster.DeepCopy())
	c.Phase = appsv1alpha1.RunningClusterCompPhase
	cluster.Status.SetComponentStatus(componentName, c)
	return cli.Status().Patch(reqCtx.Ctx, cluster, clusterPatch)
}

func isReloadPolicy(status *appsv1alpha1.ReconfiguringStatus) bool {
	if status == nil {
		return false
	}
	for _, cmStatus := range status.ConfigurationStatus {
		if cmStatus.UpdatePolicy == appsv1alpha1.AutoReload {
			return true
		}
	}
	return false
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

	if err := cfgcore.GetClusterVersionResource(cluster.Spec.ClusterVersionRef, clusterVersion, cli, reqCtx.Ctx); err != nil {
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
	return r.doMergeAndPersist(reqCtx, cli, clusterName, componentName, spec.Reconfigure, resource, configSpecs)
}

func (r *reconfigureAction) doMergeAndPersist(reqCtx intctrlutil.RequestCtx,
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
				cfgcore.MakeError("failed to reconfigure, not exist config[%s], all configs: %v", config.Name, getConfigSpecName(configSpecs)))
		}
		if len(configSpec.ConfigConstraintRef) == 0 {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("current configSpec not support reconfigure, configSpec: %v", configSpec.Name))
		}
		result := updateCfgParams(config, *configSpec, client.ObjectKey{
			Name:      cfgcore.GetComponentCfgName(clusterName, componentName, configSpec.VolumeName),
			Namespace: resource.Cluster.Namespace,
		}, reqCtx.Ctx, cli, resource.OpsRequest.Name)
		if result.err != nil {
			return processMergedFailed(resource, result.failed, result.err)
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
