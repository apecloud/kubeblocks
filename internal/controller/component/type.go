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
	Name                  string                                 `json:"name,omitempty"`
	Type                  string                                 `json:"type,omitempty"`
	CharacterType         string                                 `json:"characterType,omitempty"`
	MaxUnavailable        *intstr.IntOrString                    `json:"maxUnavailable,omitempty"`
	Replicas              int32                                  `json:"replicas"`
	WorkloadType          v1alpha1.WorkloadType                  `json:"workloadType,omitempty"`
	ConsensusSpec         *v1alpha1.ConsensusSetSpec             `json:"consensusSpec,omitempty"`
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
	ServiceAccountName    string                                 `json:"serviceAccountName,omitempty"`
	CandidateInstance     *v1alpha1.CandidateInstance            `json:"candidateInstance,omitempty"`
}
