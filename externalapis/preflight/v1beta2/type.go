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

// ExtendCollect defines extended data collector for k8s cluster
type ExtendCollect struct {
}

// ClusterAccessAnalyze analyzes the accessibility of target
type ClusterAccessAnalyze struct {
	// analyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ExtendAnalyze struct {
	// clusterAccess is to determine the accessibility of target k8s cluster
	// +optional
	ClusterAccess *ClusterAccessAnalyze `json:"clusterAccess,omitempty"`
	// StorageClass is to determine the correctness of target storage class
	// +optional
	StorageClass *KbStorageClassAnalyze `json:"storageClass,omitempty"`
}

type HostUtility struct {
	// hostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.HostCollectorMeta `json:",inline"`
	// utilityName indicates the utility which will be checked in local host
	// +kubebuilder:validation:Required
	UtilityName string `json:"utilityName"`
}

type ClusterRegion struct {
	// hostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.HostCollectorMeta `json:",inline"`
	// providerName denotes which cloud provider target k8s located on
	// +kubebuilder:validation:Required
	ProviderName string `json:"providerName"`
}

type ExtendHostCollect struct {
	// hostUtility is to collect the data of target utility.
	// +optional
	HostUtility *HostUtility `json:"hostUtility,omitempty"`
	// ClusterRegion is to collect the data of target k8s
	// +optional
	ClusterRegion *ClusterRegion `json:"clusterRegion,omitempty"`
}

type HostUtilityAnalyze struct {
	// hostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// collectorName indicates which collect data will be analyzed
	// +optional
	CollectorName string `json:"collectorName,omitempty"`
	// outcomes are expected user defined results
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ClusterRegionAnalyze struct {
	// analyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// regionNames is a set of expected region names
	// +kubebuilder:validation:Required
	RegionNames []string `json:"regionNames"`
}

// KbStorageClassAnalyze overcome default storageClassAnalyze in preflight
type KbStorageClassAnalyze struct {
	// analyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// Parameters is a set of parameters including type and fsType...
	// +kubebuilder:validation:Required
	StorageClassType string `json:"storageClassType"`
	// provisioner is the provisioner of storageClass
	// +optional
	Provisioner string `json:"provisioner,omitempty"`
}

type ExtendHostAnalyze struct {
	// hostUtility is to analyze the presence of target utility.
	// +optional
	HostUtility *HostUtilityAnalyze `json:"hostUtility,omitempty"`
	// ClusterRegion is to validate the regionName of target k8s cluster
	// +optional
	ClusterRegion *ClusterRegionAnalyze `json:"clusterRegion,omitempty"`
}
