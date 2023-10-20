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

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/class"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	roclient "github.com/apecloud/kubeblocks/pkg/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// BuildSynthesizedComponentWrapper builds a new SynthesizedComponent object with a given ClusterComponentSpec.
func BuildSynthesizedComponentWrapper(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
) (*SynthesizedComponent, error) {

	// if clusterCompSpec is nil, try to build clusterCompSpec from simplified api defined in cluster
	if clusterCompSpec == nil {
		if err := handleSimplifiedAPI(reqCtx, cli, cluster, clusterCompSpec); err != nil {
			return nil, err
		}
	}

	if clusterCompSpec == nil {
		return nil, nil
	}

	compDef, err := BuildComponentDefinition(reqCtx, cli, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	comp, err := BuildProtoComponent(reqCtx, cli, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	return buildSynthesizeComponent(reqCtx, cli, cluster, compDef, comp)
}

// BuildSynthesizedComponent builds a new SynthesizedComponent object, which is a mixture of component-related configs from input Cluster, ComponentDefinition and Component.
func BuildSynthesizedComponent(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component,
) (*SynthesizedComponent, error) {
	return buildSynthesizeComponent(reqCtx, cli, cluster, compDef, comp)
}

// buildSynthesizeComponent builds a new Component object, which is a mixture of component-related configs from input Cluster, ComponentDefinition and Component.
func buildSynthesizeComponent(reqCtx intctrlutil.RequestCtx,
	cli roclient.ReadonlyClient,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component,
) (*SynthesizedComponent, error) {
	if cluster == nil || compDef == nil || comp == nil {
		return nil, nil
	}

	var err error
	compDefObj := compDef.DeepCopy()
	synthesizeComp := &SynthesizedComponent{
		ClusterName:           cluster.Name,
		ClusterUID:            string(cluster.UID),
		Name:                  comp.Name,
		PodSpec:               &compDef.Spec.Runtime,
		LogConfigs:            compDefObj.Spec.LogConfigs,
		ConfigTemplates:       compDefObj.Spec.Configs,
		ScriptTemplates:       compDefObj.Spec.Scripts,
		Labels:                compDefObj.Spec.Labels,
		Roles:                 compDefObj.Spec.Roles,
		ConnectionCredentials: compDefObj.Spec.ConnectionCredentials,
		UpdateStrategy:        compDefObj.Spec.UpdateStrategy,
		PolicyRules:           compDefObj.Spec.PolicyRules,
		LifecycleActions:      compDefObj.Spec.LifecycleActions,
		SystemAccounts:        compDefObj.Spec.SystemAccounts,
		RoleArbitrator:        compDefObj.Spec.RoleArbitrator,
		Replicas:              comp.Spec.Replicas,
		EnabledLogs:           comp.Spec.EnabledLogs,
		TLS:                   comp.Spec.TLS,
		Issuer:                comp.Spec.Issuer,
		ServiceAccountName:    comp.Spec.ServiceAccountName,
	}

	// build backward compatible fields, including workload, services, componentRefEnvs, clusterDefName, clusterCompDefName, and clusterCompVer, etc.
	// if cluster referenced a clusterDefinition and clusterVersion, for backward compatibility, we need to merge the clusterDefinition and clusterVersion into the component
	// TODO(xingran): it will be removed in the future
	if err = buildBackwardCompatibleFields(reqCtx, cli, cluster, synthesizeComp); err != nil {
		return nil, err
	}

	// build affinity and tolerations
	if err := buildAffinitiesAndTolerations(cluster, synthesizeComp, comp); err != nil {
		reqCtx.Log.Error(err, "build affinities and tolerations failed.")
		return nil, err
	}

	// build and update resources
	// TODO(xingran): ComponentResourceConstraint API needs to be restructured.
	if err := buildAndUpdateResources(synthesizeComp, comp); err != nil {
		reqCtx.Log.Error(err, "build and update resources failed.")
		return nil, err
	}

	// build volumeClaimTemplates
	buildVolumeClaimTemplates(synthesizeComp, comp)

	// build componentService
	buildComponentServices(synthesizeComp, compDefObj)

	// build monitor
	buildMonitorConfig(compDefObj.Spec.Monitor, comp.Spec.Monitor, &compDefObj.Spec.Runtime, synthesizeComp)

	// build serviceAccountName
	buildServiceAccountName(synthesizeComp)

	// build lifecycleActions
	buildLifecycleActions(synthesizeComp, compDefObj, comp)

	// build lorryContainer
	// TODO(xingran): buildLorryContainers relies on synthesizeComp.CharacterType and synthesizeComp.Probes, which will be deprecated in the future.
	if err := buildLorryContainers(reqCtx, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "build probe container failed.")
		return nil, err
	}

	// build serviceReferences
	if err = buildServiceReferences(reqCtx, cli, cluster, synthesizeComp, compDef, comp); err != nil {
		reqCtx.Log.Error(err, "build service references failed.")
		return nil, err
	}

	// synthesizeComp podSpec containers placeholder replacement
	replaceContainerPlaceholderTokens(synthesizeComp, GetEnvReplacementMapForConnCredential(cluster.GetName()))

	return synthesizeComp, nil
}

// buildAffinitiesAndTolerations builds affinities and tolerations for component.
func buildAffinitiesAndTolerations(cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	var err error
	affinity := BuildAffinity(cluster, comp.Spec.Affinity)
	if synthesizeComp.PodSpec.Affinity, err = BuildPodAffinity(cluster, affinity, synthesizeComp); err != nil {
		return err
	}
	synthesizeComp.PodSpec.TopologySpreadConstraints = BuildPodTopologySpreadConstraints(cluster, affinity, synthesizeComp)
	if synthesizeComp.PodSpec.Tolerations, err = BuildTolerations(cluster, comp.Spec.Tolerations); err != nil {
		return err
	}
	return nil
}

// buildVolumeClaimTemplates builds volumeClaimTemplates for component.
func buildVolumeClaimTemplates(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) {
	if comp.Spec.VolumeClaimTemplates != nil {
		synthesizeComp.VolumeClaimTemplates = comp.Spec.ToVolumeClaimTemplates()
	}
}

// buildResources builds and updates podSpec resources for component.
func buildAndUpdateResources(synthesizeComp *SynthesizedComponent, comp *appsv1alpha1.Component) error {
	if comp.Spec.Resources.Requests != nil || comp.Spec.Resources.Limits != nil {
		synthesizeComp.PodSpec.Containers[0].Resources = comp.Spec.Resources
	}
	// TODO(xingran): update component resource with ComponentResourceConstraint and ComponentClassDefinition
	// However, the current API related to ComponentClassDefinition and ComponentResourceConstraint heavily relies on cd (ClusterDefinition) and cv (ClusterVersion), requiring a restructuring.
	// if err = updateResources(cluster, component, *clusterCompSpec, clsMgr); err != nil {
	//	reqCtx.Log.Error(err, "update class resources failed")
	//	return nil, err
	// }
	return nil
}

// buildServiceReferences builds serviceReferences for component.
func buildServiceReferences(reqCtx intctrlutil.RequestCtx, cli roclient.ReadonlyClient, cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) error {
	serviceReferences, err := GenServiceReferences(reqCtx, cli, cluster, compDef, comp)
	if err != nil {
		return err
	}
	synthesizeComp.ServiceReferences = serviceReferences
	return nil
}

// buildComponentRef builds componentServices for component.
func buildComponentServices(synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition) {
	if len(compDef.Spec.Services) > 0 {
		synthesizeComp.ComponentServices = compDef.Spec.Services
	}
}

// buildServiceAccountName builds serviceAccountName for component and podSpec.
func buildServiceAccountName(synthesizeComp *SynthesizedComponent) {
	// lorry container requires a service account with adequate privileges.
	// If lorry required and the serviceAccountName is not set, a default serviceAccountName will be assigned.
	if synthesizeComp.ServiceAccountName != "" {
		return
	}
	if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
		return
	}
	synthesizeComp.ServiceAccountName = constant.KBLowerPrefix + constant.KBHyphen + synthesizeComp.ClusterName
	// set component.PodSpec.ServiceAccountName
	synthesizeComp.PodSpec.ServiceAccountName = synthesizeComp.ServiceAccountName
}

