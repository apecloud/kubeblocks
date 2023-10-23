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

package component

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/class"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func BuildComponent(reqCtx intctrlutil.RequestCtx,
	clsMgr *class.Manager,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	serviceReferences map[string]*appsv1alpha1.ServiceDescriptor,
	clusterCompVers ...*appsv1alpha1.ClusterComponentVersion,
) (*SynthesizedComponent, error) {
	return buildComponent(reqCtx, clsMgr, cluster, clusterDef, clusterCompDef, clusterCompSpec, serviceReferences, clusterCompVers...)
}

// buildComponent generates a new Component object, which is a mixture of
// component-related configs from input Cluster, ClusterDef and ClusterVersion.
func buildComponent(reqCtx intctrlutil.RequestCtx,
	clsMgr *class.Manager,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	serviceReferences map[string]*appsv1alpha1.ServiceDescriptor,
	clusterCompVers ...*appsv1alpha1.ClusterComponentVersion,
) (*SynthesizedComponent, error) {
	hasSimplifiedAPI := func() bool {
		return cluster.Spec.Replicas != nil ||
			!cluster.Spec.Resources.CPU.IsZero() ||
			!cluster.Spec.Resources.Memory.IsZero() ||
			!cluster.Spec.Storage.Size.IsZero() ||
			cluster.Spec.Monitor.MonitoringInterval != nil ||
			cluster.Spec.Network != nil ||
			len(cluster.Spec.Tenancy) > 0 ||
			len(cluster.Spec.AvailabilityPolicy) > 0
	}

	fillSimplifiedAPI := func() {
		// fill simplified api only to first defined component
		if len(clusterDef.Spec.ComponentDefs) == 0 ||
			clusterDef.Spec.ComponentDefs[0].Name != clusterCompDef.Name {
			return
		}
		// return if none of simplified api is defined
		if !hasSimplifiedAPI() {
			return
		}
		if clusterCompSpec == nil {
			clusterCompSpec = &appsv1alpha1.ClusterComponentSpec{}
			clusterCompSpec.Name = clusterCompDef.Name
		}
		if cluster.Spec.Replicas != nil {
			clusterCompSpec.Replicas = *cluster.Spec.Replicas
		}
		dataVolumeName := "data"
		for _, v := range clusterCompDef.VolumeTypes {
			if v.Type == appsv1alpha1.VolumeTypeData {
				dataVolumeName = v.Name
			}
		}
		if !cluster.Spec.Resources.CPU.IsZero() || !cluster.Spec.Resources.Memory.IsZero() {
			clusterCompSpec.Resources.Limits = corev1.ResourceList{}
		}
		if !cluster.Spec.Resources.CPU.IsZero() {
			clusterCompSpec.Resources.Limits["cpu"] = cluster.Spec.Resources.CPU
		}
		if !cluster.Spec.Resources.Memory.IsZero() {
			clusterCompSpec.Resources.Limits["memory"] = cluster.Spec.Resources.Memory
		}
		if !cluster.Spec.Storage.Size.IsZero() {
			clusterCompSpec.VolumeClaimTemplates = []appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				{
					Name: dataVolumeName,
					Spec: appsv1alpha1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"storage": cluster.Spec.Storage.Size,
							},
						},
					},
				},
			}
		}
		if cluster.Spec.Monitor.MonitoringInterval != nil {
			if len(cluster.Spec.Monitor.MonitoringInterval.StrVal) == 0 && cluster.Spec.Monitor.MonitoringInterval.IntVal == 0 {
				clusterCompSpec.Monitor = false
			} else {
				clusterCompSpec.Monitor = true
				// TODO: should also set interval
			}
		}
		if cluster.Spec.Network != nil {
			clusterCompSpec.Services = []appsv1alpha1.ClusterComponentService{}
			if cluster.Spec.Network.HostNetworkAccessible {
				svc := appsv1alpha1.ClusterComponentService{
					Name:        "vpc",
					ServiceType: "LoadBalancer",
				}
				switch getCloudProvider() {
				case CloudProviderAWS:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
						"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
					}
				case CloudProviderGCP:
					svc.Annotations = map[string]string{
						"networking.gke.io/load-balancer-type": "Internal",
					}
				case CloudProviderAliyun:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "intranet",
					}
				case CloudProviderAzure:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/azure-load-balancer-internal": "true",
					}
				}
				clusterCompSpec.Services = append(clusterCompSpec.Services, svc)
			}
			if cluster.Spec.Network.PubliclyAccessible {
				svc := appsv1alpha1.ClusterComponentService{
					Name:        "public",
					ServiceType: "LoadBalancer",
				}
				switch getCloudProvider() {
				case CloudProviderAWS:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
						"service.beta.kubernetes.io/aws-load-balancer-internal": "false",
					}
				case CloudProviderAliyun:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "internet",
					}
				case CloudProviderAzure:
					svc.Annotations = map[string]string{
						"service.beta.kubernetes.io/azure-load-balancer-internal": "false",
					}
				}
				clusterCompSpec.Services = append(clusterCompSpec.Services, svc)
			}
		}
	}

	// priority: cluster.spec.componentSpecs > simplified api (e.g. cluster.spec.storage etc.) > cluster template
	if clusterCompSpec == nil {
		fillSimplifiedAPI()
	}
	if clusterCompSpec == nil {
		return nil, nil
	}

	var err error
	// make a copy of clusterCompDef
	clusterCompDefObj := clusterCompDef.DeepCopy()
	component := &SynthesizedComponent{
		ClusterDefName:        clusterDef.Name,
		ClusterName:           cluster.Name,
		ClusterUID:            string(cluster.UID),
		Name:                  clusterCompSpec.Name,
		CompDefName:           clusterCompDefObj.Name,
		CharacterType:         clusterCompDefObj.CharacterType,
		WorkloadType:          clusterCompDefObj.WorkloadType,
		StatelessSpec:         clusterCompDefObj.StatelessSpec,
		StatefulSpec:          clusterCompDefObj.StatefulSpec,
		ConsensusSpec:         clusterCompDefObj.ConsensusSpec,
		ReplicationSpec:       clusterCompDefObj.ReplicationSpec,
		RSMSpec:               clusterCompDefObj.RSMSpec,
		PodSpec:               clusterCompDefObj.PodSpec,
		Probes:                clusterCompDefObj.Probes,
		LogConfigs:            clusterCompDefObj.LogConfigs,
		HorizontalScalePolicy: clusterCompDefObj.HorizontalScalePolicy,
		ConfigTemplates:       clusterCompDefObj.ConfigSpecs,
		ScriptTemplates:       clusterCompDefObj.ScriptSpecs,
		VolumeTypes:           clusterCompDefObj.VolumeTypes,
		VolumeProtection:      clusterCompDefObj.VolumeProtectionSpec,
		CustomLabelSpecs:      clusterCompDefObj.CustomLabelSpecs,
		SwitchoverSpec:        clusterCompDefObj.SwitchoverSpec,
		StatefulSetWorkload:   clusterCompDefObj.GetStatefulSetWorkload(),
		MinAvailable:          clusterCompSpec.GetMinAvailable(clusterCompDefObj.GetMinAvailable()),
		Replicas:              clusterCompSpec.Replicas,
		EnabledLogs:           clusterCompSpec.EnabledLogs,
		TLS:                   clusterCompSpec.TLS,
		Issuer:                clusterCompSpec.Issuer,
		ComponentDef:          clusterCompSpec.ComponentDefRef,
		ServiceAccountName:    clusterCompSpec.ServiceAccountName,
	}

	if len(clusterCompVers) > 0 && clusterCompVers[0] != nil {
		// only accept 1st ClusterVersion override context
		clusterCompVer := clusterCompVers[0]
		component.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterCompVer.ConfigSpecs, component.ConfigTemplates)
		// override component.PodSpec.InitContainers and component.PodSpec.Containers
		for _, c := range clusterCompVer.VersionsCtx.InitContainers {
			component.PodSpec.InitContainers = appendOrOverrideContainerAttr(component.PodSpec.InitContainers, c)
		}
		for _, c := range clusterCompVer.VersionsCtx.Containers {
			component.PodSpec.Containers = appendOrOverrideContainerAttr(component.PodSpec.Containers, c)
		}
		// override component.SwitchoverSpec
		overrideSwitchoverSpecAttr(component.SwitchoverSpec, clusterCompVer.SwitchoverSpec)
	}

	// handle component.PodSpec extra settings
	// set affinity and tolerations
	affinity := BuildAffinity(cluster, clusterCompSpec)
	if component.PodSpec.Affinity, err = BuildPodAffinity(cluster, affinity, component); err != nil {
		reqCtx.Log.Error(err, "build pod affinity failed.")
		return nil, err
	}
	component.PodSpec.TopologySpreadConstraints = BuildPodTopologySpreadConstraints(cluster, affinity, component)
	if component.PodSpec.Tolerations, err = BuildTolerations(cluster, clusterCompSpec); err != nil {
		reqCtx.Log.Error(err, "build pod tolerations failed.")
		return nil, err
	}

	if clusterCompSpec.VolumeClaimTemplates != nil {
		component.VolumeClaimTemplates = clusterCompSpec.ToVolumeClaimTemplates()
	}

	if clusterCompSpec.Resources.Requests != nil || clusterCompSpec.Resources.Limits != nil {
		component.PodSpec.Containers[0].Resources = clusterCompSpec.Resources
	}
	if err = updateResources(cluster, component, *clusterCompSpec, clsMgr); err != nil {
		reqCtx.Log.Error(err, "update class resources failed")
		return nil, err
	}

	if clusterCompDefObj.Service != nil {
		service := corev1.Service{Spec: clusterCompDefObj.Service.ToSVCSpec()}
		service.Spec.Type = corev1.ServiceTypeClusterIP
		component.Services = append(component.Services, service)
		for _, item := range clusterCompSpec.Services {
			service = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        item.Name,
					Annotations: item.Annotations,
				},
				Spec: service.Spec,
			}
			service.Spec.Type = item.ServiceType
			component.Services = append(component.Services, service)
		}
	}

	buildMonitorConfig(clusterCompDefObj, clusterCompSpec, component)

	// lorry container requires a service account with adequate privileges.
	// If lorry required and the serviceAccountName is not set,
	// a default serviceAccountName will be assigned.
	if component.ServiceAccountName == "" && component.Probes != nil {
		component.ServiceAccountName = "kb-" + component.ClusterName
	}
	// set component.PodSpec.ServiceAccountName
	component.PodSpec.ServiceAccountName = component.ServiceAccountName
	if err = buildLorryContainers(reqCtx, component); err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
		return nil, err
	}

	replaceContainerPlaceholderTokens(component, GetEnvReplacementMapForConnCredential(cluster.GetName()))

	if err = buildComponentRef(clusterDef, cluster, clusterCompDefObj, clusterCompSpec, component); err != nil {
		reqCtx.Log.Error(err, "failed to merge componentRef")
		return nil, err
	}

	if serviceReferences != nil {
		component.ServiceReferences = serviceReferences
	}

	return component, nil
}

