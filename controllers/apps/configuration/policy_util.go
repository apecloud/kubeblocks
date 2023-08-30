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

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

func getDeploymentRollingPods(params reconfigureParams) ([]corev1.Pod, error) {
	// util.GetComponentPodList supports deployment
	return getReplicationSetPods(params)
}

func getReplicationSetPods(params reconfigureParams) ([]corev1.Pod, error) {
	var ctx = params.Ctx
	var cluster = params.Cluster
	podList, err := components.GetComponentPodList(ctx.Ctx, params.Client, *cluster, params.ClusterComponent.Name)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

// GetComponentPods gets all pods of the component.
func GetComponentPods(params reconfigureParams) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.ComponentUnits {
		pods, err := components.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, &params.ComponentUnits[i])
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
		cfgAnnotationKey       = core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
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
		return nil, core.MakeError("statefulSet component require only one statefulset, actual %d components", len(params.ComponentUnits))
	}

	stsObj := &params.ComponentUnits[0]
	pods, err := components.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
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
		return nil, core.MakeError("consensus component require only one statefulset, actual %d components", len(params.ComponentUnits))
	}

	if len(params.ComponentUnits) == 0 {
		return nil, nil
	}

	stsObj := &params.ComponentUnits[0]
	pods, err := components.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, stsObj)
	if err != nil {
		return nil, err
	}

	// TODO: should resolve the dependency on consensus module
	components.SortPods(pods, components.ComposeRolePriorityMap(params.Component.ConsensusSpec), constant.RoleLabelKey)
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
		return core.MakeError(errMessage)
	}
	return nil
}

func commonStopContainerWithPod(pod *corev1.Pod, ctx context.Context, containerNames []string, createClient createReconfigureClient) error {
	containerIDs := make([]string, 0, len(containerNames))
	for _, name := range containerNames {
		containerID := intctrlutil.GetContainerID(pod, name)
		if containerID == "" {
			return core.MakeError("failed to find container in pod[%s], name=%s", name, pod.Name)
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
		return core.MakeError(errMessage)
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
		return "", core.MakeError("%s is not a valid IP", pod.Status.PodIP)
	}

	// Sanity check PodIP
	if ip.To4() == nil && ip.To16() == nil {
		return "", fmt.Errorf("%s is not a valid IPv4/IPv6 address", pod.Status.PodIP)
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(portPort)), nil
}

func restartStatelessComponent(client client.Client, ctx intctrlutil.RequestCtx, configKey string, expectedVersion string, deployObjs []client.Object, recordEvent func(obj client.Object)) (client.Object, error) {
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	deployRestart := func(deploy *appv1.Deployment, expectedVersion string) error {
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = map[string]string{}
		}
		deploy.Spec.Template.Annotations[cfgAnnotationKey] = expectedVersion
		if err := client.Update(ctx.Ctx, deploy); err != nil {
			return err
		}
		return nil
	}

	for _, obj := range deployObjs {
		deploy, ok := obj.(*appv1.Deployment)
		if !ok {
			continue
		}
		if updatedVersion(&deploy.Spec.Template, cfgAnnotationKey, expectedVersion) {
			continue
		}
		if err := deployRestart(deploy, expectedVersion); err != nil {
			return deploy, err
		}
		if recordEvent != nil {
			recordEvent(deploy)
		}
	}
	return nil, nil
}

func restartStatefulComponent(client client.Client, ctx intctrlutil.RequestCtx, configKey string, newVersion string, objs []client.Object, recordEvent func(obj client.Object)) (client.Object, error) {
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	stsRestart := func(sts *appv1.StatefulSet, expectedVersion string) error {
		if sts.Spec.Template.Annotations == nil {
			sts.Spec.Template.Annotations = map[string]string{}
		}
		sts.Spec.Template.Annotations[cfgAnnotationKey] = expectedVersion
		if err := client.Update(ctx.Ctx, sts); err != nil {
			return err
		}
		return nil
	}

	for _, obj := range objs {
		sts, ok := obj.(*appv1.StatefulSet)
		if !ok {
			continue
		}
		if updatedVersion(&sts.Spec.Template, cfgAnnotationKey, newVersion) {
			continue
		}
		if err := stsRestart(sts, newVersion); err != nil {
			return sts, err
		}
		if recordEvent != nil {
			recordEvent(sts)
		}
	}
	return nil, nil
}

func updatedVersion(podTemplate *corev1.PodTemplateSpec, keyPath, expectedVersion string) bool {
	return podTemplate.Annotations != nil && podTemplate.Annotations[keyPath] == expectedVersion
}
