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

package generics

import (
	"reflect"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
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
// Goland IDE 2023.2.1 and 2023.2.2 code inspector has a bug to infer pointer type like PObject and PObjList from Object and ObjectList.
// To workaround this bug, we also pass the pointer type to generic functions in signature.

var SecretSignature = func(_ corev1.Secret, _ *corev1.Secret, _ corev1.SecretList, _ *corev1.SecretList) {}
var ServiceSignature = func(_ corev1.Service, _ *corev1.Service, _ corev1.ServiceList, _ *corev1.ServiceList) {}
var PersistentVolumeClaimSignature = func(_ corev1.PersistentVolumeClaim, _ *corev1.PersistentVolumeClaim, _ corev1.PersistentVolumeClaimList, _ *corev1.PersistentVolumeClaimList) {
}
var PersistentVolumeSignature = func(_ corev1.PersistentVolume, _ *corev1.PersistentVolume, _ corev1.PersistentVolumeList, _ *corev1.PersistentVolumeList) {
}
var PodSignature = func(_ corev1.Pod, _ *corev1.Pod, _ corev1.PodList, _ *corev1.PodList) {}
var EventSignature = func(_ corev1.Event, _ *corev1.Event, _ corev1.EventList, _ *corev1.EventList) {}
var ConfigMapSignature = func(_ corev1.ConfigMap, _ *corev1.ConfigMap, _ corev1.ConfigMapList, _ *corev1.ConfigMapList) {}
var EndpointsSignature = func(_ corev1.Endpoints, _ *corev1.Endpoints, _ corev1.EndpointsList, _ *corev1.EndpointsList) {}

var RSMSignature = func(_ workloads.ReplicatedStateMachine, _ *workloads.ReplicatedStateMachine, _ workloads.ReplicatedStateMachineList, _ *workloads.ReplicatedStateMachineList) {
}
var StatefulSetSignature = func(_ appsv1.StatefulSet, _ *appsv1.StatefulSet, _ appsv1.StatefulSetList, _ *appsv1.StatefulSetList) {
}
var DeploymentSignature = func(_ appsv1.Deployment, _ *appsv1.Deployment, _ appsv1.DeploymentList, _ *appsv1.DeploymentList) {}
var ReplicaSetSignature = func(_ appsv1.ReplicaSet, _ *appsv1.ReplicaSet, _ appsv1.ReplicaSetList, _ *appsv1.ReplicaSetList) {}

var JobSignature = func(_ batchv1.Job, _ *batchv1.Job, _ batchv1.JobList, _ *batchv1.JobList) {}
var CronJobSignature = func(_ batchv1.CronJob, _ *batchv1.CronJob, _ batchv1.CronJobList, _ *batchv1.CronJobList) {}

var PodDisruptionBudgetSignature = func(_ policyv1.PodDisruptionBudget, _ *policyv1.PodDisruptionBudget, _ policyv1.PodDisruptionBudgetList, _ *policyv1.PodDisruptionBudgetList) {
}

var StorageClassSignature = func(_ storagev1.StorageClass, _ *storagev1.StorageClass, _ storagev1.StorageClassList, _ *storagev1.StorageClassList) {
}
var CSIDriverSignature = func(_ storagev1.CSIDriver, _ *storagev1.CSIDriver, _ storagev1.CSIDriverList, _ *storagev1.CSIDriverList) {
}

var VolumeSnapshotSignature = func(_ snapshotv1.VolumeSnapshot, _ *snapshotv1.VolumeSnapshot, _ snapshotv1.VolumeSnapshotList, _ *snapshotv1.VolumeSnapshotList) {
}
var VolumeSnapshotClassSignature = func(_ snapshotv1.VolumeSnapshotClass, _ *snapshotv1.VolumeSnapshotClass, _ snapshotv1.VolumeSnapshotClassList, _ *snapshotv1.VolumeSnapshotClassList) {
}

var ClusterSignature = func(_ appsv1alpha1.Cluster, _ *appsv1alpha1.Cluster, _ appsv1alpha1.ClusterList, _ *appsv1alpha1.ClusterList) {
}
var ClusterVersionSignature = func(_ appsv1alpha1.ClusterVersion, _ *appsv1alpha1.ClusterVersion, _ appsv1alpha1.ClusterVersionList, _ *appsv1alpha1.ClusterVersionList) {
}
var ClusterDefinitionSignature = func(_ appsv1alpha1.ClusterDefinition, _ *appsv1alpha1.ClusterDefinition, _ appsv1alpha1.ClusterDefinitionList, _ *appsv1alpha1.ClusterDefinitionList) {
}
var OpsRequestSignature = func(_ appsv1alpha1.OpsRequest, _ *appsv1alpha1.OpsRequest, _ appsv1alpha1.OpsRequestList, _ *appsv1alpha1.OpsRequestList) {
}
var ConfigConstraintSignature = func(_ appsv1alpha1.ConfigConstraint, _ *appsv1alpha1.ConfigConstraint, _ appsv1alpha1.ConfigConstraintList, _ *appsv1alpha1.ConfigConstraintList) {
}

var BackupPolicyTemplateSignature = func(_ appsv1alpha1.BackupPolicyTemplate, _ *appsv1alpha1.BackupPolicyTemplate, _ appsv1alpha1.BackupPolicyTemplateList, _ *appsv1alpha1.BackupPolicyTemplateList) {
}
var BackupPolicySignature = func(_ dpv1alpha1.BackupPolicy, _ *dpv1alpha1.BackupPolicy, _ dpv1alpha1.BackupPolicyList, _ *dpv1alpha1.BackupPolicyList) {
}
var BackupSignature = func(_ dpv1alpha1.Backup, _ *dpv1alpha1.Backup, _ dpv1alpha1.BackupList, _ *dpv1alpha1.BackupList) {
}
var BackupScheduleSignature = func(_ dpv1alpha1.BackupSchedule, _ *dpv1alpha1.BackupSchedule, _ dpv1alpha1.BackupScheduleList, _ *dpv1alpha1.BackupScheduleList) {
}
var RestoreSignature = func(_ dpv1alpha1.Restore, _ *dpv1alpha1.Restore, _ dpv1alpha1.RestoreList, _ *dpv1alpha1.RestoreList) {
}
var ActionSetSignature = func(_ dpv1alpha1.ActionSet, _ *dpv1alpha1.ActionSet, _ dpv1alpha1.ActionSetList, _ *dpv1alpha1.ActionSetList) {
}
var BackupRepoSignature = func(_ dpv1alpha1.BackupRepo, _ *dpv1alpha1.BackupRepo, _ dpv1alpha1.BackupRepoList, _ *dpv1alpha1.BackupRepoList) {
}

var AddonSignature = func(_ extensionsv1alpha1.Addon, _ *extensionsv1alpha1.Addon, _ extensionsv1alpha1.AddonList, _ *extensionsv1alpha1.AddonList) {
}
var ComponentResourceConstraintSignature = func(_ appsv1alpha1.ComponentResourceConstraint, _ *appsv1alpha1.ComponentResourceConstraint, _ appsv1alpha1.ComponentResourceConstraintList, _ *appsv1alpha1.ComponentResourceConstraintList) {
}
var ComponentClassDefinitionSignature = func(_ appsv1alpha1.ComponentClassDefinition, _ *appsv1alpha1.ComponentClassDefinition, _ appsv1alpha1.ComponentClassDefinitionList, _ *appsv1alpha1.ComponentClassDefinitionList) {
}

var StorageProviderSignature = func(_ storagev1alpha1.StorageProvider, _ *storagev1alpha1.StorageProvider, _ storagev1alpha1.StorageProviderList, _ *storagev1alpha1.StorageProviderList) {
}

var ConfigurationSignature = func(_ appsv1alpha1.Configuration, _ *appsv1alpha1.Configuration, _ appsv1alpha1.ConfigurationList, _ *appsv1alpha1.ConfigurationList) {
}

func ToGVK(object client.Object) schema.GroupVersionKind {
	t := reflect.TypeOf(object)
	if t.Kind() != reflect.Pointer {
		// Shouldn't ever get here.
		return schema.GroupVersionKind{}
	}
	t = t.Elem()
	return corev1.SchemeGroupVersion.WithKind(t.Name())
}
