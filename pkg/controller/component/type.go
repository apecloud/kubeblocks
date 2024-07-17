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

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type SynthesizedComponent struct {
	Namespace            string                                 `json:"namespace,omitempty"`
	ClusterName          string                                 `json:"clusterName,omitempty"`
	ClusterUID           string                                 `json:"clusterUID,omitempty"`
	ClusterGeneration    string                                 `json:"clusterGeneration,omitempty"`
	Comp2CompDefs        map[string]string                      `json:"comp2CompDefs,omitempty"` // {compName: compDefName}
	Name                 string                                 `json:"name,omitempty"`          // the name of the component w/o clusterName prefix
	FullCompName         string                                 `json:"fullCompName,omitempty"`  // the full name of the component w/ clusterName prefix
	CompDefName          string                                 `json:"compDefName,omitempty"`   // the name of the componentDefinition
	ServiceVersion       string                                 `json:"serviceVersion,omitempty"`
	Replicas             int32                                  `json:"replicas"`
	Resources            corev1.ResourceRequirements            `json:"resources,omitempty"`
	PodSpec              *corev1.PodSpec                        `json:"podSpec,omitempty"`
	VolumeClaimTemplates []corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
	LogConfigs           []v1alpha1.LogConfig                   `json:"logConfigs,omitempty"`
	ConfigTemplates      []v1alpha1.ComponentConfigSpec         `json:"configTemplates,omitempty"`
	ScriptTemplates      []v1alpha1.ComponentTemplateSpec       `json:"scriptTemplates,omitempty"`
	TLSConfig            *v1alpha1.TLSConfig                    `json:"tlsConfig"`
	ServiceAccountName   string                                 `json:"serviceAccountName,omitempty"`
	// TODO: remove this later
	ComponentRefEnvs       []corev1.EnvVar                        `json:"componentRefEnvs,omitempty"`
	ServiceReferences      map[string]*v1alpha1.ServiceDescriptor `json:"serviceReferences,omitempty"`
	UserDefinedLabels      map[string]string
	UserDefinedAnnotations map[string]string
	TemplateVars           map[string]any                      `json:"templateVars,omitempty"`
	EnvVars                []corev1.EnvVar                     `json:"envVars,omitempty"`
	EnvFromSources         []corev1.EnvFromSource              `json:"envFromSources,omitempty"`
	Instances              []v1alpha1.InstanceTemplate         `json:"instances,omitempty"`
	OfflineInstances       []string                            `json:"offlineInstances,omitempty"`
	Roles                  []v1alpha1.ReplicaRole              `json:"roles,omitempty"`
	Labels                 map[string]string                   `json:"labels,omitempty"`
	Annotations            map[string]string                   `json:"annotations,omitempty"`
	UpdateStrategy         *v1alpha1.UpdateStrategy            `json:"updateStrategy,omitempty"`
	PodManagementPolicy    *appsv1.PodManagementPolicyType     `json:"podManagementPolicy,omitempty"`
	PodUpdatePolicy        *workloads.PodUpdatePolicyType      `json:"podUpdatePolicy,omitempty"`
	PolicyRules            []rbacv1.PolicyRule                 `json:"policyRules,omitempty"`
	LifecycleActions       *v1alpha1.ComponentLifecycleActions `json:"lifecycleActions,omitempty"`
	SystemAccounts         []v1alpha1.SystemAccount            `json:"systemAccounts,omitempty"`
	RoleArbitrator         *v1alpha1.RoleArbitrator            `json:"roleArbitrator,omitempty"`
	Volumes                []v1alpha1.ComponentVolume          `json:"volumes,omitempty"`
	HostNetwork            *v1alpha1.HostNetwork               `json:"hostNetwork,omitempty"`
	ComponentServices      []v1alpha1.ComponentService         `json:"componentServices,omitempty"`
	MinReadySeconds        int32                               `json:"minReadySeconds,omitempty"`
	DisableExporter        *bool                               `json:"disableExporter,omitempty"`
	Stop                   *bool

	// TODO(xingran): The following fields will be deprecated after version 0.8.0 and will be replaced with a new data structure.
	Probes           *v1alpha1.ClusterDefinitionProbes `json:"probes,omitempty"`           // The Probes will be replaced with LifecycleActions.RoleProbe in the future.
	VolumeTypes      []v1alpha1.VolumeTypeSpec         `json:"volumeTypes,omitempty"`      // The VolumeTypes will be replaced with Volumes in the future.
	VolumeProtection *v1alpha1.VolumeProtectionSpec    `json:"volumeProtection,omitempty"` // The VolumeProtection will be replaced with Volumes in the future.
	Services         []corev1.Service                  `json:"services,omitempty"`         // The Services will be replaced with ComponentServices in the future.
	TLS              bool                              `json:"tls"`                        // The TLS will be replaced with TLSConfig in the future.

	// TODO(xingran): The following fields will be deprecated after KubeBlocks version 0.8.0
	ClusterDefName        string                          `json:"clusterDefName,omitempty"`     // the name of the clusterDefinition
	ClusterCompDefName    string                          `json:"clusterCompDefName,omitempty"` // the name of the clusterDefinition.Spec.ComponentDefs[*].Name or cluster.Spec.ComponentSpecs[*].ComponentDefRef
	CharacterType         string                          `json:"characterType,omitempty"`
	WorkloadType          v1alpha1.WorkloadType           `json:"workloadType,omitempty"`
	HorizontalScalePolicy *v1alpha1.HorizontalScalePolicy `json:"horizontalScalePolicy,omitempty"`
	EnabledLogs           []string                        `json:"enabledLogs,omitempty"`
}
