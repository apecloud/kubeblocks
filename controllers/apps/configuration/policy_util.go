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
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createReconfigureClient func(addr string) (cfgproto.ReconfigureClient, error)

type GetPodsFunc func(params reconfigureParams) ([]corev1.Pod, error)

type RestartContainerFunc func(pod *corev1.Pod, ctx context.Context, containerName []string, createConnFn createReconfigureClient) error
type OnlineUpdatePodFunc func(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, updatedParams map[string]string) error

type RollingUpgradeFuncs struct {
	GetPodsFunc          GetPodsFunc
	RestartContainerFunc RestartContainerFunc
	OnlineUpdatePodFunc  OnlineUpdatePodFunc
}

func GetConsensusRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getConsensusPods,
		RestartContainerFunc: commonStopContainerWithPod,
		OnlineUpdatePodFunc:  commonOnlineUpdateWithPod,
	}
}

func GetStatefulSetRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getStatefulSetPods,
		RestartContainerFunc: commonStopContainerWithPod,
		OnlineUpdatePodFunc:  commonOnlineUpdateWithPod,
	}
}

func GetReplicationRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getReplicationSetPods,
		RestartContainerFunc: commonStopContainerWithPod,
		OnlineUpdatePodFunc:  commonOnlineUpdateWithPod,
	}
}

func GetDeploymentRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getDeploymentRollingPods,
		RestartContainerFunc: commonStopContainerWithPod,
		OnlineUpdatePodFunc:  commonOnlineUpdateWithPod,
	}
}

func getDeploymentRollingPods(params reconfigureParams) ([]corev1.Pod, error) {
	// util.GetComponentPodList support deployment
	return getReplicationSetPods(params)
}

func getReplicationSetPods(params reconfigureParams) ([]corev1.Pod, error) {
	var ctx = params.Ctx
	var cluster = params.Cluster
	podList, err := util.GetComponentPodList(ctx.Ctx, params.Client, *cluster, params.ClusterComponent.Name)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

// GetComponentPods get all pods of the component.
func GetComponentPods(params reconfigureParams) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.ComponentUnits {
		pods, err := util.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, &params.ComponentUnits[i])
		if err != nil {
			return nil, err
		}
		componentPods = append(componentPods, pods...)
	}
	return componentPods, nil
}

// CheckReconfigureUpdateProgress checks pods of the component is ready.
func CheckReconfigureUpdateProgress(pods []corev1.Pod, configKey, version string) int32 {
	var (
		readyPods        int32 = 0
		cfgAnnotationKey       = cfgcore.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	)

	for _, pod := range pods {
		annotations := pod.Annotations
		if len(annotations) != 0 && annotations[cfgAnnotationKey] == version && intctrlutil.PodIsReady(&pod) {
			readyPods++
		}
	}
	return readyPods
}

func getStatefulSetPods(params reconfigureParams) ([]corev1.Pod, error) {
	if len(params.ComponentUnits) != 1 {
		return nil, cfgcore.MakeError("statefulSet component require only one statefulset, actual %d component", len(params.ComponentUnits))
	}

	stsObj := &params.ComponentUnits[0]
	pods, err := util.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(pods, func(i, j int) bool {
		_, ordinal1 := intctrlutil.GetParentNameAndOrdinal(&pods[i])
		_, ordinal2 := intctrlutil.GetParentNameAndOrdinal(&pods[j])
		return ordinal1 < ordinal2
	})
	return pods, nil
}

func getConsensusPods(params reconfigureParams) ([]corev1.Pod, error) {
	if len(params.ComponentUnits) > 1 {
		return nil, cfgcore.MakeError("consensus component require only one statefulset, actual %d component", len(params.ComponentUnits))
	}

	if len(params.ComponentUnits) == 0 {
		return nil, nil
	}

	stsObj := &params.ComponentUnits[0]
	pods, err := util.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
	if err != nil {
		return nil, err
	}

	// TODO: should resolve the dependency on consensus module
	util.SortPods(pods, consensus.ComposeRolePriorityMap(params.Component.ConsensusSpec), constant.RoleLabelKey)
	r := make([]corev1.Pod, 0, len(pods))
	for i := len(pods); i > 0; i-- {
		r = append(r, pods[i-1:i]...)
	}
	return r, nil
}

// TODO commonOnlineUpdateWithPod migrate to sql command pipeline
func commonOnlineUpdateWithPod(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, updatedParams map[string]string) error {
	address, err := cfgManagerGrpcURL(pod)
	if err != nil {
		return err
	}
	client, err := createClient(address)
	if err != nil {
		return err
	}

	response, err := client.OnlineUpgradeParams(ctx, &cfgproto.OnlineUpgradeParamsRequest{
		ConfigSpec: configSpec,
		Params:     updatedParams,
	})
	if err != nil {
		return err
	}

	errMessage := response.GetErrMessage()
	if errMessage != "" {
		return cfgcore.MakeError(errMessage)
	}
	return nil
}

func commonStopContainerWithPod(pod *corev1.Pod, ctx context.Context, containerNames []string, createClient createReconfigureClient) error {
	containerIDs := make([]string, 0, len(containerNames))
	for _, name := range containerNames {
		containerID := intctrlutil.GetContainerID(pod, name)
		if containerID == "" {
			return cfgcore.MakeError("failed to find container in pod[%s], name=%s", name, pod.Name)
		}
		containerIDs = append(containerIDs, containerID)
	}

	address, err := cfgManagerGrpcURL(pod)
	if err != nil {
		return err
	}
	// stop container
	client, err := createClient(address)
	if err != nil {
		return err
	}

	response, err := client.StopContainer(ctx, &cfgproto.StopContainerRequest{
		ContainerIDs: containerIDs,
	})
	if err != nil {
		return err
	}

	errMessage := response.GetErrMessage()
	if errMessage != "" {
		return cfgcore.MakeError(errMessage)
	}
	return nil
}

func cfgManagerGrpcURL(pod *corev1.Pod) (string, error) {
	podPort := viper.GetInt(constant.ConfigManagerGPRCPortEnv)
	return getURLFromPod(pod, podPort)
}

func getURLFromPod(pod *corev1.Pod, portPort int) (string, error) {
	ip := net.ParseIP(pod.Status.PodIP)
	if ip == nil {
		return "", cfgcore.MakeError("%s is not a valid IP", pod.Status.PodIP)
	}

	// Sanity check PodIP
	if ip.To4() == nil && ip.To16() == nil {
		return "", fmt.Errorf("%s is not a valid IPv4/IPv6 address", pod.Status.PodIP)
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(portPort)), nil
}
