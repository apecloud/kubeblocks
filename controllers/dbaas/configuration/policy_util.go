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
	"context"
	"fmt"
	"sort"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/controllers/dbaas/components/consensusset"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type createReconfigureClient func(addr string) (cfgproto.ReconfigureClient, error)

type GetPodsFunc func(params reconfigureParams) ([]corev1.Pod, error)

type RestartContainerFunc func(pod *corev1.Pod, containerName []string, createConnFn createReconfigureClient) error

type RollingUpgradeFuncs struct {
	GetPodsFunc          GetPodsFunc
	RestartContainerFunc RestartContainerFunc
}

func GetConsensusRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getConsensusPods,
		RestartContainerFunc: commonStopContainer,
	}
}

func GetStatefulSetRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getStatefulSetPods,
		RestartContainerFunc: commonStopContainer,
	}
}

func GetReplicationRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getReplicationSetPods,
		RestartContainerFunc: commonStopContainer,
	}
}

func getReplicationSetPods(params reconfigureParams) ([]corev1.Pod, error) {
	panic("")
}

func getStatefulSetPods(params reconfigureParams) ([]corev1.Pod, error) {
	if len(params.ComponentUnits) != 1 {
		return nil, cfgcore.MakeError("statefulSet component require only one statefulset, actual %d component", len(params.ComponentUnits))
	}

	stsObj := &params.ComponentUnits[0]
	pods, err := consensusset.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
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
	pods, err := consensusset.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
	if err != nil {
		return nil, err
	}

	// sort pods
	consensusset.SortPods(pods, consensusset.ComposeRolePriorityMap(*params.Component))
	r := make([]corev1.Pod, 0, len(pods))
	for i := len(pods); i > 0; i-- {
		r = append(r, pods[i-1:i]...)
	}
	return r, nil
}

func commonStopContainer(pod *corev1.Pod, containerNames []string, createClient createReconfigureClient) error {
	containerIDs := make([]string, 0, len(containerNames))
	for _, name := range containerNames {
		containerID := intctrlutil.GetContainerID(pod, name)
		if containerID == "" {
			return cfgcore.MakeError("failed to find container in pod[%s], name=%s", name, pod.Name)
		}
		containerIDs = append(containerIDs, containerID)
	}

	// stop container
	client, err := createClient(generateManagerSidecarAddr(pod))
	if err != nil {
		return err
	}

	response, err := client.StopContainer(context.Background(), &cfgproto.StopContainerRequest{
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

func generateManagerSidecarAddr(pod *corev1.Pod) string {
	var (
		podAddress = pod.Status.PodIP
		podPort    = viper.GetInt32(cfgcore.ConfigManagerGPRCPortEnv)
	)
	return fmt.Sprintf("%s:%d", podAddress, podPort)
}
