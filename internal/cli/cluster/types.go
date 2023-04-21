/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