// buildLifecycleActions builds lifecycleActions for component.
func buildLifecycleActions(synthesizeComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition, comp *appsv1alpha1.Component) {
	return
}

// buildBackwardCompatibleFields builds backward compatible fields for component which referenced a clusterComponentDefinition and clusterComponentVersion before KubeBlocks Version 0.7.0
// TODO(xingran): it will be removed in the future
func buildBackwardCompatibleFields(reqCtx intctrlutil.RequestCtx, cli roclient.ReadonlyClient, cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent) error {
	if cluster.Spec.ClusterDefRef == "" || cluster.Spec.ClusterVersionRef == "" {
		return nil // no need to build backward compatible fields
	}

	cd := &appsv1alpha1.ClusterDefinition{}
	if err := intctrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd, false); err != nil {
		return err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if err := intctrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv, false); err != nil {
		return err
	}

	var clusterCompDef *appsv1alpha1.ClusterComponentDefinition
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	clusterCompSpec := cluster.Spec.GetComponentByName(synthesizeComp.Name)
	if clusterCompSpec == nil {
		return errors.New(fmt.Sprintf("component spec %s not found", synthesizeComp.Name))
	}
	clusterCompDef = cd.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return fmt.Errorf("referenced component definition does not exist, cluster: %s, component: %s, component definition ref:%s",
			cluster.Name, clusterCompSpec.Name, clusterCompSpec.ComponentDefRef)
	}
	if cv != nil {
		clusterCompVer = cv.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
	}

	buildWorkload := func() {
		synthesizeComp.WorkloadType = clusterCompDef.WorkloadType
		synthesizeComp.CharacterType = clusterCompDef.CharacterType
		synthesizeComp.HorizontalScalePolicy = clusterCompDef.HorizontalScalePolicy
		synthesizeComp.StatelessSpec = clusterCompDef.StatelessSpec
		synthesizeComp.StatefulSpec = clusterCompDef.StatefulSpec
		synthesizeComp.ConsensusSpec = clusterCompDef.ConsensusSpec
		synthesizeComp.ReplicationSpec = clusterCompDef.ReplicationSpec
		synthesizeComp.RSMSpec = clusterCompDef.RSMSpec
		synthesizeComp.StatefulSetWorkload = clusterCompDef.GetStatefulSetWorkload()
		synthesizeComp.Probes = clusterCompDef.Probes
		synthesizeComp.VolumeTypes = clusterCompDef.VolumeTypes
		synthesizeComp.VolumeProtection = clusterCompDef.VolumeProtectionSpec
		synthesizeComp.CustomLabelSpecs = clusterCompDef.CustomLabelSpecs
		synthesizeComp.SwitchoverSpec = clusterCompDef.SwitchoverSpec
		synthesizeComp.MinAvailable = clusterCompSpec.GetMinAvailable(clusterCompDef.GetMinAvailable())
	}

	// Services is a backward compatible field, which will be replaced with ComponentServices in the future.
	buildServices := func() {
		if clusterCompDef.Service != nil {
			service := corev1.Service{Spec: clusterCompDef.Service.ToSVCSpec()}
			service.Spec.Type = corev1.ServiceTypeClusterIP
			synthesizeComp.Services = append(synthesizeComp.Services, service)
			for _, item := range clusterCompSpec.Services {
				service = corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        item.Name,
						Annotations: item.Annotations,
					},
					Spec: service.Spec,
				}
				service.Spec.Type = item.ServiceType
				synthesizeComp.Services = append(synthesizeComp.Services, service)
			}
		}
	}

	mergeClusterCompVersion := func() {
		if clusterCompVer != nil {
			// only accept 1st ClusterVersion override context
			synthesizeComp.ConfigTemplates = cfgcore.MergeConfigTemplates(clusterCompVer.ConfigSpecs, synthesizeComp.ConfigTemplates)
			// override component.PodSpec.InitContainers and component.PodSpec.Containers
			for _, c := range clusterCompVer.VersionsCtx.InitContainers {
				synthesizeComp.PodSpec.InitContainers = appendOrOverrideContainerAttr(synthesizeComp.PodSpec.InitContainers, c)
			}
			for _, c := range clusterCompVer.VersionsCtx.Containers {
				synthesizeComp.PodSpec.Containers = appendOrOverrideContainerAttr(synthesizeComp.PodSpec.Containers, c)
			}
			// override component.SwitchoverSpec
			overrideSwitchoverSpecAttr(synthesizeComp.SwitchoverSpec, clusterCompVer.SwitchoverSpec)
		}
	}

	// build workload
	buildWorkload()

	// merge clusterCompVersion
	mergeClusterCompVersion()

	// build services
	buildServices()

	// build componentRefEnvs
	if err := buildComponentRef(cd, cluster, clusterCompDef, synthesizeComp); err != nil {
		reqCtx.Log.Error(err, "failed to merge componentRef")
		return err
	}

	return nil
}

// handleSimplifiedAPI handles simplified api for cluster.
// TODO(xingran): handleSimplifiedAPI should also support the new componentDefinition API.
func handleSimplifiedAPI(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) error {
	if clusterCompSpec != nil {
		return nil
	}

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

	fillSimplifiedAPI := func(cd *appsv1alpha1.ClusterDefinition, clusterCompDef *appsv1alpha1.ClusterComponentDefinition) {
		// fill simplified api only to first defined component
		if len(cd.Spec.ComponentDefs) == 0 ||
			cd.Spec.ComponentDefs[0].Name != clusterCompDef.Name {
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

	cd, _, err := getClusterDefAndVersion(reqCtx, cli, cluster)
	if err != nil {
		return err
	}

	clusterCompDef, _, err := getClusterCompDefAndVersion(reqCtx, cli, cluster, clusterCompSpec)
	if err != nil {
		return err
	}

	fillSimplifiedAPI(cd, clusterCompDef)

	return nil
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
		constant.KBConnCredentialPlaceHolder: constant.GenerateDefaultConnCredential(clusterName),
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
