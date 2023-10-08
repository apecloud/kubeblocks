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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

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
	synthesizeComp.PodSpec.TopologySpreadConstraints = buildPodTopologySpreadConstraints(cluster, affinity, synthesizeComp)
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

	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err := cli.Get(reqCtx.Ctx, key, object)
		if err != nil {
			return err
		}
		return nil
	}

	cd := &appsv1alpha1.ClusterDefinition{}
	if err := validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if err := validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv); err != nil {
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
