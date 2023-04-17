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

package configuration

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	podutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
		return makeReturnedStatus(ESNotSupport), cfgutil.MakeError("not support component workload type[%s]", params.WorkloadType())
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
		return makeReturnedStatus(ESAndRetryFailed), err
	}
	return sync(params, updatedParameters, pods, funcs)
}

func matchLabel(pods []corev1.Pod, selector *metav1.LabelSelector) ([]corev1.Pod, error) {
	var result []corev1.Pod

	match, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, cfgutil.WrapError(err, "failed to convert selector: %v", selector)
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
		progress = cfgutil.NotStarted

		err         error
		ctx         = params.Ctx.Ctx
		configKey   = params.getConfigKey()
		versionHash = params.getTargetVersionHash()
	)

	if params.ConfigConstraint.Selector != nil {
		pods, err = matchLabel(pods, params.ConfigConstraint.Selector)
	}
	if err != nil {
		return makeReturnedStatus(ESAndRetryFailed), err
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
			return makeReturnedStatus(ESAndRetryFailed), err
		}
		err = updatePodLabelsWithConfigVersion(&pod, configKey, versionHash, params.Client, ctx)
		if err != nil {
			return makeReturnedStatus(ESAndRetryFailed), err
		}
		progress++
	}

	if requireUpdatedCount != progress || replicas != total {
		r = ESRetry
	}
	return makeReturnedStatus(r, withExpected(requireUpdatedCount), withSucceed(progress)), nil
}

func getOnlineUpdateParams(configPatch *cfgutil.ConfigPatchInfo, formatConfig *appsv1alpha1.FormatterConfig) map[string]string {
	r := make(map[string]string)
	parameters := cfgutil.GenerateVisualizedParamsList(configPatch, formatConfig, nil)
	for _, key := range parameters {
		if key.UpdateType == cfgutil.UpdatedType {
			for _, p := range key.Parameters {
				r[p.Key] = p.Value
			}
		}
	}
	return r
}
