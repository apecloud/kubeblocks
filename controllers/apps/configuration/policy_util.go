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
	"strconv"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// GetComponentPods gets all pods of the component.
func GetComponentPods(params reconfigureParams) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.ComponentUnits {
		pods, err := common.GetPodListByStatefulSet(params.Ctx.Ctx, params.Client, &params.ComponentUnits[i])
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

func getPodsForOnlineUpdate(params reconfigureParams) ([]corev1.Pod, error) {
	if len(params.ComponentUnits) > 1 {
		return nil, core.MakeError("component require only one statefulSet, actual %d components", len(params.ComponentUnits))
	}

	if len(params.ComponentUnits) == 0 {
		return nil, nil
	}

	pods, err := GetComponentPods(params)
	// stsObj := &params.ComponentUnits[0]
	// pods, err := components.GetPodListByStatefulSetWithSelector(params.Ctx.Ctx, params.Client, stsObj, client.MatchingLabels{
	//	constant.KBAppComponentLabelKey: params.ClusterComponent.Name,
	//	constant.AppInstanceLabelKey:    params.Cluster.Name,
	// })
	if err != nil {
		return nil, err
	}

	if params.SynthesizedComponent != nil {
		rsmcore.SortPods(pods, rsmcore.ComposeRolePriorityMap(component.ConvertSynthesizeCompRoleToRSMRole(params.SynthesizedComponent)), true)
	}
	return pods, nil
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

func restartWorkloadComponent[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](cli client.Client, ctx context.Context, annotationKey, annotationValue string, obj PT, _ func(T, PT, L, PL)) error {
	template := transformPodTemplate(obj)
	if updatedVersion(template, annotationKey, annotationValue) {
		return nil
	}

	patch := client.MergeFrom(PT(obj.DeepCopy()))
	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}
	template.Annotations[annotationKey] = annotationValue
	if err := cli.Patch(ctx, obj, patch); err != nil {
		return err
	}
	return nil
}

func restartComponent(cli client.Client, ctx intctrlutil.RequestCtx, configKey string, newVersion string, objs []client.Object, recordEvent func(obj client.Object)) (client.Object, error) {
	var err error
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	for _, obj := range objs {
		switch w := obj.(type) {
		case *appv1.StatefulSet:
			err = restartWorkloadComponent(cli, ctx.Ctx, cfgAnnotationKey, newVersion, w, generics.StatefulSetSignature)
		case *appv1.Deployment:
			err = restartWorkloadComponent(cli, ctx.Ctx, cfgAnnotationKey, newVersion, w, generics.DeploymentSignature)
		case *workloads.ReplicatedStateMachine:
			err = restartWorkloadComponent(cli, ctx.Ctx, cfgAnnotationKey, newVersion, w, generics.RSMSignature)
		default:
			// ignore other types workload
		}
		if err != nil {
			return obj, err
		}
		if recordEvent != nil {
			recordEvent(obj)
		}
	}
	return nil, nil
}

func updatedVersion(podTemplate *corev1.PodTemplateSpec, keyPath, expectedVersion string) bool {
	return podTemplate.Annotations != nil && podTemplate.Annotations[keyPath] == expectedVersion
}
