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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	// CliDefaultHome defines kbcli default home name
	CliDefaultHome = ".kbcli"
	// CliHomeEnv defines kbcli home system env
	CliHomeEnv = "KBCLI_HOME"

	// DefaultNamespace is the namespace where kubeblocks is installed if
	// no other namespace is specified
	DefaultNamespace = "kb-system"

	// GoosLinux is os.GOOS linux string
	GoosLinux = "linux"
	// GoosDarwin is os.GOOS darwin string
	GoosDarwin = "darwin"
	// GoosWindows is os.GOOS windows string
	GoosWindows = "windows"
)

// K8s core API group
const (
	K8sCoreAPIVersion    = "v1"
	ResourceDeployments  = "deployments"
	ResourceConfigmaps   = "configmaps"
	ResourceStatefulSets = "statefulsets"
	ResourceSecrets      = "secrets"
)

// K8s webhook API group
const (
	WebhookAPIGroup                         = "admissionregistration.k8s.io"
	K8sWebhookAPIVersion                    = "v1"
	ResourceValidatingWebhookConfigurations = "validatingwebhookconfigurations"
	ResourceMutatingWebhookConfigurations   = "mutatingwebhookconfigurations"
)

// Apps API group
const (
	AppsAPIGroup                     = "apps.kubeblocks.io"
	AppsAPIVersion                   = "v1alpha1"
	ResourceClusters                 = "clusters"
	ResourceClusterDefs              = "clusterdefinitions"
	ResourceClusterVersions          = "clusterversions"
	ResourceOpsRequests              = "opsrequests"
	ResourceConfigConstraintVersions = "configconstraints"
	ResourceClassFamily              = "classfamilies"
	KindCluster                      = "Cluster"
	KindClusterDef                   = "ClusterDefinition"
	KindClusterVersion               = "ClusterVersion"
	KindConfigConstraint             = "ConfigConstraint"
	KindBackup                       = "Backup"
	KindRestoreJob                   = "RestoreJob"
	KindBackupPolicy                 = "BackupPolicy"
	KindOps                          = "OpsRequest"
)

// K8S rbac API group
const (
	RBACAPIGroup        = rbacv1.GroupName
	RBACAPIVersion      = "v1"
	ClusterRoles        = "clusterroles"
	ClusterRoleBindings = "clusterrolebindings"
)

// Annotations
const (
	ServiceHAVIPTypeAnnotationKey   = "service.kubernetes.io/kubeblocks-havip-type"
	ServiceHAVIPTypeAnnotationValue = "private-ip"
	ServiceFloatingIPAnnotationKey  = "service.kubernetes.io/kubeblocks-havip-floating-ip"

	ClassLevelLabelKey          = "class.kubeblocks.io/level"
	ClassProviderLabelKey       = "class.kubeblocks.io/provider"
	ClassFamilyProviderLabelKey = "classfamily.kubeblocks.io/provider"
	ComponentClassAnnotationKey = "cluster.kubeblocks.io/component-class"
)

// DataProtection API group
const (
	DPAPIGroup                    = "dataprotection.kubeblocks.io"
	DPAPIVersion                  = "v1alpha1"
	ResourceBackups               = "backups"
	ResourceBackupTools           = "backuptools"
	ResourceRestoreJobs           = "restorejobs"
	ResourceBackupPolicies        = "backuppolicies"
	ResourceBackupPolicyTemplates = "backuppolicytemplates"
)

// Extensions API group
const (
	ExtensionsAPIGroup   = "extensions.kubeblocks.io"
	ExtensionsAPIVersion = "v1alpha1"
	ResourceAddons       = "addons"
)

const (
	None = "<none>"

	// AddonReleasePrefix is the prefix of addon release name
	AddonReleasePrefix = "kb-addon"
)

