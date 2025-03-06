/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var syncPolicyInstance = &syncPolicy{}

type syncPolicy struct{}

func init() {
	registerPolicy(parametersv1alpha1.SyncDynamicReloadPolicy, syncPolicyInstance)
}

func (o *syncPolicy) GetPolicyName() string {
	return string(parametersv1alpha1.SyncDynamicReloadPolicy)
}

func (o *syncPolicy) Upgrade(rctx reconfigureContext) (ReturnedStatus, error) {
	updatedParameters := rctx.UpdatedParameters
	if len(updatedParameters) == 0 {
		return makeReturnedStatus(ESNone), nil
	}

	funcs := GetInstanceSetRollingUpgradeFuncs()
	pods, err := funcs.GetPodsFunc(rctx)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	return sync(rctx, updatedParameters, pods, funcs)
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

func sync(rctx reconfigureContext, updatedParameters map[string]string, pods []corev1.Pod, funcs RollingUpgradeFuncs) (ReturnedStatus, error) {
	var (
		r        = ESNone
		total    = int32(len(pods))
		replicas = int32(rctx.getTargetReplicas())
		progress = core.NotStarted

		err         error
		ctx         = rctx.Ctx
		configKey   = rctx.generateConfigIdentifier()
		versionHash = rctx.getTargetVersionHash()
		selector    = intctrlutil.GetPodSelector(rctx.ParametersDef)
		fileName    string
	)

	if selector != nil {
		pods, err = matchLabel(pods, selector)
	}
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	if len(pods) == 0 {
		rctx.Log.Info(fmt.Sprintf("no pods to update, and retry, selector: %v", selector))
		return makeReturnedStatus(ESRetry), nil
	}
	if rctx.ConfigDescription != nil {
		fileName = rctx.ConfigDescription.Name
	}

	requireUpdatedCount := int32(len(pods))
	for _, pod := range pods {
		rctx.Log.V(1).Info(fmt.Sprintf("sync pod: %s", pod.Name))
		if intctrlutil.IsMatchConfigVersion(&pod, configKey, versionHash) {
			progress++
			continue
		}
		if !intctrlutil.PodIsReady(&pod) {
			continue
		}
		if err = funcs.OnlineUpdatePodFunc(&pod, ctx, rctx.ReconfigureClientFactory, rctx.ConfigTemplate.Name, fileName, updatedParameters); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		if err = updatePodLabelsWithConfigVersion(&pod, configKey, versionHash, rctx.Client, ctx); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		progress++
	}

	if requireUpdatedCount != progress || replicas != total {
		r = ESRetry
	}
	return makeReturnedStatus(r, withExpected(requireUpdatedCount), withSucceed(progress)), nil
}
