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
)

type DBClusterInfo struct {
	DBCluster       string
	DBPort          string
	Version         string
	Topology        string
	Status          string
	StartTime       string
	Labels          string
	RootUser        string
	DBNamespace     string
	Instances       int64
	ServerId        int64
	Secret          string
	OnlineInstances int64
	Storage         int64
	Engine          string
	HostIP          string
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
