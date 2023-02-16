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
)

type reconfigureAction struct {
}

func init() {
	var (
		reAction   = reconfigureAction{}
		opsManager = GetOpsManager()
	)

	reconfigureBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.Phase{
			appsv1alpha1.RunningPhase,
			appsv1alpha1.FailedPhase,
			appsv1alpha1.AbnormalPhase,
		},
		ToClusterPhase: appsv1alpha1.ReconfiguringPhase,
		OpsHandler:     &reAction,
	}
	cfgcore.ConfigEventHandlerMap["ops_status_reconfigure"] = &reAction
	opsManager.RegisterOps(appsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

func (r *reconfigureAction) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewReconfigureCondition(opsRequest)
}

func (r *reconfigureAction) SaveLastConfiguration(_ *OpsResource) error {
	return nil
}

func (r *reconfigureAction) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return make(map[string]struct{})
}

func (r *reconfigureAction) Handle(eventContext cfgcore.ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.Phase, err error) error {
	var (
		opsRequest = &appsv1alpha1.OpsRequest{}
		cm         = eventContext.CfgCM
		cli        = eventContext.Client
		ctx        = eventContext.ReqCtx.Ctx
	)

	opsRes := &OpsResource{
		Ctx:        ctx,
		OpsRequest: opsRequest,
		Recorder:   eventContext.ReqCtx.Recorder,
		Client:     cli,
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

	if err := patchReconfigureOpsStatus(opsRes, eventContext.TplName,
		handleReconfigureStatusProgress(eventContext.PolicyStatus, phase, &opsRequest.Status)); err != nil {
		return err
	}

	switch phase {
	case appsv1alpha1.SucceedPhase:
		return PatchOpsStatus(opsRes, appsv1alpha1.RunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureSucceed,
				eventContext.TplName,
				formatConfigPatchToMessage(eventContext.ConfigPatch, &eventContext.PolicyStatus)),
			appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.FailedPhase:
		return PatchOpsStatus(opsRes, appsv1alpha1.RunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureFailed,
				eventContext.TplName,
				formatConfigPatchToMessage(eventContext.ConfigPatch, &eventContext.PolicyStatus)),
			appsv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return PatchOpsStatus(opsRes, appsv1alpha1.RunningPhase,
			appsv1alpha1.NewReconfigureRunningCondition(opsRequest,
				appsv1alpha1.ReasonReconfigureRunning,
				eventContext.TplName))
	}
}

func handleReconfigureStatusProgress(execStatus cfgcore.PolicyExecStatus, phase appsv1alpha1.Phase, opsStatus *appsv1alpha1.OpsRequestStatus) handleReconfigureOpsStatus {
	return func(cmStatus *appsv1alpha1.ConfigurationStatus) error {
		cmStatus.LastAppliedStatus = execStatus.ExecStatus
		cmStatus.UpdatePolicy = appsv1alpha1.UpgradePolicy(execStatus.PolicyName)
		cmStatus.SucceedCount = execStatus.SucceedCount
		cmStatus.ExpectedCount = execStatus.ExpectedCount
		if cmStatus.SucceedCount != cfgcore.Unconfirmed && cmStatus.ExpectedCount != cfgcore.Unconfirmed {
			opsStatus.Progress = getSlowestReconfiguringProgress(opsStatus.ReconfiguringStatus.ConfigurationStatus)
		}
		switch phase {
		case appsv1alpha1.SucceedPhase:
			cmStatus.Status = appsv1alpha1.ReasonReconfigureSucceed
		case appsv1alpha1.FailedPhase:
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

func (r reconfigureAction) ReconcileAction(opsRes *OpsResource) (appsv1alpha1.Phase, time.Duration, error) {
	status := opsRes.OpsRequest.Status
	if len(status.Conditions) == 0 {
		return status.Phase, 30 * time.Second, nil
	}
	condition := status.Conditions[len(status.Conditions)-1]
	if condition.Type == appsv1alpha1.ConditionTypeSucceed && condition.Status == metav1.ConditionTrue {
		return appsv1alpha1.SucceedPhase, 0, nil
	}
	if condition.Type == appsv1alpha1.ConditionTypeFailed && condition.Status == metav1.ConditionFalse {
		return appsv1alpha1.FailedPhase, 0, nil
	}
	return appsv1alpha1.RunningPhase, 30 * time.Second, nil
}

func (r *reconfigureAction) Action(resource *OpsResource) error {
	var (
		opsRequest        = resource.OpsRequest
		spec              = &opsRequest.Spec
		clusterName       = spec.ClusterRef
		componentName     = spec.Reconfigure.ComponentName
		cluster           = resource.Cluster
		clusterDefinition = &appsv1alpha1.ClusterDefinition{}
		clusterVersion    = &appsv1alpha1.ClusterVersion{}
	)

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterDefRef,
		Namespace: cluster.Namespace,
	}, clusterDefinition); err != nil {
		return cfgcore.WrapError(err, "failed to get clusterdefinition[%s]", cluster.Spec.ClusterDefRef)
	}

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterVersionRef,
		Namespace: cluster.Namespace,
	}, clusterVersion); err != nil {
		return cfgcore.WrapError(err, "failed to get clusterversion[%s]", cluster.Spec.ClusterVersionRef)
	}

	if opsRequest.Status.ObservedGeneration == opsRequest.ObjectMeta.Generation {
		return nil
	}

	tpls, err := cfgcore.GetConfigTemplatesFromComponent(
		cluster.Spec.ComponentSpecs,
		clusterDefinition.Spec.ComponentDefs,
		clusterVersion.Spec.ComponentVersions,
		componentName)
	if err != nil {
		return cfgcore.WrapError(err, "failed to get config template[%s]", componentName)
	}
	return r.doMergeAndPersist(clusterName, componentName, spec.Reconfigure, resource, tpls)
}

func (r *reconfigureAction) doMergeAndPersist(clusterName, componentName string,
	reconfigure *appsv1alpha1.Reconfigure,
	resource *OpsResource,
	tpls []appsv1alpha1.ConfigTemplate) error {
	findTpl := func(tplName string) *appsv1alpha1.ConfigTemplate {
		if len(tplName) == 0 && len(tpls) == 1 {
			return &tpls[0]
		}
		for _, tpl := range tpls {
			if tpl.Name == tplName {
				return &tpl
			}
		}
		return nil
	}

	// Update params to configmap
	// TODO support multi tpl conditions merge
	for _, config := range reconfigure.Configurations {
		tpl := findTpl(config.Name)
		if tpl == nil {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("failed to reconfigure, not exist config[%s], all configs: %v", config.Name, tpls))
		}
		if len(tpl.ConfigConstraintRef) == 0 {
			return processMergedFailed(resource, true,
				cfgcore.MakeError("current tpl not support reconfigure, tpl: %v", tpl))
		}
		result := updateCfgParams(config, *tpl, client.ObjectKey{
			Name:      cfgcore.GetComponentCfgName(clusterName, componentName, tpl.VolumeName),
			Namespace: resource.Cluster.Namespace,
		}, resource.Ctx, resource.Client, resource.OpsRequest.Name)
		if result.err != nil {
			return processMergedFailed(resource, result.failed, result.err)
		}

		// merged successfully
		if err := patchReconfigureOpsStatus(resource, tpl.Name,
			handleNewReconfigureRequest(result.configPatch, result.lastAppliedConfigs)); err != nil {
			return err
		}
		conditions := constructReconfiguringConditions(result, resource, tpl)
		if err := PatchOpsStatus(resource, appsv1alpha1.RunningPhase, conditions...); err != nil {
			return err
		}
	}
	return nil
}
