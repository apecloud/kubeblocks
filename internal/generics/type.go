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

package generics

import (
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
)

// Object a generic representation of various resource object types
type Object interface{}

// PObject pointer of Object
type PObject[T Object] interface {
	*T
	client.Object
	DeepCopy() *T // DeepCopy have a pointer receiver
}

// ObjList a generic representation of various resource list object types
type ObjList[T Object] interface{}

// PObjList pointer of ObjList
type PObjList[T Object, L ObjList[T]] interface {
	*L
	client.ObjectList
}

// signature is used as an argument passed to generic functions for type deduction.

var SecretSignature = func(_ corev1.Secret, _ corev1.SecretList) {}
var ServiceSignature = func(_ corev1.Service, _ corev1.ServiceList) {}
var ServiceAccountSignature = func(_ corev1.ServiceAccount, _ corev1.ServiceAccountList) {}
var ClusterRoleSignature = func(_ rbacv1.ClusterRole, _ rbacv1.ClusterRoleList) {}
var ClusterRoleBindingSignature = func(_ rbacv1.ClusterRoleBinding, _ rbacv1.ClusterRoleBindingList) {}
var PersistentVolumeClaimSignature = func(_ corev1.PersistentVolumeClaim, _ corev1.PersistentVolumeClaimList) {}
var PodSignature = func(_ corev1.Pod, _ corev1.PodList) {}
var EventSignature = func(_ corev1.Event, _ corev1.EventList) {}
var ConfigMapSignature = func(_ corev1.ConfigMap, _ corev1.ConfigMapList) {}
var EndpointsSignature = func(_ corev1.Endpoints, _ corev1.EndpointsList) {}

var StatefulSetSignature = func(_ appsv1.StatefulSet, _ appsv1.StatefulSetList) {}
var DeploymentSignature = func(_ appsv1.Deployment, _ appsv1.DeploymentList) {}

var JobSignature = func(_ batchv1.Job, _ batchv1.JobList) {}
var CronJobSignature = func(_ batchv1.CronJob, _ batchv1.CronJobList) {}

var PodDisruptionBudgetSignature = func(_ policyv1.PodDisruptionBudget, _ policyv1.PodDisruptionBudgetList) {
}

var StorageClassSignature = func(_ storagev1.StorageClass, _ storagev1.StorageClassList) {}

var VolumeSnapshotSignature = func(_ snapshotv1.VolumeSnapshot, _ snapshotv1.VolumeSnapshotList) {}

var ClusterSignature = func(_ appsv1alpha1.Cluster, _ appsv1alpha1.ClusterList) {}
var ClusterVersionSignature = func(_ appsv1alpha1.ClusterVersion, _ appsv1alpha1.ClusterVersionList) {}
var ClusterDefinitionSignature = func(_ appsv1alpha1.ClusterDefinition, _ appsv1alpha1.ClusterDefinitionList) {
}
var OpsRequestSignature = func(_ appsv1alpha1.OpsRequest, _ appsv1alpha1.OpsRequestList) {}
var ConfigConstraintSignature = func(_ appsv1alpha1.ConfigConstraint, _ appsv1alpha1.ConfigConstraintList) {
}

var BackupPolicyTemplateSignature = func(_ dataprotectionv1alpha1.BackupPolicyTemplate, _ dataprotectionv1alpha1.BackupPolicyTemplateList) {
}
var BackupPolicySignature = func(_ dataprotectionv1alpha1.BackupPolicy, _ dataprotectionv1alpha1.BackupPolicyList) {
}
var BackupSignature = func(_ dataprotectionv1alpha1.Backup, _ dataprotectionv1alpha1.BackupList) {
}
var BackupToolSignature = func(_ dataprotectionv1alpha1.BackupTool, _ dataprotectionv1alpha1.BackupToolList) {
}
var RestoreJobSignature = func(_ dataprotectionv1alpha1.RestoreJob, _ dataprotectionv1alpha1.RestoreJobList) {
}
var AddonSignature = func(_ extensionsv1alpha1.Addon, _ extensionsv1alpha1.AddonList) {
}
