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
	"fmt"
	"net"
	"net/netip"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// GetComponentPods gets all pods of the component.
func GetComponentPods(params reconfigureContext) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.InstanceSetUnits {
		pods, err := intctrlutil.GetPodListByInstanceSet(params.Ctx, params.Client, &params.InstanceSetUnits[i])
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

func getPodsForOnlineUpdate(params reconfigureContext) ([]corev1.Pod, error) {
	if len(params.InstanceSetUnits) > 1 {
		return nil, core.MakeError("component require only one InstanceSet, actual %d components", len(params.InstanceSetUnits))
	}

	if len(params.InstanceSetUnits) == 0 {
		return nil, nil
	}

	pods, err := GetComponentPods(params)
	if err != nil {
		return nil, err
	}

	if params.SynthesizedComponent != nil {
		instanceset.SortPods(pods, instanceset.ComposeRolePriorityMap(component.ConvertSynthesizeCompRoleToInstanceSetRole(params.SynthesizedComponent)), true)
	}
	return pods, nil
}

// TODO commonOnlineUpdateWithPod migrate to sql command pipeline
func commonOnlineUpdateWithPod(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, configFile string, updatedParams map[string]string) error {
	address, err := resolveReloadServerGrpcURL(pod)
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
		ConfigFile: pointer.String(configFile),
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

	address, err := resolveReloadServerGrpcURL(pod)
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

func resolveReloadServerGrpcURL(pod *corev1.Pod) (string, error) {
	podPort := viper.GetInt(constant.ConfigManagerGPRCPortEnv)
	if pod.Spec.HostNetwork {
		containerPort, err := configuration.ResolveReloadServerGRPCPort(pod.Spec.Containers)
		if err != nil {
			return "", err
		}
		podPort = int(containerPort)
	}
	return generateGrpcURL(pod, podPort)
}

func generateGrpcURL(pod *corev1.Pod, portPort int) (string, error) {
	ip, err := ipAddressFromPod(pod.Status)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(portPort)), nil
}

func ipAddressFromPod(status corev1.PodStatus) (net.IP, error) {
	// IPv4 address priority
	for _, ip := range status.PodIPs {
		address, err := netip.ParseAddr(ip.IP)
		if err != nil || address.Is6() {
			continue
		}
		return net.ParseIP(ip.IP), nil
	}

	// Using status.PodIP
	address := net.ParseIP(status.PodIP)
	if !validIPv4Address(address) && !validIPv6Address(address) {
		return nil, fmt.Errorf("%s is not a valid IPv4/IPv6 address", status.PodIP)
	}
	return address, nil
}

func validIPv4Address(ip net.IP) bool {
	return ip != nil && ip.To4() != nil
}

func validIPv6Address(ip net.IP) bool {
	return ip != nil && ip.To16() != nil
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
		case *workloads.InstanceSet:
			err = restartWorkloadComponent(cli, ctx.Ctx, cfgAnnotationKey, newVersion, w, generics.InstanceSetSignature)
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

type ReloadAction interface {
	ExecReload() (ReturnedStatus, error)
	ReloadType() string
}

type reconfigureTask struct {
	parametersv1alpha1.ReloadPolicy
	taskCtx reconfigureContext
}

func (r reconfigureTask) ReloadType() string {
	return string(r.ReloadPolicy)
}

func (r reconfigureTask) ExecReload() (ReturnedStatus, error) {
	if executor, ok := upgradePolicyMap[r.ReloadPolicy]; ok {
		return executor.Upgrade(r.taskCtx)
	}

	return ReturnedStatus{}, fmt.Errorf("not support reload action[%s]", r.ReloadPolicy)
}

func resolveReloadActionPolicy(jsonPatch string,
	format *parametersv1alpha1.FileFormatConfig,
	pd *parametersv1alpha1.ParametersDefinitionSpec) (parametersv1alpha1.ReloadPolicy, error) {
	var policy = parametersv1alpha1.NonePolicy
	dynamicUpdate, err := core.CheckUpdateDynamicParameters(format, pd, jsonPatch)
	if err != nil {
		return policy, err
	}

	// make decision
	switch {
	case !dynamicUpdate && intctrlutil.NeedDynamicReloadAction(pd): // static parameters update
		policy = parametersv1alpha1.DynamicReloadAndRestartPolicy
	case cfgcm.IsAutoReload(pd.ReloadAction): // if core support hot update, don't need to do anything
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	case enableSyncTrigger(pd.ReloadAction): // sync config-manager exec hot update
		policy = parametersv1alpha1.SyncDynamicReloadPolicy
	default: // config-manager auto trigger to hot update
		policy = parametersv1alpha1.AsyncDynamicReloadPolicy
	}
	return policy, nil
}

// genReconfigureActionTasks generates a list of reconfiguration tasks based on the provided templateSpec,
// reconfiguration context, configuration patch, and a restart flag.
func genReconfigureActionTasks(templateSpec *appsv1.ComponentTemplateSpec, rctx *ReconcileContext, patch *core.ConfigPatchInfo, restart bool) ([]ReloadAction, error) {
	var tasks []ReloadAction

	// If the patch or ConfigRender is nil, return a single restart task.
	if patch == nil || rctx.ConfigRender == nil {
		return []ReloadAction{buildRestartTask(templateSpec, rctx)}, nil
	}

	// needReloadAction determines if a reload action is needed based on the ParametersDefinition and ReloadPolicy.
	needReloadAction := func(pd *parametersv1alpha1.ParametersDefinition, policy parametersv1alpha1.ReloadPolicy) bool {
		return !restart || (policy == parametersv1alpha1.SyncDynamicReloadPolicy && intctrlutil.NeedDynamicReloadAction(&pd.Spec))
	}

	for key, jsonPatch := range patch.UpdateConfig {
		pd, ok := rctx.ParametersDefs[key]
		// If the ParametersDefinition or its ReloadAction is nil, continue to the next iteration.
		if !ok || pd.Spec.ReloadAction == nil {
			continue
		}
		configFormat := intctrlutil.GetComponentConfigDescription(&rctx.ConfigRender.Spec, key)
		if configFormat == nil || configFormat.FileFormatConfig == nil {
			continue
		}
		// Determine the appropriate ReloadPolicy.
		policy, err := resolveReloadActionPolicy(string(jsonPatch), configFormat.FileFormatConfig, &pd.Spec)
		if err != nil {
			return nil, err
		}
		// If a reload action is needed, append a new reload action task to the tasks slice.
		if needReloadAction(pd, policy) {
			tasks = append(tasks, buildReloadActionTask(policy, templateSpec, rctx, pd, configFormat, patch))
		}
	}

	// If no tasks were added, return a single restart task.
	if len(tasks) == 0 {
		return []ReloadAction{buildRestartTask(templateSpec, rctx)}, nil
	}

	return tasks, nil
}

func buildReloadActionTask(reloadPolicy parametersv1alpha1.ReloadPolicy, templateSpec *appsv1.ComponentTemplateSpec, rctx *ReconcileContext, pd *parametersv1alpha1.ParametersDefinition, configDescription *parametersv1alpha1.ComponentConfigDescription, patch *core.ConfigPatchInfo) reconfigureTask {
	reCtx := reconfigureContext{
		RequestCtx:               rctx.RequestCtx,
		Client:                   rctx.Client,
		ConfigTemplate:           *templateSpec,
		ConfigMap:                rctx.ConfigMap,
		ParametersDef:            &pd.Spec,
		ConfigDescription:        configDescription,
		Cluster:                  rctx.ClusterObj,
		ContainerNames:           rctx.Containers,
		InstanceSetUnits:         rctx.InstanceSetList,
		ClusterComponent:         rctx.ClusterComObj,
		SynthesizedComponent:     rctx.BuiltinComponent,
		ReconfigureClientFactory: GetClientFactory(),
	}

	if reloadPolicy == parametersv1alpha1.SyncDynamicReloadPolicy {
		reCtx.UpdatedParameters = generateOnlineUpdateParams(patch, &pd.Spec, *configDescription)
	}

	return reconfigureTask{ReloadPolicy: reloadPolicy, taskCtx: reCtx}
}

func buildRestartTask(configTemplate *appsv1.ComponentTemplateSpec, rctx *ReconcileContext) reconfigureTask {
	return reconfigureTask{
		ReloadPolicy: parametersv1alpha1.RestartPolicy,
		taskCtx: reconfigureContext{
			RequestCtx:           rctx.RequestCtx,
			Client:               rctx.Client,
			ConfigTemplate:       *configTemplate,
			ClusterComponent:     rctx.ClusterComObj,
			SynthesizedComponent: rctx.BuiltinComponent,
			InstanceSetUnits:     rctx.InstanceSetList,
			ConfigMap:            rctx.ConfigMap,
		},
	}
}

func generateOnlineUpdateParams(configPatch *core.ConfigPatchInfo, paramDef *parametersv1alpha1.ParametersDefinitionSpec, description parametersv1alpha1.ComponentConfigDescription) map[string]string {
	params := make(map[string]string)
	dynamicAction := intctrlutil.NeedDynamicReloadAction(paramDef)
	needReloadStaticParams := intctrlutil.ReloadStaticParameters(paramDef)
	visualizedParams := core.GenerateVisualizedParamsList(configPatch, []parametersv1alpha1.ComponentConfigDescription{description})

	for _, key := range visualizedParams {
		if key.UpdateType != core.UpdatedType {
			continue
		}
		for _, p := range key.Parameters {
			if dynamicAction && !needReloadStaticParams && !core.IsDynamicParameter(p.Key, paramDef) {
				continue
			}
			if p.Value != nil {
				params[p.Key] = *p.Value
			}
		}
	}
	return params
}
