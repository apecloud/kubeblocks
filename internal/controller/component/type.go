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
	v12 "k8s.io/api/core/v1"
	v1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type MonitorConfig struct {
	Enable     bool   `json:"enable"`
	ScrapePort int32  `json:"scrapePort,omitempty"`
	ScrapePath string `json:"scrapePath,omitempty"`
}

type SynthesizedComponent struct {
	ClusterDefName          string                              `json:"clusterDefName,omitempty"`
	Name                    string                              `json:"name,omitempty"`
	Type                    string                              `json:"type,omitempty"`
	CharacterType           string                              `json:"characterType,omitempty"`
	MaxUnavailable          *intstr.IntOrString                 `json:"maxUnavailable,omitempty"`
	Replicas                int32                               `json:"replicas"`
	PodDisruptionBudgetSpec *v1.PodDisruptionBudgetSpec         `json:"podDisruptionBudgetSpec,omitempty"`
	WorkloadType            v1alpha1.WorkloadType               `json:"workloadType,omitempty"`
	ConsensusSpec           *v1alpha1.ConsensusSetSpec          `json:"consensusSpec,omitempty"`
	PrimaryIndex            *int32                              `json:"primaryIndex,omitempty"`
	PodSpec                 *v12.PodSpec                        `json:"podSpec,omitempty"`
	Service                 *v12.ServiceSpec                    `json:"service,omitempty"`
	Probes                  *v1alpha1.ClusterDefinitionProbes   `json:"probes,omitempty"`
	VolumeClaimTemplates    []v12.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
	Monitor                 *MonitorConfig                      `json:"monitor,omitempty"`
	EnabledLogs             []string                            `json:"enabledLogs,omitempty"`
	LogConfigs              []v1alpha1.LogConfig                `json:"logConfigs,omitempty"`
	ConfigTemplates         []v1alpha1.ConfigTemplate           `json:"configTemplates,omitempty"`
	HorizontalScalePolicy   *v1alpha1.HorizontalScalePolicy     `json:"horizontalScalePolicy,omitempty"`
}
