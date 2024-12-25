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

package parameters

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	podutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	defaultMinReadySeconds = 10
)

var rollingUpgradePolicyInstance = &rollingUpgradePolicy{}

type rollingUpgradePolicy struct{}

func init() {
	registerPolicy(parametersv1alpha1.RollingPolicy, rollingUpgradePolicyInstance)
	if err := viper.BindEnv(constant.PodMinReadySecondsEnv); err != nil {
		log.Fatalf("failed to bind environment variable: %v", err)
	}
	viper.SetDefault(constant.PodMinReadySecondsEnv, defaultMinReadySeconds)
}

// Upgrade performs a rolling upgrade based on the provided parameters.
func (r *rollingUpgradePolicy) Upgrade(rctx reconfigureContext) (ReturnedStatus, error) {
	return performRollingUpgrade(rctx, GetInstanceSetRollingUpgradeFuncs())
}

func (r *rollingUpgradePolicy) GetPolicyName() string {
	return string(parametersv1alpha1.RollingPolicy)
}

func canPerformUpgrade(pods []corev1.Pod, params reconfigureContext) bool {
	return len(pods) == params.getTargetReplicas()
}

func performRollingUpgrade(rctx reconfigureContext, funcs RollingUpgradeFuncs) (ReturnedStatus, error) {
	pods, err := funcs.GetPodsFunc(rctx)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	var (
		rollingReplicas = rctx.maxRollingReplicas()
		configKey       = rctx.getConfigKey()
		configVersion   = rctx.getTargetVersionHash()
	)

	if !canPerformUpgrade(pods, rctx) {
		return makeReturnedStatus(ESRetry), nil
	}

	podStatus := classifyPodByStats(pods, rctx.getTargetReplicas(), rctx.podMinReadySeconds())
	updateWindow := markDynamicSwitchWindow(pods, podStatus, configKey, configVersion, rollingReplicas)
	if !canSafeUpdatePods(updateWindow) {
		rctx.Log.Info("wait for pod stat ready.")
		return makeReturnedStatus(ESRetry), nil
	}

	podsToUpgrade := updateWindow.getPendingUpgradePods()
	if len(podsToUpgrade) == 0 {
		return makeReturnedStatus(ESNone, withSucceed(int32(podStatus.targetReplica)), withExpected(int32(podStatus.targetReplica))), nil
	}

	for _, pod := range podsToUpgrade {
		if podStatus.isUpdating(&pod) {
			rctx.Log.Info("pod is in rolling update.", "pod name", pod.Name)
			continue
		}
		if err := funcs.RestartContainerFunc(&pod, rctx.Ctx, rctx.ContainerNames, rctx.ReconfigureClientFactory); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
		if err := updatePodLabelsWithConfigVersion(&pod, configKey, configVersion, rctx.Client, rctx.Ctx); err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}
	}

	return makeReturnedStatus(ESRetry,
		withExpected(int32(podStatus.targetReplica)),
		withSucceed(int32(len(podStatus.updated)+len(podStatus.updating)))), nil
}

func canSafeUpdatePods(wind switchWindow) bool {
	for i := 0; i < wind.begin; i++ {
		pod := &wind.pods[i]
		if !wind.isReady(pod) {
			return false
		}
	}
	return true
}

func markDynamicSwitchWindow(pods []corev1.Pod, podsStats *componentPodStats, configKey, currentVersion string, rollingReplicas int32) switchWindow {
	podWindows := switchWindow{
		end:               0,
		begin:             len(pods),
		pods:              pods,
		componentPodStats: podsStats,
	}

	for i := podsStats.targetReplica - 1; i >= 0; i-- {
		pod := &pods[i]
		if !podutil.IsMatchConfigVersion(pod, configKey, currentVersion) {
			podWindows.end = i + 1
			break
		}
		if !podsStats.isAvailable(pod) {
			podsStats.updating[pod.Name] = pod
			podWindows.end = i + 1
			break
		}
		podsStats.updated[pod.Name] = pod
	}

	podWindows.begin = max(podWindows.end-int(rollingReplicas), 0)
	for i := podWindows.begin; i < podWindows.end; i++ {
		pod := &pods[i]
		if podutil.IsMatchConfigVersion(pod, configKey, currentVersion) {
			podsStats.updating[pod.Name] = pod
		}
	}
	return podWindows
}

func classifyPodByStats(pods []corev1.Pod, targetReplicas int, minReadySeconds int32) *componentPodStats {
	podsStats := &componentPodStats{
		updated:       make(map[string]*corev1.Pod),
		updating:      make(map[string]*corev1.Pod),
		available:     make(map[string]*corev1.Pod),
		ready:         make(map[string]*corev1.Pod),
		targetReplica: targetReplicas,
	}

	for i := 0; i < len(pods); i++ {
		pod := &pods[i]
		switch {
		case podutil.IsAvailable(pod, minReadySeconds):
			podsStats.available[pod.Name] = pod
		case podutil.PodIsReady(pod):
			podsStats.ready[pod.Name] = pod
		default:
		}
	}
	return podsStats
}

type componentPodStats struct {
	ready         map[string]*corev1.Pod
	available     map[string]*corev1.Pod
	updated       map[string]*corev1.Pod
	updating      map[string]*corev1.Pod
	targetReplica int
}

func (s *componentPodStats) isAvailable(pod *corev1.Pod) bool {
	_, ok := s.available[pod.Name]
	return ok
}

func (s *componentPodStats) isReady(pod *corev1.Pod) bool {
	_, ok := s.ready[pod.Name]
	return ok || s.isAvailable(pod)
}

func (s *componentPodStats) isUpdating(pod *corev1.Pod) bool {
	_, ok := s.updating[pod.Name]
	return ok
}

type switchWindow struct {
	begin int
	end   int
	pods  []corev1.Pod
	*componentPodStats
}

func (w *switchWindow) getPendingUpgradePods() []corev1.Pod {
	return w.pods[w.begin:w.end]
}

func updatePodLabelsWithConfigVersion(pod *corev1.Pod, labelKey, configVersion string, cli client.Client, ctx context.Context) error {
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Labels == nil {
		pod.Labels = make(map[string]string, 1)
	}
	pod.Labels[labelKey] = configVersion
	return cli.Patch(ctx, pod, patch)
}
