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

package v1beta2

import (
	v1 "k8s.io/api/core/v1"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// ExtendCollect defines extended data collector for k8s cluster
type ExtendCollect struct {
}

// ClusterAccessAnalyze analyzes the accessibility of target
type ClusterAccessAnalyze struct {
	// AnalyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// Outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ExtendAnalyze struct {
	// ClusterAccess is to determine the accessibility of target k8s cluster
	// +optional
	ClusterAccess *ClusterAccessAnalyze `json:"clusterAccess,omitempty"`
	// StorageClass is to determine the correctness of target storage class
	// +optional
	StorageClass *KBStorageClassAnalyze `json:"storageClass,omitempty"`
	// Taint is to Determine the matching between the taint and toleration
	// +optional
	Taint *KBTaintAnalyze `json:"taint,omitempty"`
}

type HostUtility struct {
	// HostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.HostCollectorMeta `json:",inline"`
	// UtilityName indicates the utility which will be checked in local host
	// +kubebuilder:validation:Required
	UtilityName string `json:"utilityName"`
}

type ClusterRegion struct {
	// HostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.HostCollectorMeta `json:",inline"`
	// ProviderName denotes the cloud provider target k8s located on
	// +kubebuilder:validation:Required
	ProviderName string `json:"providerName"`
}

type ExtendHostCollect struct {
	// HostUtility is to collect the data of target utility.
	// +optional
	HostUtility *HostUtility `json:"hostUtility,omitempty"`
	// ClusterRegion is region of target k8s
	// +optional
	ClusterRegion *ClusterRegion `json:"clusterRegion,omitempty"`
}

type HostUtilityAnalyze struct {
	// HostCollectorMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// CollectorName indicates the collected data to be analyzed
	// +optional
	CollectorName string `json:"collectorName,omitempty"`
	// Outcomes are expected user defined results
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
}

type ClusterRegionAnalyze struct {
	// AnalyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// Outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// RegionNames is a set of expected region names
	// +kubebuilder:validation:Required
	RegionNames []string `json:"regionNames"`
}

// KBStorageClassAnalyze replaces default storageClassAnalyze in preflight
type KBStorageClassAnalyze struct {
	// AnalyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// Outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// StorageClassType is StorageClass type
	// +kubebuilder:validation:Required
	StorageClassType string `json:"storageClassType"`
	// Provisioner is the provisioner of StorageClass
	// +optional
	Provisioner string `json:"provisioner,omitempty"`
}

// KBTaintAnalyze matches the analysis of taints with TolerationsMap
type KBTaintAnalyze struct {
	// AnalyzeMeta is defined in troubleshoot.sh
	troubleshoot.AnalyzeMeta `json:",inline"`
	// Outcomes are expected user defined results.
	// +kubebuilder:validation:Required
	Outcomes []*troubleshoot.Outcome `json:"outcomes"`
	// Tolerations are toleration configuration passed by kbcli
	// +optional
	TolerationsMap map[string][]v1.Toleration `json:"tolerations"`
}

type ExtendHostAnalyze struct {
	// HostUtility is to analyze the presence of target utility
	// +optional
	HostUtility *HostUtilityAnalyze `json:"hostUtility,omitempty"`
	// ClusterRegion is to validate the regionName of target k8s cluster
	// +optional
	ClusterRegion *ClusterRegionAnalyze `json:"clusterRegion,omitempty"`
}
