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

package configuration

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	podutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type syncPolicy struct {
}

func init() {
	RegisterPolicy(appsv1alpha1.OperatorSyncUpdate, &syncPolicy{})
}

func (o *syncPolicy) GetPolicyName() string {
	return string(appsv1alpha1.OperatorSyncUpdate)
}

func (o *syncPolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	configPatch := params.ConfigPatch
	if !configPatch.IsModify {
		return makeReturnedStatus(ESNone), nil
	}

	updatedParameters := getOnlineUpdateParams(configPatch, params.ConfigConstraint.FormatterConfig)
	if len(updatedParameters) == 0 {
		return makeReturnedStatus(ESNone), nil
	}

	var funcs RollingUpgradeFuncs
	switch params.WorkloadType() {
	default:
		return makeReturnedStatus(ESNotSupport), core.MakeError("not support component workload type[%s]", params.WorkloadType())
	case appsv1alpha1.Stateless:
		funcs = GetDeploymentRollingUpgradeFuncs()
	case appsv1alpha1.Consensus:
		funcs = GetConsensusRollingUpgradeFuncs()
	case appsv1alpha1.Stateful:
		funcs = GetStatefulSetRollingUpgradeFuncs()
	case appsv1alpha1.Replication:
		funcs = GetReplicationRollingUpgradeFuncs()
	}

	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	return sync(params, updatedParameters, pods, funcs)
}

func matchLabel(pods []corev1.Pod, selector *metav1.LabelSelector) ([]corev1.Pod, error) {
	var result []corev1.Pod

	match, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, core.WrapError(err, "failed to convert selector: %v", selector)
	}
	for _, pod := range pods {
		if match.Matches(labels.Set(pod.Labels)) {
			result = append(result, pod)
		}
	}
	return result, nil
}

func sync(params reconfigureParams, updatedParameters map[string]string, pods []corev1.Pod, funcs RollingUpgradeFuncs) (ReturnedStatus, error) {
	var (
		r        = ESNone
		total    = int32(len(pods))
		replicas = int32(params.getTargetReplicas())
		progress = core.NotStarted

		err         error
		ctx         = params.Ctx.Ctx
		configKey   = params.getConfigKey()
		versionHash = params.getTargetVersionHash()
	)

	if params.ConfigConstraint.Selector != nil {
		pods, err = matchLabel(pods, params.ConfigConstraint.Selector)
	}
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	if len(pods) == 0 {
		params.Ctx.Log.Info(fmt.Sprintf("no pods to update, and retry, selector: %s", params.ConfigConstraint.Selector.String()))
		return makeReturnedStatus(ESRetry), nil
	}

	requireUpdatedCount := int32(len(pods))
	for _, pod := range pods {
		params.Ctx.Log.V(1).Info(fmt.Sprintf("sync pod: %s", pod.Name))
		if podutil.IsMatchConfigVersion(&pod, configKey, versionHash) {
			progress++
			continue
		}
		if !podutil.PodIsReady(&pod) {
			continue
		}
		err = funcs.OnlineUpdatePodFunc(&pod, ctx, params.ReconfigureClientFactory, params.ConfigSpecName, updatedParameters)
		if err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		err = updatePodLabelsWithConfigVersion(&pod, configKey, versionHash, params.Client, ctx)
		if err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		progress++
	}

	if requireUpdatedCount != progress || replicas != total {
		r = ESRetry
	}
	return makeReturnedStatus(r, withExpected(requireUpdatedCount), withSucceed(progress)), nil
}

func getOnlineUpdateParams(configPatch *core.ConfigPatchInfo, formatConfig *appsv1alpha1.FormatterConfig) map[string]string {
	r := make(map[string]string)
	parameters := core.GenerateVisualizedParamsList(configPatch, formatConfig, nil)
	for _, key := range parameters {
		if key.UpdateType == core.UpdatedType {
			for _, p := range key.Parameters {
				if p.Value != nil {
					r[p.Key] = *p.Value
				}
			}
		}
	}
	return r
}
