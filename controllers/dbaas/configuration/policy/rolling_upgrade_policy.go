/*
Copyright ApeCloud Inc.

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

package policy

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	podutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	defaultMinReadySeconds = 10 // 10s
)

type RollingUpgradePolicy struct {
}

func init() {
	RegisterPolicy(dbaasv1alpha1.RollingPolicy, &RollingUpgradePolicy{})
	if err := viper.BindEnv(cfgcore.PodMinReadySecondsEnv); err != nil {
		logrus.Errorf("failed to set bind env: %s", cfgcore.PodMinReadySecondsEnv)
		os.Exit(-1)
	}
	viper.SetDefault(cfgcore.PodMinReadySecondsEnv, defaultMinReadySeconds)
}

func (r *RollingUpgradePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	var (
		funcs RollingUpgradeFuncs
		cType = params.ComponentType()
	)

	switch cType {
	case dbaasv1alpha1.Consensus:
		funcs = GetConsensusRollingUpgradeFuncs()
	case dbaasv1alpha1.Stateful:
		funcs = GetStatefulSetRollingUpgradeFuncs()
	default:
		return ESNotSupport, cfgcore.MakeError("not support component type[%s]", cType)
	}
	return performRollingUpgrade(params, funcs)
}

func (r *RollingUpgradePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.RollingPolicy)
}

func performRollingUpgrade(params ReconfigureParams, funcs RollingUpgradeFuncs) (ExecStatus, error) {
	pods, err := funcs.GetPodsFunc(params)
	if err != nil {
		return ESAndRetryFailed, err
	}

	var (
		rollingReplicas = params.MaxRollingReplicas()
		target          = params.GetTargetReplicas()
		configKey       = params.GetConfigKey()
		configVersion   = params.GetModifyVersion()
	)

	updatePodLabelsVersion := func(pod *corev1.Pod, labelKey, labelValue string) error {
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Labels == nil {
			pod.Labels = make(map[string]string, 1)
		}
		pod.Labels[labelKey] = labelValue
		return params.Client.Patch(params.Ctx.Ctx, pod, patch)
	}

	if len(pods) < target {
		params.Ctx.Log.Info("component pod not all ready.")
		return ESRetry, nil
	}

	podStats := staticPodStats(pods, params.GetTargetReplicas())
	podWins := markDynamicCursor(pods, podStats, configKey, configVersion, rollingReplicas)
	if !validPodState(podWins) {
		params.Ctx.Log.Info("wait pod stat ready.")
		return ESRetry, nil
	}

	waitRollingPods := podWins.getWaitRollingPods()
	if len(waitRollingPods) == 0 {
		return ESNone, nil
	}

	for _, pod := range waitRollingPods {
		if podStats.isUpdating(&pod) {
			params.Ctx.Log.Info("pod is rolling updating.", "pod name", pod.Name)
			continue
		}
		if err := funcs.RestartContainerFunc(&pod, params.ContainerNames, params.ReconfigureClientFactory); err != nil {
			return ESAndRetryFailed, err
		}
		if err := updatePodLabelsVersion(&pod, configKey, configVersion); err != nil {
			return ESAndRetryFailed, err
		}
	}
	return ESRetry, nil
}

func validPodState(wind switchWindow) bool {
	for i := 0; i < wind.begin; i++ {
		pod := &wind.pods[i]
		if !wind.isReady(pod) {
			return false
		}
	}
	return true
}

func markDynamicCursor(pods []corev1.Pod, podsStats *componentPodStats, configKey, currentVersion string, rollingReplicas int32) switchWindow {
	podWindows := switchWindow{
		end:               0,
		begin:             len(pods),
		pods:              pods,
		componentPodStats: podsStats,
	}

	// find update last
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

	podWindows.begin = cfgcore.Max[int](podWindows.end-int(rollingReplicas), 0)
	for i := podWindows.begin; i < podWindows.end; i++ {
		pod := &pods[i]
		if podutil.IsMatchConfigVersion(pod, configKey, currentVersion) {
			podsStats.updating[pod.Name] = pod
		}
	}
	return podWindows
}

func staticPodStats(pods []corev1.Pod, targetReplicas int) *componentPodStats {
	podsStats := &componentPodStats{
		updated:       make(map[string]*corev1.Pod),
		updating:      make(map[string]*corev1.Pod),
		available:     make(map[string]*corev1.Pod),
		ready:         make(map[string]*corev1.Pod),
		targetReplica: targetReplicas,
	}

	minReadySeconds := viper.GetInt32(cfgcore.PodMinReadySecondsEnv)
	for i := 0; i < len(pods); i++ {
		pod := &pods[i]
		switch {
		case podutil.IsAvailable(pod, minReadySeconds):
			podsStats.available[pod.Name] = pod
		case podutil.IsReady(pod):
			podsStats.ready[pod.Name] = pod
		default:
		}
	}
	return podsStats
}

type componentPodStats struct {
	// failed to start pod
	ready     map[string]*corev1.Pod
	available map[string]*corev1.Pod

	// updated pod count
	updated  map[string]*corev1.Pod
	updating map[string]*corev1.Pod

	// expected pod
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

	pods []corev1.Pod
	*componentPodStats
}

func (w *switchWindow) getWaitRollingPods() []corev1.Pod {
	return w.pods[w.begin:w.end]
}
