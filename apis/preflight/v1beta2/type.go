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

package v1beta2

import troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

type ExtendCollect struct {
}

type ClusterAccess struct {
	troubleshootv1beta2.AnalyzeMeta `json:",inline" yaml:",inline"`
	// +kubebuilder:validation:Required
	Outcomes []*troubleshootv1beta2.Outcome `json:"outcomes" yaml:"outcomes"`
}

type ExtendAnalyze struct {
	ClusterAccess *ClusterAccess `json:"clusterAccess,omitempty" yaml:"clusterAccess,omitempty"`
}

// 这上面要不要添加kubebuilder的校验

type HostUtility struct {
	troubleshootv1beta2.HostCollectorMeta `json:",inline" yaml:",inline"`
	// +kubebuilder:validation:Required
	UtilityName string `json:"utilityName" yaml:"utilityName"`
}

type ExtendHostCollect struct {
	// +optional
	HostUtility *HostUtility `json:"hostUtility,omitempty" yaml:"hostUtility,omitempty"`
}

type HostUtilityAnalyze struct {
	troubleshootv1beta2.AnalyzeMeta `json:",inline" yaml:",inline"`
	// +optional
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +kubebuilder:validation:Required
	Outcomes []*troubleshootv1beta2.Outcome `json:"outcomes" yaml:"outcomes"`
}

type ExtendHostAnalyze struct {
	// +optional
	HostUtility *HostUtilityAnalyze `json:"hostUtility,omitempty" yaml:"hostUtility,omitempty"`
}
