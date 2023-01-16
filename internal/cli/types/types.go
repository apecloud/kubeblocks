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

package types

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// CliDefaultHome defines kbcli default home name
	CliDefaultHome = ".kbcli"
	// CliHomeEnv defines kbcli home system env
	CliHomeEnv = "KBCLI_HOME"

	// GoosLinux is os.GOOS linux string
	GoosLinux = "linux"
	// GoosDarwin is os.GOOS darwin string
	GoosDarwin = "darwin"
	// GoosWindows is os.GOOS windows string
	GoosWindows = "windows"

	// Group api group
	Group = "dbaas.kubeblocks.io"

	// AppsGroup k8s apps group
	AppsGroup = "apps"

	// Version api version
	Version = "v1alpha1"

	VersionV1 = "v1"

	// ResourceClusters clusters resource
	ResourceClusters = "clusters"
	// ResourceClusterDefs clusterDefinition resource
	ResourceClusterDefs = "clusterdefinitions"
	// ResourceClusterVersions clusterVersion resource
	ResourceClusterVersions = "clusterversions"
	// ResourceOpsRequests opsrequests resource
	ResourceOpsRequests = "opsrequests"
	// ResourceDeployments deployment resource
	ResourceDeployments = "deployments"

	// KindCluster cluster king
	KindCluster = "Cluster"
	// KindClusterDef clusterDefinition kine
	KindClusterDef = "ClusterDefinition"
	// KindClusterVersion clusterVersion kind
	KindClusterVersion = "ClusterVersion"

	NameLabelKey                   = "app.kubernetes.io/name"
	InstanceLabelKey               = "app.kubernetes.io/instance"
	ConsensusSetRoleLabelKey       = "cs.dbaas.kubeblocks.io/role"
	ConsensusSetAccessModeLabelKey = "cs.dbaas.kubeblocks.io/access-mode"
	ComponentLabelKey              = "app.kubernetes.io/component-name"
	RegionLabelKey                 = "topology.kubernetes.io/region"
	ZoneLabelKey                   = "topology.kubernetes.io/zone"
	ClusterDefLabelKey             = "clusterdefinition.kubeblocks.io/name"

	ServiceLBTypeAnnotationKey     = "service.kubernetes.io/apecloud-loadbalancer-type"
	ServiceLBTypeAnnotationValue   = "private-ip"
	ServiceFloatingIPAnnotationKey = "service.kubernetes.io/apecloud-loadbalancer-floating-ip"
	StorageClassAnnotationKey      = "kubeblocks.io/storage-class"

	// DataProtection definitions
	DPGroup                = "dataprotection.kubeblocks.io"
	DPVersion              = "v1alpha1"
	ResourceBackups        = "backups"
	ResourceRestoreJobs    = "restorejobs"
	ResourceBackupPolicies = "backuppolicies"

	None = "<none>"
)

var (
	// KubeBlocksChartName helm name for installing kubeblocks
	KubeBlocksChartName = "kubeblocks"

	// KubeBlocksChartURL the helm chart for installing kubeblocks
	KubeBlocksChartURL = "https://apecloud.github.io/helm-charts"
)

type BackupJobInfo struct {
	Name           string
	Namespace      string
	Phase          string
	StartTime      string
	CompletionTime string
	Labels         string
}

type BackupSnapInfo struct {
	Name          string
	Namespace     string
	ReadyToUse    bool
	CreationTime  string
	RestoreSize   string
	SourcePVC     string
	SnapshotClass string
	Labels        string
}

func ClusterGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceClusters}
}

func ClusterGK() schema.GroupKind {
	return schema.GroupKind{Group: Group, Kind: KindCluster}
}

func ClusterDefGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceClusterDefs}
}

func ClusterDefGK() schema.GroupKind {
	return schema.GroupKind{Group: Group, Kind: KindClusterDef}
}

func ClusterVersionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceClusterVersions}
}

func ClusterVersionGK() schema.GroupKind {
	return schema.GroupKind{Group: Group, Kind: KindClusterVersion}
}

func BackupGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceBackups}
}

func RestoreJobGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceRestoreJobs}
}

func OpsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceOpsRequests}
}

func CRDGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  VersionV1,
		Resource: "customresourcedefinitions",
	}
}
