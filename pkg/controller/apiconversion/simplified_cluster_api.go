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

package apiconversion

import (
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// TODO(xingran): HandleSimplifiedClusterAPI should also support the new component definition API.

// HandleSimplifiedClusterAPI handles simplified api for cluster.
func HandleSimplifiedClusterAPI(clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster) *appsv1alpha1.ClusterComponentSpec {
	if hasClusterCompDefined(cluster) {
		return nil
	}
	if !hasSimplifiedClusterAPI(cluster) {
		return nil
	}
	if len(clusterDef.Spec.ComponentDefs) == 0 {
		return nil
	}
	// fill simplified api only to first defined component
	return fillSimplifiedClusterAPI(cluster, &clusterDef.Spec.ComponentDefs[0])
}

func hasClusterCompDefined(cluster *appsv1alpha1.Cluster) bool {
	return cluster.Spec.ComponentSpecs != nil && len(cluster.Spec.ComponentSpecs) > 0
}

func hasSimplifiedClusterAPI(cluster *appsv1alpha1.Cluster) bool {
	return cluster.Spec.Replicas != nil ||
		!cluster.Spec.Resources.CPU.IsZero() ||
		!cluster.Spec.Resources.Memory.IsZero() ||
		!cluster.Spec.Storage.Size.IsZero() ||
		cluster.Spec.Monitor.MonitoringInterval != nil ||
		cluster.Spec.Network != nil ||
		len(cluster.Spec.Tenancy) > 0 ||
		len(cluster.Spec.AvailabilityPolicy) > 0
}

func fillSimplifiedClusterAPI(cluster *appsv1alpha1.Cluster, clusterCompDef *appsv1alpha1.ClusterComponentDefinition) *appsv1alpha1.ClusterComponentSpec {
	clusterCompSpec := &appsv1alpha1.ClusterComponentSpec{
		Name:            clusterCompDef.Name,
		ComponentDefRef: clusterCompDef.Name,
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
			case cloudProviderAWS:
				svc.Annotations = map[string]string{
					"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
					"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
				}
			case cloudProviderGCP:
				svc.Annotations = map[string]string{
					"networking.gke.io/load-balancer-type": "Internal",
				}
			case cloudProviderAliyun:
				svc.Annotations = map[string]string{
					"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "intranet",
				}
			case cloudProviderAzure:
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
			case cloudProviderAWS:
				svc.Annotations = map[string]string{
					"service.beta.kubernetes.io/aws-load-balancer-type":     "nlb",
					"service.beta.kubernetes.io/aws-load-balancer-internal": "false",
				}
			case cloudProviderAliyun:
				svc.Annotations = map[string]string{
					"service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type": "internet",
				}
			case cloudProviderAzure:
				svc.Annotations = map[string]string{
					"service.beta.kubernetes.io/azure-load-balancer-internal": "false",
				}
			}
			clusterCompSpec.Services = append(clusterCompSpec.Services, svc)
		}
	}
	return clusterCompSpec
}

func getCloudProvider() cloudProvider {
	k8sVersion := viper.GetString(constant.CfgKeyServerInfo)
	if strings.Contains(k8sVersion, "eks") {
		return cloudProviderAWS
	}
	if strings.Contains(k8sVersion, "gke") {
		return cloudProviderGCP
	}
	if strings.Contains(k8sVersion, "aliyun") {
		return cloudProviderAliyun
	}
	if strings.Contains(k8sVersion, "tke") {
		return cloudProviderTencent
	}
	return cloudProviderUnknown
}