// appendOrOverrideContainerAttr appends targetContainer to compContainers or overrides the attributes of compContainers with a given targetContainer,
// if targetContainer does not exist in compContainers, it will be appended. otherwise it will be updated with the attributes of the target container.
func appendOrOverrideContainerAttr(compContainers []corev1.Container, targetContainer corev1.Container) []corev1.Container {
	index, compContainer := intctrlutil.GetContainerByName(compContainers, targetContainer.Name)
	if compContainer == nil {
		compContainers = append(compContainers, targetContainer)
	} else {
		doContainerAttrOverride(&compContainers[index], targetContainer)
	}
	return compContainers
}

// doContainerAttrOverride overrides the attributes in compContainer with the attributes in container.
func doContainerAttrOverride(compContainer *corev1.Container, container corev1.Container) {
	if compContainer == nil {
		return
	}
	if container.Image != "" {
		compContainer.Image = container.Image
	}
	if len(container.Command) != 0 {
		compContainer.Command = container.Command
	}
	if len(container.Args) != 0 {
		compContainer.Args = container.Args
	}
	if container.WorkingDir != "" {
		compContainer.WorkingDir = container.WorkingDir
	}
	if len(container.Ports) != 0 {
		compContainer.Ports = container.Ports
	}
	if len(container.EnvFrom) != 0 {
		compContainer.EnvFrom = container.EnvFrom
	}
	if len(container.Env) != 0 {
		compContainer.Env = container.Env
	}
	if container.Resources.Limits != nil || container.Resources.Requests != nil {
		compContainer.Resources = container.Resources
	}
	if len(container.VolumeMounts) != 0 {
		compContainer.VolumeMounts = container.VolumeMounts
	}
	if len(container.VolumeDevices) != 0 {
		compContainer.VolumeDevices = container.VolumeDevices
	}
	if container.LivenessProbe != nil {
		compContainer.LivenessProbe = container.LivenessProbe
	}
	if container.ReadinessProbe != nil {
		compContainer.ReadinessProbe = container.ReadinessProbe
	}
	if container.StartupProbe != nil {
		compContainer.StartupProbe = container.StartupProbe
	}
	if container.Lifecycle != nil {
		compContainer.Lifecycle = container.Lifecycle
	}
	if container.TerminationMessagePath != "" {
		compContainer.TerminationMessagePath = container.TerminationMessagePath
	}
	if container.TerminationMessagePolicy != "" {
		compContainer.TerminationMessagePolicy = container.TerminationMessagePolicy
	}
	if container.ImagePullPolicy != "" {
		compContainer.ImagePullPolicy = container.ImagePullPolicy
	}
	if container.SecurityContext != nil {
		compContainer.SecurityContext = container.SecurityContext
	}
}

