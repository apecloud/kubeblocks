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

import troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

type ExtendCollect struct {
}

type ClusterAccess struct {
	// analyzeMeta is defined in troubleshoot.sh.
	troubleshoot.AnalyzeMeta `json:",inline"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ClusterRegion struct {
	// analyzeMeta is defined in troubleshoot.sh.
	troubleshoot.AnalyzeMeta `json:",inline"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// regionList is a list of expected regionName
	// +kubebuilder:validation:Required
	RegionList []string `json:"regionList"`
}

type ExtendAnalyze struct {
	// clusterAccess is to determine the accessibility of target K8S cluster.
	// +optional
	ClusterAccess *ClusterAccess `json:"clusterAccess,omitempty"`
	// ClusterRegion is to validate the regionName of target K8S cluster location.
	// +optional
	ClusterRegion *ClusterRegion `json:"clusterRegion,omitempty"`
}

type HostUtility struct {
	// hostCollectorMeta is defined in troubleshoot.sh.
	troubleshoot.HostCollectorMeta `json:",inline"`
	// utilityName will be checked in local host.
	// +kubebuilder:validation:Required
	UtilityName string `json:"utilityName"`
}

type ExtendHostCollect struct {
	// hostUtility is to collect the info of target utility.
	// +optional
	HostUtility *HostUtility `json:"hostUtility,omitempty"`
}

type HostUtilityAnalyze struct {
	// hostCollectorMeta is defined in troubleshoot.sh.
	troubleshoot.AnalyzeMeta `json:",inline"`
	// collectorName indicates which collect data will be analyzed
	// +optional
	CollectorName string `json:"collectorName,omitempty"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ExtendHostAnalyze struct {
	// hostUtility is to analyze the presence of target utility.
	// +optional
	HostUtility *HostUtilityAnalyze `json:"hostUtility,omitempty"`
}
