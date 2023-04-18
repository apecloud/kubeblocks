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

package component

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type MonitorConfig struct {
	Enable     bool   `json:"enable"`
	ScrapePort int32  `json:"scrapePort,omitempty"`
	ScrapePath string `json:"scrapePath,omitempty"`
}

type SynthesizedComponent struct {
	ClusterDefName        string                                 `json:"clusterDefName,omitempty"`
	ClusterName           string                                 `json:"clusterName,omitempty"`
	Name                  string                                 `json:"name,omitempty"`
	Type                  string                                 `json:"type,omitempty"`
	CharacterType         string                                 `json:"characterType,omitempty"`
	MaxUnavailable        *intstr.IntOrString                    `json:"maxUnavailable,omitempty"`
	Replicas              int32                                  `json:"replicas"`
	WorkloadType          v1alpha1.WorkloadType                  `json:"workloadType,omitempty"`
	ConsensusSpec         *v1alpha1.ConsensusSetSpec             `json:"consensusSpec,omitempty"`
	PrimaryIndex          *int32                                 `json:"primaryIndex,omitempty"`
	PodSpec               *corev1.PodSpec                        `json:"podSpec,omitempty"`
	Services              []corev1.Service                       `json:"services,omitempty"`
	Probes                *v1alpha1.ClusterDefinitionProbes      `json:"probes,omitempty"`
	VolumeClaimTemplates  []corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
	Monitor               *MonitorConfig                         `json:"monitor,omitempty"`
	EnabledLogs           []string                               `json:"enabledLogs,omitempty"`
	LogConfigs            []v1alpha1.LogConfig                   `json:"logConfigs,omitempty"`
	ConfigTemplates       []v1alpha1.ComponentConfigSpec         `json:"configTemplates,omitempty"`
	ScriptTemplates       []v1alpha1.ComponentTemplateSpec       `json:"scriptTemplates,omitempty"`
	HorizontalScalePolicy *v1alpha1.HorizontalScalePolicy        `json:"horizontalScalePolicy,omitempty"`
	TLS                   bool                                   `json:"tls"`
	Issuer                *v1alpha1.Issuer                       `json:"issuer,omitempty"`
	VolumeTypes           []v1alpha1.VolumeTypeSpec              `json:"VolumeTypes,omitempty"`
	CustomLabelSpecs      []v1alpha1.CustomLabelSpec             `json:"customLabelSpecs,omitempty"`
	ComponentDef          string                                 `json:"componentDef,omitempty"`
}

// GetPrimaryIndex provides PrimaryIndex value getter, if PrimaryIndex is
// a nil pointer it's treated at 0, return -1 if function receiver is nil.
func (r *SynthesizedComponent) GetPrimaryIndex() int32 {
	if r == nil {
		return -1
	}
	if r.PrimaryIndex == nil {
		return 0
	}
	return *r.PrimaryIndex
}