// GetEnvReplacementMapForConnCredential gets the replacement map for connect credential
func GetEnvReplacementMapForConnCredential(clusterName string) map[string]string {
	return map[string]string{
		constant.KBConnCredentialPlaceHolder: GenerateConnCredential(clusterName),
	}
}

func replaceContainerPlaceholderTokens(component *SynthesizedComponent, namedValuesMap map[string]string) {
	// replace env[].valueFrom.secretKeyRef.name variables
	for _, cc := range [][]corev1.Container{component.PodSpec.InitContainers, component.PodSpec.Containers} {
		for _, c := range cc {
			c.Env = ReplaceSecretEnvVars(namedValuesMap, c.Env)
		}
	}
}

// GetReplacementMapForBuiltInEnv gets the replacement map for KubeBlocks built-in environment variables.
func GetReplacementMapForBuiltInEnv(clusterName, clusterUID, componentName string) map[string]string {
	cc := fmt.Sprintf("%s-%s", clusterName, componentName)
	replacementMap := map[string]string{
		constant.KBClusterNamePlaceHolder:     clusterName,
		constant.KBCompNamePlaceHolder:        componentName,
		constant.KBClusterCompNamePlaceHolder: cc,
		constant.KBComponentEnvCMPlaceHolder:  fmt.Sprintf("%s-env", cc),
	}
	if len(clusterUID) > 8 {
		replacementMap[constant.KBClusterUIDPostfix8PlaceHolder] = clusterUID[len(clusterUID)-8:]
	} else {
		replacementMap[constant.KBClusterUIDPostfix8PlaceHolder] = clusterUID
	}
	return replacementMap
}

