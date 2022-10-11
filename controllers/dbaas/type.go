/*
Copyright 2022 The Kubeblocks Authors

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

package dbaas

import (
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	// name of our custom finalizer
	dbClusterFinalizerName    = "cluster.infracreate.com/finalizer"
	dbClusterDefFinalizerName = "clusterdefinition.infracreate.com/finalizer"
	appVersionFinalizerName   = "appversion.infracreate.com/finalizer"
	opsRequestFinalizerName   = "opsrequest.infracreate.com/finalizer"

	// label keys
	clusterDefLabelKey         = "clusterdefinition.infracreate.com/name"
	appVersionLabelKey         = "appversion.infracreate.com/name"
	appInstanceLabelKey        = "app.kubernetes.io/instance"
	appComponentLabelKey       = "app.kubernetes.io/component-name"
	appNameLabelKey            = "app.kubernetes.io/name"
	statefulSetPodNameLabelKey = "statefulset.kubernetes.io/pod-name"
	consensusSetRoleLabelKey   = "cs.dbaas.infracreate.com/role"
	clusterLabelKey            = "cluster.infracreate.com/name"
)

type Component struct {
	ClusterDefName       string                                 `json:"clusterDefName,omitempty"`
	ClusterType          string                                 `json:"clusterType,omitempty"`
	Name                 string                                 `json:"name,omitempty"`
	Type                 string                                 `json:"type,omitempty"`
	MinAvailable         int                                    `json:"minAvailable,omitempty"`
	MaxAvailable         int                                    `json:"maxAvailable,omitempty"`
	DefaultReplicas      int                                    `json:"defaultReplicas,omitempty"`
	Replicas             int                                    `json:"replicas,omitempty"`
	AntiAffinity         bool                                   `json:"antiAffinity,omitempty"`
	ComponentType        dbaasv1alpha1.ComponentType            `json:"componentType,omitempty"`
	ConsensusSpec        dbaasv1alpha1.ConsensusSetSpec         `json:"consensusSpec,omitempty"`
	PodSpec              *corev1.PodSpec                        `json:"podSpec,omitempty"`
	Service              corev1.ServiceSpec                     `json:"service,omitempty"`
	ReadonlyService      corev1.ServiceSpec                     `json:"readonlyService,omitempty"`
	Scripts              dbaasv1alpha1.ClusterDefinitionScripts `json:"scripts,omitempty"`
	VolumeClaimTemplates []corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
}
