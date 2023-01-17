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

package types

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	// ResourceConfigmaps configmap resource
	ResourceConfigmaps = "configmaps"
	// ResourceStatefulSets sts resource
	ResourceStatefulSets = "statefulsets"
	// ResourceConfigConstraintVersions clusterVersion resource
	ResourceConfigConstraintVersions = "configconstraints"

	// KindCluster cluster king
	KindCluster = "Cluster"
	// KindClusterDef clusterDefinition kine
	KindClusterDef = "ClusterDefinition"
	// KindClusterVersion clusterVersion kind
	KindClusterVersion       = "ClusterVersion"
	KindConfigConstraint     = "ConfigConstraint"
	KindBackup               = "Backup"
	KindRestoreJob           = "RestoreJob"
	KindBackupPolicyTemplate = "BackupPolicyTemplate"
	KindOps                  = "OpsRequest"
	KindCM                   = "ConfigMap"
	KindSTS                  = "StatefulSet"

	NameLabelKey                   = "app.kubernetes.io/name"
	InstanceLabelKey               = "app.kubernetes.io/instance"
	ConsensusSetAccessModeLabelKey = "cs.dbaas.kubeblocks.io/access-mode"
	ComponentLabelKey              = "app.kubernetes.io/component-name"
	RegionLabelKey                 = "topology.kubernetes.io/region"
	ZoneLabelKey                   = "topology.kubernetes.io/zone"
	ClusterDefLabelKey             = "clusterdefinition.kubeblocks.io/name"
	RoleLabelKey                   = "kubeblocks.io/role"

	ServiceLBTypeAnnotationKey     = "service.kubernetes.io/apecloud-loadbalancer-type"
	ServiceLBTypeAnnotationValue   = "private-ip"
	ServiceFloatingIPAnnotationKey = "service.kubernetes.io/apecloud-loadbalancer-floating-ip"
	StorageClassAnnotationKey      = "kubeblocks.io/storage-class"

	// DataProtection definitions
	DPGroup                       = "dataprotection.kubeblocks.io"
	DPVersion                     = "v1alpha1"
	ResourceBackups               = "backups"
	ResourceBackupTools           = "backuptools"
	ResourceRestoreJobs           = "restorejobs"
	ResourceBackupPolicies        = "backuppolicies"
	ResourceBackupPolicyTemplates = "backuppolicytemplates"

	None = "<none>"
)

var (
	// KubeBlocksChartName helm name for installing kubeblocks
	KubeBlocksChartName = "kubeblocks"

	// KubeBlocksChartURL the helm chart repo for installing kubeblocks
	KubeBlocksChartURL = "https://apecloud.github.io/helm-charts"

	// GitLabHelmChartRepo the helm chart repo in GitLab
	GitLabHelmChartRepo = "https://jihulab.com/api/v4/projects/85949/packages/helm/stable"
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

func ClusterDefGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceClusterDefs}
}

func ClusterVersionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceClusterVersions}
}

func BackupGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceBackups}
}

func BackupPolicyTemplateGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceBackupPolicyTemplates}
}

func RestoreJobGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceRestoreJobs}
}

func BackupToolGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPGroup, Version: DPVersion, Resource: ResourceBackupTools}
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

func CMGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: VersionV1, Resource: ResourceConfigmaps}
}

func STSGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: appsv1.GroupName, Version: VersionV1, Resource: ResourceStatefulSets}
}

func ConfigConstraintGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceConfigConstraintVersions}
}

func StorageClassGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "storage.k8s.io",
		Version:  VersionV1,
		Resource: "storageclasses",
	}
}

func VolumeSnapshotClassGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  VersionV1,
		Resource: "volumesnapshotclasses",
	}
}
