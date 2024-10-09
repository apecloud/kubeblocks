/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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

var InstanceSetSignature = func(_ workloads.InstanceSet, _ *workloads.InstanceSet, _ workloads.InstanceSetList, _ *workloads.InstanceSetList) {
}

var JobSignature = func(_ batchv1.Job, _ *batchv1.Job, _ batchv1.JobList, _ *batchv1.JobList) {}
var CronJobSignature = func(_ batchv1.CronJob, _ *batchv1.CronJob, _ batchv1.CronJobList, _ *batchv1.CronJobList) {}

var StorageClassSignature = func(_ storagev1.StorageClass, _ *storagev1.StorageClass, _ storagev1.StorageClassList, _ *storagev1.StorageClassList) {
}
var CSIDriverSignature = func(_ storagev1.CSIDriver, _ *storagev1.CSIDriver, _ storagev1.CSIDriverList, _ *storagev1.CSIDriverList) {
}

var VolumeSnapshotSignature = func(_ snapshotv1.VolumeSnapshot, _ *snapshotv1.VolumeSnapshot, _ snapshotv1.VolumeSnapshotList, _ *snapshotv1.VolumeSnapshotList) {
}
var VolumeSnapshotClassSignature = func(_ snapshotv1.VolumeSnapshotClass, _ *snapshotv1.VolumeSnapshotClass, _ snapshotv1.VolumeSnapshotClassList, _ *snapshotv1.VolumeSnapshotClassList) {
}

var ClusterSignature = func(_ appsv1.Cluster, _ *appsv1.Cluster, _ appsv1.ClusterList, _ *appsv1.ClusterList) {
}
var ClusterDefinitionSignature = func(_ appsv1.ClusterDefinition, _ *appsv1.ClusterDefinition, _ appsv1.ClusterDefinitionList, _ *appsv1.ClusterDefinitionList) {
}
var ComponentSignature = func(appsv1.Component, *appsv1.Component, appsv1.ComponentList, *appsv1.ComponentList) {
}
var ComponentDefinitionSignature = func(appsv1.ComponentDefinition, *appsv1.ComponentDefinition, appsv1.ComponentDefinitionList, *appsv1.ComponentDefinitionList) {
}
var ComponentVersionSignature = func(appsv1.ComponentVersion, *appsv1.ComponentVersion, appsv1.ComponentVersionList, *appsv1.ComponentVersionList) {
}
var OpsDefinitionSignature = func(_ opsv1alpha1.OpsDefinition, _ *opsv1alpha1.OpsDefinition, _ opsv1alpha1.OpsDefinitionList, _ *opsv1alpha1.OpsDefinitionList) {
}
var OpsRequestSignature = func(_ opsv1alpha1.OpsRequest, _ *opsv1alpha1.OpsRequest, _ opsv1alpha1.OpsRequestList, _ *opsv1alpha1.OpsRequestList) {
}
var ConfigConstraintSignature = func(_ appsv1beta1.ConfigConstraint, _ *appsv1beta1.ConfigConstraint, _ appsv1beta1.ConfigConstraintList, _ *appsv1beta1.ConfigConstraintList) {
}

var BackupPolicyTemplateSignature = func(_ dpv1alpha1.BackupPolicyTemplate, _ *dpv1alpha1.BackupPolicyTemplate, _ dpv1alpha1.BackupPolicyTemplateList, _ *dpv1alpha1.BackupPolicyTemplateList) {
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

var StorageProviderSignature = func(_ dpv1alpha1.StorageProvider, _ *dpv1alpha1.StorageProvider, _ dpv1alpha1.StorageProviderList, _ *dpv1alpha1.StorageProviderList) {
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