// ReplaceNamedVars replaces the placeholder in targetVar if it is match and returns the replaced result
func ReplaceNamedVars(namedValuesMap map[string]string, targetVar string, limits int, matchAll bool) string {
	for placeHolderKey, mappingValue := range namedValuesMap {
		r := strings.Replace(targetVar, placeHolderKey, mappingValue, limits)
		// early termination on matching, when matchAll = false
		if r != targetVar && !matchAll {
			return r
		}
		targetVar = r
	}
	return targetVar
}

// ReplaceSecretEnvVars replaces the env secret value with namedValues and returns new envs
func ReplaceSecretEnvVars(namedValuesMap map[string]string, envs []corev1.EnvVar) []corev1.EnvVar {
	newEnvs := make([]corev1.EnvVar, 0, len(envs))
	for _, e := range envs {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		name := ReplaceNamedVars(namedValuesMap, e.ValueFrom.SecretKeyRef.Name, 1, false)
		if name != e.ValueFrom.SecretKeyRef.Name {
			e.ValueFrom.SecretKeyRef.Name = name
		}
		newEnvs = append(newEnvs, e)
	}
	return newEnvs
}

func GenerateConnCredential(clusterName string) string {
	return fmt.Sprintf("%s-conn-credential", clusterName)
}

func GenerateDefaultServiceDescriptorName(clusterName string) string {
	return fmt.Sprintf("kbsd-%s", GenerateConnCredential(clusterName))
}