var (
	// KubeBlocksRepoName helm repo name for kubeblocks
	KubeBlocksRepoName = "kubeblocks"

	// KubeBlocksChartName helm chart name for kubeblocks
	KubeBlocksChartName = "kubeblocks"

	// KubeBlocksReleaseName helm release name for kubeblocks
	KubeBlocksReleaseName = "kubeblocks"

	// KubeBlocksChartURL the helm chart repo for installing kubeblocks
	KubeBlocksChartURL = "https://apecloud.github.io/helm-charts"

	// GitLabHelmChartRepo the helm chart repo in GitLab
	GitLabHelmChartRepo = "https://jihulab.com/api/v4/projects/85949/packages/helm/stable"

	// InstanceLabelSelector app.kubernetes.io/instance=kubeblocks, hit most workloads and configuration
	InstanceLabelSelector = fmt.Sprintf("%s=%s", constant.AppInstanceLabelKey, KubeBlocksChartName)

	// ReleaseLabelSelector release=kubeblocks, for prometheus-alertmanager and prometheus-server
	ReleaseLabelSelector = fmt.Sprintf("release=%s", KubeBlocksChartName)

	// KubeBlocksHelmLabel name=kubeblocks,owner-helm, for helm secret
	KubeBlocksHelmLabel = fmt.Sprintf("%s=%s,%s=%s", "name", KubeBlocksChartName, "owner", "helm")
)

// Playground
var (
	// K3dClusterName is the k3d cluster name for playground
	K3dClusterName = "kb-playground"
)

type ConfigTemplateInfo struct {
	Name  string
	TPL   appsv1alpha1.ComponentConfigSpec
	CMObj *corev1.ConfigMap
}

func ClusterGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceClusters}
}

func ClusterDefGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceClusterDefs}
}

func ClusterVersionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceClusterVersions}
}

func OpsGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceOpsRequests}
}

func BackupGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPAPIGroup, Version: DPAPIVersion, Resource: ResourceBackups}
}

func BackupPolicyGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPAPIGroup, Version: DPAPIVersion, Resource: ResourceBackupPolicies}
}

func BackupToolGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPAPIGroup, Version: DPAPIVersion, Resource: ResourceBackupTools}
}

func RestoreJobGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: DPAPIGroup, Version: DPAPIVersion, Resource: ResourceRestoreJobs}
}

func AddonGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: ExtensionsAPIGroup, Version: ExtensionsAPIVersion, Resource: ResourceAddons}
}

func ClassFamilyGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceClassFamily}
}

func CRDGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  K8sCoreAPIVersion,
		Resource: "customresourcedefinitions",
	}
}

func ConfigmapGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: ResourceConfigmaps}
}

func SecretGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: ResourceSecrets}
}

func StatefulSetGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: appsv1.GroupName, Version: K8sCoreAPIVersion, Resource: ResourceStatefulSets}
}

func DeployGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: appsv1.GroupName, Version: K8sCoreAPIVersion, Resource: ResourceDeployments}
}

func ServiceGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: "services"}
}

func PVCGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: "persistentvolumeclaims"}
}

func PVGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: "persistentvolumes"}
}

func ConfigConstraintGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceConfigConstraintVersions}
}

func StorageClassGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "storage.k8s.io",
		Version:  K8sCoreAPIVersion,
		Resource: "storageclasses",
	}
}

func VolumeSnapshotClassGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  K8sCoreAPIVersion,
		Resource: "volumesnapshotclasses",
	}
}

func ValidatingWebhookConfigurationGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    WebhookAPIGroup,
		Version:  K8sWebhookAPIVersion,
		Resource: ResourceValidatingWebhookConfigurations,
	}
}

func MutatingWebhookConfigurationGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    WebhookAPIGroup,
		Version:  K8sWebhookAPIVersion,
		Resource: ResourceMutatingWebhookConfigurations,
	}
}

func ClusterRoleGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: RBACAPIGroup, Version: RBACAPIVersion, Resource: ClusterRoles}
}
func ClusterRoleBindingGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: RBACAPIGroup, Version: RBACAPIVersion, Resource: ClusterRoleBindings}
}
