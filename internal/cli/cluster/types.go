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

package cluster

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type ClusterObjects struct {
	Cluster        *appsv1alpha1.Cluster
	ClusterDef     *appsv1alpha1.ClusterDefinition
	ClusterVersion *appsv1alpha1.ClusterVersion

	Pods       *corev1.PodList
	Services   *corev1.ServiceList
	Secrets    *corev1.SecretList
	PVCs       *corev1.PersistentVolumeClaimList
	Nodes      []*corev1.Node
	ConfigMaps *corev1.ConfigMapList
	Events     *corev1.EventList

	BackupPolicies []dpv1alpha1.BackupPolicy
	Backups        []dpv1alpha1.Backup
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
	CreatedTime       string `json:"age,omitempty"`
	Labels            string `json:"labels,omitempty"`
}

type ComponentInfo struct {
	Name      string `json:"name,omitempty"`
	NameSpace string `json:"nameSpace,omitempty"`
	Type      string `json:"type,omitempty"`
	Cluster   string `json:"cluster,omitempty"`
	Status    string `json:"status,omitempty"`
	Replicas  string `json:"replicas,omitempty"`
	CPU       string `json:"cpu,omitempty"`
	Memory    string `json:"memory,omitempty"`
	Image     string `json:"image,omitempty"`
	Storage   []StorageInfo
}

type StorageInfo struct {
	Name         string
	Size         string
	StorageClass string
	AccessMode   string
}

type InstanceInfo struct {
	Name        string `json:"name,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Cluster     string `json:"cluster,omitempty"`
	Component   string `json:"component,omitempty"`
	Status      string `json:"status,omitempty"`
	Role        string `json:"role,omitempty"`
	AccessMode  string `json:"accessMode,omitempty"`
	AZ          string `json:"az,omitempty"`
	Region      string `json:"region,omitempty"`
	CPU         string `json:"cpu,omitempty"`
	Memory      string `json:"memory,omitempty"`
	Storage     []StorageInfo
	Node        string `json:"node,omitempty"`
	CreatedTime string `json:"age,omitempty"`
}