// overrideSwitchoverSpecAttr overrides the attributes in switchoverSpec with the attributes of SwitchoverShortSpec in clusterVersion.
func overrideSwitchoverSpecAttr(switchoverSpec *appsv1alpha1.SwitchoverSpec, cvSwitchoverSpec *appsv1alpha1.SwitchoverShortSpec) {
	if switchoverSpec == nil || cvSwitchoverSpec == nil || cvSwitchoverSpec.CmdExecutorConfig == nil {
		return
	}
	applyCmdExecutorConfig := func(cmdExecutorConfig *appsv1alpha1.CmdExecutorConfig) {
		if cmdExecutorConfig == nil {
			return
		}
		if len(cvSwitchoverSpec.CmdExecutorConfig.Image) > 0 {
			cmdExecutorConfig.Image = cvSwitchoverSpec.CmdExecutorConfig.Image
		}
		if len(cvSwitchoverSpec.CmdExecutorConfig.Env) > 0 {
			cmdExecutorConfig.Env = cvSwitchoverSpec.CmdExecutorConfig.Env
		}
	}
	if switchoverSpec.WithCandidate != nil {
		applyCmdExecutorConfig(switchoverSpec.WithCandidate.CmdExecutorConfig)
	}
	if switchoverSpec.WithoutCandidate != nil {
		applyCmdExecutorConfig(switchoverSpec.WithoutCandidate.CmdExecutorConfig)
	}
}

func GenerateComponentEnvName(clusterName, componentName string) string {
	return fmt.Sprintf("%s-%s-env", clusterName, componentName)
}

func updateResources(cluster *appsv1alpha1.Cluster, component *SynthesizedComponent, clusterCompSpec appsv1alpha1.ClusterComponentSpec, clsMgr *class.Manager) error {
	if ignoreResourceConstraint(cluster) {
		return nil
	}

	if clsMgr == nil {
		return nil
	}

	expectResources, err := clsMgr.GetResources(cluster.Spec.ClusterDefRef, &clusterCompSpec)
	if err != nil || expectResources == nil {
		return err
	}

	actualResources := component.PodSpec.Containers[0].Resources
	if actualResources.Requests == nil {
		actualResources.Requests = corev1.ResourceList{}
	}
	if actualResources.Limits == nil {
		actualResources.Limits = corev1.ResourceList{}
	}
	for k, v := range expectResources {
		actualResources.Requests[k] = v
		actualResources.Limits[k] = v
	}
	component.PodSpec.Containers[0].Resources = actualResources
	return nil
}

func getCloudProvider() CloudProvider {
	k8sVersion := viper.GetString(constant.CfgKeyServerInfo)
	if strings.Contains(k8sVersion, "eks") {
		return CloudProviderAWS
	}
	if strings.Contains(k8sVersion, "gke") {
		return CloudProviderGCP
	}
	if strings.Contains(k8sVersion, "aliyun") {
		return CloudProviderAliyun
	}
	if strings.Contains(k8sVersion, "tke") {
		return CloudProviderTencent
	}
	return CloudProviderUnknown
}

func GetConfigSpecByName(component *SynthesizedComponent, configSpec string) *appsv1alpha1.ComponentConfigSpec {
	for i := range component.ConfigTemplates {
		template := &component.ConfigTemplates[i]
		if template.Name == configSpec {
			return template
		}
	}
	return nil
}
