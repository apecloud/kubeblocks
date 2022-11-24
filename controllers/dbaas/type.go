/*
Copyright ApeCloud Inc.

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
	policyv1 "k8s.io/api/policy/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	// settings keys
	cmNamespaceKey              = "CM_NAMESPACE"
	maxConcurReconAppVersionKey = "MAXCONCURRENTRECONCILES_APPVERSION"
	maxConcurReconClusterDefKey = "MAXCONCURRENTRECONCILES_CLUSTERDEF"

	// name of our custom finalizer
	dbClusterFinalizerName    = "cluster.kubeblocks.io/finalizer"
	dbClusterDefFinalizerName = "clusterdefinition.kubeblocks.io/finalizer"
	appVersionFinalizerName   = "appversion.kubeblocks.io/finalizer"
	opsRequestFinalizerName   = "opsrequest.kubeblocks.io/finalizer"

	// label keys
	clusterDefLabelKey         = "clusterdefinition.kubeblocks.io/name"
	appVersionLabelKey         = "appversion.kubeblocks.io/name"
	statefulSetPodNameLabelKey = "statefulset.kubernetes.io/pod-name"

	DeploymentKind  = "Deployment"
	StatefulSetKind = "StatefulSet"
	PodKind         = "Pod"
)

type MonitorConfig struct {
	Enable     bool   `json:"enable"`
	ScrapePort int32  `json:"scrapePort,omitempty"`
	ScrapePath string `json:"scrapePath,omitempty"`
}

type Component struct {
	ClusterDefName          string                                 `json:"clusterDefName,omitempty"`
	ClusterType             string                                 `json:"clusterType,omitempty"`
	Name                    string                                 `json:"name,omitempty"`
	Type                    string                                 `json:"type,omitempty"`
	MinReplicas             int32                                  `json:"minReplicas,omitempty"`
	MaxReplicas             int32                                  `json:"maxReplicas,omitempty"`
	DefaultReplicas         int32                                  `json:"defaultReplicas,omitempty"`
	Replicas                int32                                  `json:"replicas,omitempty"`
	PodDisruptionBudgetSpec *policyv1.PodDisruptionBudgetSpec      `json:"podDisruptionBudgetSpec,omitempty"`
	AntiAffinity            bool                                   `json:"antiAffinity,omitempty"`
	ComponentType           dbaasv1alpha1.ComponentType            `json:"componentType,omitempty"`
	ConsensusSpec           *dbaasv1alpha1.ConsensusSetSpec        `json:"consensusSpec,omitempty"`
	PodSpec                 *corev1.PodSpec                        `json:"podSpec,omitempty"`
	Service                 *corev1.ServiceSpec                    `json:"service,omitempty"`
	Probes                  *dbaasv1alpha1.ClusterDefinitionProbes `json:"probes,omitempty"`
	VolumeClaimTemplates    []corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates,omitempty"`
	Monitor                 *MonitorConfig                         `json:"monitor,omitempty"`
	EnabledLogs             []string                               `json:"enabledLogs,omitempty"`
	LogConfigs              []dbaasv1alpha1.LogConfig              `json:"logConfigs,omitempty"`
}
