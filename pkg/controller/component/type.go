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

package component

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type SynthesizedComponent struct {
	Namespace                        string            `json:"namespace,omitempty"`
	ClusterName                      string            `json:"clusterName,omitempty"`
	ClusterUID                       string            `json:"clusterUID,omitempty"`
	Comp2CompDefs                    map[string]string `json:"comp2CompDefs,omitempty"` // {compName: compDefName}
	Name                             string            `json:"name,omitempty"`          // the name of the component w/o clusterName prefix
	FullCompName                     string            `json:"fullCompName,omitempty"`  // the full name of the component w/ clusterName prefix
	Generation                       string
	CompDefName                      string `json:"compDefName,omitempty"` // the name of the componentDefinition
	ServiceKind                      string
	ServiceVersion                   string                                 `json:"serviceVersion,omitempty"`
	Replicas                         int32                                  `json:"replicas"`
	Resources                        corev1.ResourceRequirements            `json:"resources,omitempty"`
	PodSpec                          *corev1.PodSpec                        `json:"podSpec,omitempty"`
	VolumeClaimTemplates             []corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
	LogConfigs                       []kbappsv1.LogConfig                   `json:"logConfigs,omitempty"`
	ConfigTemplates                  []kbappsv1.ComponentConfigSpec         `json:"configTemplates,omitempty"`
	ScriptTemplates                  []kbappsv1.ComponentTemplateSpec       `json:"scriptTemplates,omitempty"`
	TLSConfig                        *kbappsv1.TLSConfig                    `json:"tlsConfig"`
	ServiceAccountName               string                                 `json:"serviceAccountName,omitempty"`
	ServiceReferences                map[string]*kbappsv1.ServiceDescriptor `json:"serviceReferences,omitempty"`
	Labels                           map[string]string                      `json:"labels,omitempty"`
	Annotations                      map[string]string                      `json:"annotations,omitempty"`
	DynamicLabels                    map[string]string                      // labels defined by the cluster and component API
	DynamicAnnotations               map[string]string                      // annotations defined by the cluster and component API
	StaticAnnotations                map[string]string                      // annotations defined by the component definition
	TemplateVars                     map[string]any                         `json:"templateVars,omitempty"`
	EnvVars                          []corev1.EnvVar                        `json:"envVars,omitempty"`
	EnvFromSources                   []corev1.EnvFromSource                 `json:"envFromSources,omitempty"`
	Instances                        []kbappsv1.InstanceTemplate            `json:"instances,omitempty"`
	OfflineInstances                 []string                               `json:"offlineInstances,omitempty"`
	Roles                            []kbappsv1.ReplicaRole                 `json:"roles,omitempty"`
	UpdateStrategy                   *kbappsv1.UpdateStrategy               `json:"updateStrategy,omitempty"`
	PodManagementPolicy              *appsv1.PodManagementPolicyType        `json:"podManagementPolicy,omitempty"`
	ParallelPodManagementConcurrency *intstr.IntOrString                    `json:"parallelPodManagementConcurrency,omitempty"`
	PodUpdatePolicy                  *kbappsv1.PodUpdatePolicyType          `json:"podUpdatePolicy,omitempty"`
	PolicyRules                      []rbacv1.PolicyRule                    `json:"policyRules,omitempty"`
	LifecycleActions                 *kbappsv1.ComponentLifecycleActions    `json:"lifecycleActions,omitempty"`
	SystemAccounts                   []kbappsv1.SystemAccount               `json:"systemAccounts,omitempty"`
	Volumes                          []kbappsv1.ComponentVolume             `json:"volumes,omitempty"`
	HostNetwork                      *kbappsv1.HostNetwork                  `json:"hostNetwork,omitempty"`
	ComponentServices                []kbappsv1.ComponentService            `json:"componentServices,omitempty"`
	MinReadySeconds                  int32                                  `json:"minReadySeconds,omitempty"`
	DisableExporter                  *bool                                  `json:"disableExporter,omitempty"`
	Stop                             *bool

	// TODO(xingran): The following fields will be deprecated after KubeBlocks version 0.8.0
	ClusterDefName                      string `json:"clusterDefName,omitempty"` // the name of the clusterDefinition
	HorizontalScaleBackupPolicyTemplate *string
}
