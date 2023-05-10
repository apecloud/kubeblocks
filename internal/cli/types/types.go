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

	// DefaultLogFilePrefix is the default log file prefix
	DefaultLogFilePrefix = "kbcli"

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
	AppsAPIGroup                        = "apps.kubeblocks.io"
	AppsAPIVersion                      = "v1alpha1"
	ResourcePods                        = "pods"
	ResourceClusters                    = "clusters"
	ResourceClusterDefs                 = "clusterdefinitions"
	ResourceClusterVersions             = "clusterversions"
	ResourceOpsRequests                 = "opsrequests"
	ResourceConfigConstraintVersions    = "configconstraints"
	ResourceComponentResourceConstraint = "componentresourceconstraints"
	ResourceComponentClassDefinition    = "componentclassdefinitions"
	KindCluster                         = "Cluster"
	KindComponentClassDefinition        = "ComponentClassDefinition"
	KindClusterDef                      = "ClusterDefinition"
	KindClusterVersion                  = "ClusterVersion"
	KindConfigConstraint                = "ConfigConstraint"
	KindBackup                          = "Backup"
	KindRestoreJob                      = "RestoreJob"
	KindBackupPolicy                    = "BackupPolicy"
	KindOps                             = "OpsRequest"
)

// K8S rbac API group
const (
	RBACAPIGroup        = rbacv1.GroupName
	RBACAPIVersion      = "v1"
	ClusterRoles        = "clusterroles"
	ClusterRoleBindings = "clusterrolebindings"
	Roles               = "roles"
	RoleBindings        = "rolebindings"
	ServiceAccounts     = "serviceaccounts"
)

// Annotations
const (
	ServiceHAVIPTypeAnnotationKey   = "service.kubernetes.io/kubeblocks-havip-type"
	ServiceHAVIPTypeAnnotationValue = "private-ip"
	ServiceFloatingIPAnnotationKey  = "service.kubernetes.io/kubeblocks-havip-floating-ip"

	ClassProviderLabelKey              = "class.kubeblocks.io/provider"
	ResourceConstraintProviderLabelKey = "resourceconstraint.kubeblocks.io/provider"
	ReloadConfigMapAnnotationKey       = "kubeblocks.io/reload-configmap" // mark an annotation to load configmap
)

// DataProtection API group
const (
	DPAPIGroup             = "dataprotection.kubeblocks.io"
	DPAPIVersion           = "v1alpha1"
	ResourceBackups        = "backups"
	ResourceBackupTools    = "backuptools"
	ResourceRestoreJobs    = "restorejobs"
	ResourceBackupPolicies = "backuppolicies"
)

// Extensions API group
const (
	ExtensionsAPIGroup   = "extensions.kubeblocks.io"
	ExtensionsAPIVersion = "v1alpha1"
	ResourceAddons       = "addons"
)

// Migration API group
const (
	MigrationAPIGroup          = "datamigration.apecloud.io"
	MigrationAPIVersion        = "v1alpha1"
	ResourceMigrationTasks     = "migrationtasks"
	ResourceMigrationTemplates = "migrationtemplates"
)

// Crd Api group
const (
	CustomResourceDefinitionAPIGroup   = "apiextensions.k8s.io"
	CustomResourceDefinitionAPIVersion = "v1"
	ResourceCustomResourceDefinition   = "customresourcedefinitions"
)

const (
	None = "<none>"

	// AddonReleasePrefix is the prefix of addon release name
	AddonReleasePrefix = "kb-addon"
)

var (
	// KubeBlocksName is the name of KubeBlocks project
	KubeBlocksName = "kubeblocks"

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

func PodGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "", Version: K8sCoreAPIVersion, Resource: ResourcePods}
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

func ComponentResourceConstraintGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceComponentResourceConstraint}
}

func ComponentClassDefinitionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: AppsAPIGroup, Version: AppsAPIVersion, Resource: ResourceComponentClassDefinition}
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

func RoleGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: RBACAPIGroup, Version: RBACAPIVersion, Resource: Roles}
}

func RoleBindingGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: RBACAPIGroup, Version: RBACAPIVersion, Resource: RoleBindings}
}

func ServiceAccountGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: corev1.GroupName, Version: K8sCoreAPIVersion, Resource: ServiceAccounts}
}

func MigrationTaskGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    MigrationAPIGroup,
		Version:  MigrationAPIVersion,
		Resource: ResourceMigrationTasks,
	}
}

func MigrationTemplateGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    MigrationAPIGroup,
		Version:  MigrationAPIVersion,
		Resource: ResourceMigrationTemplates,
	}
}

func CustomResourceDefinitionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    CustomResourceDefinitionAPIGroup,
		Version:  CustomResourceDefinitionAPIVersion,
		Resource: ResourceCustomResourceDefinition,
	}
}
