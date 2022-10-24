/*
Copyright 2022 The KubeBlocks Authors

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	// DBCtlDefaultHome defines dbctl default home name
	DBCtlDefaultHome = ".dbctl"
	// DBCtlHomeEnv defines dbctl home system env
	DBCtlHomeEnv = "DBCTL_HOME"

	// GoosLinux is os.GOOS linux string
	GoosLinux = "linux"
	// GoosDarwin is os.GOOS darwin string
	GoosDarwin = "darwin"
	// GoosWindows is os.GOOS windows string
	GoosWindows = "windows"

	// DbaasDefaultVersion default kubeblocks version to install
	DbaasDefaultVersion = "0.1.0-alpha.5"
	// DbaasHelmName helm name for installing kubeblocks
	DbaasHelmName = "opendbaas-core"
	// DbaasHelmChart the helm chart for installing kubeblocks
	DbaasHelmChart = "oci://yimeisun.azurecr.io/helm-chart/opendbaas-core"

	// PlaygroundSourceName is the playground default operator
	PlaygroundSourceName = "innodbclusters"

	// BackupJobSourceName is the playground default operator
	BackupJobSourceName = "backupJobs"

	// RestoreJobSourceName is the playground default operator
	RestoreJobSourceName = "restoreJobs"

	BackupSnapSourceName = "volumesnapshots"
)

const (
	// Group api group
	Group = "dbaas.infracreate.com"

	// Version api version
	Version = "v1alpha1"

	// ResourceClusters clusters resource
	ResourceClusters = "clusters"

	ResourceClusterDefinitions = "clusterdefinitions"

	ResourceAppVersions = "appversions"

	// ResourceOpsRequest opsrequests resource
	ResourceOpsRequest = "opsrequests"

	// KindCluster kind of cluster
	KindCluster = "Cluster"

	// ResourceClusterDefs clusterDefinition resource
	ResourceClusterDefs = "clusterdefinitions"

	// KindClusterDef kind of clusterDefinition
	KindClusterDef = "ClusterDefinition"

	ResourceAppVersion = "appversions"
	KindAppVersion     = "AppVersion"

	InstanceLabelKey         = "app.kubernetes.io/instance"
	ConsensusSetRoleLabelKey = "cs.dbaas.infracreate.com/role"
	ComponentLabelKey        = "app.kubernetes.io/component-name"
	RegionLabelKey           = "topology.kubernetes.io/region"
	ZoneLabelKey             = "topology.kubernetes.io/zone"

	ServiceLBTypeAnnotationKey     = "service.kubernetes.io/apecloud-loadbalancer-type"
	ServiceLBTypeAnnotationValue   = "private-ip"
	ServiceFloatingIPAnnotationKey = "service.kubernetes.io/apecloud-loadbalancer-floating-ip"
)

type ClusterObjects struct {
	Cluster    *dbaasv1alpha1.Cluster
	ClusterDef *dbaasv1alpha1.ClusterDefinition
	AppVersion *dbaasv1alpha1.AppVersion

	Pods     *corev1.PodList
	Services *corev1.ServiceList
	Secrets  *corev1.SecretList
	Nodes    []*corev1.Node
}

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

func AppVersionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: Group, Version: Version, Resource: ResourceAppVersion}
}

func AppVersionGK() schema.GroupKind {
	return schema.GroupKind{Group: Group, Kind: KindAppVersion}
}
