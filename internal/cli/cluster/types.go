/*
Copyright ApeCloud Inc.

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

package cluster

import (
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type ClusterObjects struct {
	Cluster        *dbaasv1alpha1.Cluster
	ClusterDef     *dbaasv1alpha1.ClusterDefinition
	ClusterVersion *dbaasv1alpha1.ClusterVersion

	Pods       *corev1.PodList
	Services   *corev1.ServiceList
	Secrets    *corev1.SecretList
	PVCs       *corev1.PersistentVolumeClaimList
	Nodes      []*corev1.Node
	ConfigMaps *corev1.ConfigMapList
}

type ClusterInfo struct {
	Name              string `json:"name,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
	ClusterVersion    string `json:"clusterVersion,omitempty"`
	TerminationPolicy string `json:"terminationPolicy,omitempty"`
	ClusterDefinition string `json:"clusterDefinition,omitempty"`
	Status            string `json:"status,omitempty"`
	InternalEP        string `json:"internalEP,omitempty"`
	ExternalEP        string `json:"externalEP,omitempty"`
	Age               string `json:"age,omitempty"`
}

type ComponentInfo struct {
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Cluster  string `json:"cluster,omitempty"`
	Status   string `json:"status,omitempty"`
	Replicas string `json:"replicas,omitempty"`
	Image    string `json:"image,omitempty"`
}

type InstanceInfo struct {
	Name       string `json:"name,omitempty"`
	Cluster    string `json:"cluster,omitempty"`
	Component  string `json:"component,omitempty"`
	Status     string `json:"status,omitempty"`
	Role       string `json:"role,omitempty"`
	AccessMode string `json:"accessMode,omitempty"`
	AZ         string `json:"AZ,omitempty"`
	Region     string `json:"Region,omitempty"`
	CPU        string `json:"CPU,omitempty"`
	Memory     string `json:"memory,omitempty"`
	Storage    string `json:"storage,omitempty"`
	Node       string `json:"node,omitempty"`
	Age        string `json:"age,omitempty"`
}
