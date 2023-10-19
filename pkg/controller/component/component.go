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
	"github.com/apecloud/kubeblocks/pkg/class"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// BuildProtoComponent builds a new Component object from cluster componentSpec.
func BuildProtoComponent(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {

	// check if clusterCompSpec enable the ComponentDefinition API feature gate.
	if clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDef != "" {
		return buildProtoCompFromCompDef(reqCtx, cli, cluster, clusterCompSpec)
	} else if !clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDefRef != "" {
		if cluster.Spec.ClusterDefRef == "" {
			return nil, errors.New("clusterDefRef is required when enableComponentDefinition is false")
		}
		return buildProtoCompFromConvertor(reqCtx, cli, cluster, clusterCompSpec)
	} else {
		return nil, errors.New("invalid component spec")
	}
}

// buildProtoCompFromCompDef builds a new Component object based on ComponentDefinition API.
func buildProtoCompFromCompDef(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	return nil, nil
}

// buildProtoCompFromConvertor builds a new Component object based on converting clusterComponentDefinition to ComponentDefinition.
func buildProtoCompFromConvertor(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {

	cd := &appsv1alpha1.ClusterDefinition{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return nil, err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv); err != nil {
		return nil, err
	}
	var clusterCompDef *appsv1alpha1.ClusterComponentDefinition
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	clusterCompDef = cd.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return nil, fmt.Errorf("referenced component definition does not exist, cluster: %s, component: %s, component definition ref:%s", cluster.Name, clusterCompSpec.Name, clusterCompSpec.ComponentDefRef)
	}
	if cv != nil {
		clusterCompVer = cv.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
	}
	return BuildComponentFrom(clusterCompDef, clusterCompVer, clusterCompSpec)
}

// BuildComponent builds SynthesizedComponent object
// TODO(xingran): delete it and use BuildSynthesizedComponent instead
func BuildComponent(reqCtx ictrlutil.RequestCtx,
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
// TODO(xingran): delete it and use BuildSynthesizedComponent instead
func buildComponent(reqCtx ictrlutil.RequestCtx,
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
		ClusterCompDefName:    clusterCompSpec.ComponentDefRef,
		ServiceAccountName:    clusterCompSpec.ServiceAccountName,
	}

	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if len(clusterCompVers) > 0 && clusterCompVers[0] != nil {
		// only accept 1st ClusterVersion override context
		clusterCompVer = clusterCompVers[0]
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
	affinity := BuildAffinity(cluster, clusterCompSpec.Affinity)
	if component.PodSpec.Affinity, err = BuildPodAffinity(cluster, affinity, component); err != nil {
		reqCtx.Log.Error(err, "build pod affinity failed.")
		return nil, err
	}
	component.PodSpec.TopologySpreadConstraints = BuildPodTopologySpreadConstraints(cluster, affinity, component)
	if component.PodSpec.Tolerations, err = BuildTolerations(cluster, clusterCompSpec.Tolerations); err != nil {
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

	buildMonitorConfig(clusterCompDef.Monitor, clusterCompSpec.Monitor, clusterCompDef.PodSpec, component)

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

	if err = buildComponentRef(clusterDef, cluster, clusterCompDefObj, component); err != nil {
		reqCtx.Log.Error(err, "failed to merge componentRef")
		return nil, err
	}

	if serviceReferences != nil {
		component.ServiceReferences = serviceReferences
	}

	return component, nil
}
