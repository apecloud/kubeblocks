package controllerutil

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
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

// ObjListTraits A wrapper of resource objects, since golang generics currently
// doesn't support fields access use a workaround mentioned in https://github.com/golang/go/issues/48522
type ObjListTraits[T Object, L ObjList[T]] interface {
	GetItems(l *L) []T
}

// SecretListTraits ObjListTraits of corev1.SecretList
type SecretListTraits struct{}

func (w SecretListTraits) GetItems(list *corev1.SecretList) []corev1.Secret {
	return list.Items
}

var SecretSignature = func(_ corev1.Secret, _ corev1.SecretList, _ SecretListTraits) {}

// ServiceListTraits ObjListTraits of corev1.ServiceList
type ServiceListTraits struct{}

func (w ServiceListTraits) GetItems(list *corev1.ServiceList) []corev1.Service {
	return list.Items
}

var ServiceSignature = func(_ corev1.Service, _ corev1.ServiceList, _ ServiceListTraits) {}

// StatefulSetListTraits ObjListTraits of appsv1.StatefulSetList
type StatefulSetListTraits struct{}

func (w StatefulSetListTraits) GetItems(list *appsv1.StatefulSetList) []appsv1.StatefulSet {
	return list.Items
}

var StatefulSetSignature = func(_ appsv1.StatefulSet, _ appsv1.StatefulSetList, _ StatefulSetListTraits) {}

// DeploymentListTraits ObjListTraits of appsv1.DeploymentList
type DeploymentListTraits struct{}

func (w DeploymentListTraits) GetItems(list *appsv1.DeploymentList) []appsv1.Deployment {
	return list.Items
}

var DeploymentSignature = func(_ appsv1.Deployment, _ appsv1.DeploymentList, _ DeploymentListTraits) {}

// ConfigMapListTraits ObjListTraits of corev1.ConfigMapList
type ConfigMapListTraits struct{}

func (w ConfigMapListTraits) GetItems(list *corev1.ConfigMapList) []corev1.ConfigMap {
	return list.Items
}

var ConfigMapSignature = func(_ corev1.ConfigMap, _ corev1.ConfigMapList, _ ConfigMapListTraits) {}

// PodDisruptionBudgetListTraits ObjListTraits of policyv1.PodDisruptionBudgetList
type PodDisruptionBudgetListTraits struct{}

func (w PodDisruptionBudgetListTraits) GetItems(list *policyv1.PodDisruptionBudgetList) []policyv1.PodDisruptionBudget {
	return list.Items
}

var PodDisruptionBudgetSignature = func(_ policyv1.PodDisruptionBudget, _ policyv1.PodDisruptionBudgetList, _ PodDisruptionBudgetListTraits) {
}

// PersistentVolumeClaimListTraits ObjListTraits of corev1.PersistentVolumeClaimList
type PersistentVolumeClaimListTraits struct{}

func (w PersistentVolumeClaimListTraits) GetItems(list *corev1.PersistentVolumeClaimList) []corev1.PersistentVolumeClaim {
	return list.Items
}

var PersistentVolumeClaimSignature = func(_ corev1.PersistentVolumeClaim, _ corev1.PersistentVolumeClaimList, _ PersistentVolumeClaimListTraits) {
}

type StorageClassListTraits struct{}

func (w StorageClassListTraits) GetItems(list *storagev1.StorageClassList) []storagev1.StorageClass {
	return list.Items
}

var StorageClassSignature = func(_ storagev1.StorageClass, _ storagev1.StorageClassList, _ StorageClassListTraits) {}

type PodListTraits struct{}

func (w PodListTraits) GetItems(list *corev1.PodList) []corev1.Pod {
	return list.Items
}

var PodSignature = func(_ corev1.Pod, _ corev1.PodList, _ PodListTraits) {}

type EndpointsListTraits struct{}

func (w EndpointsListTraits) GetItems(list *corev1.EndpointsList) []corev1.Endpoints {
	return list.Items
}

var EndpointsSignature = func(_ corev1.Endpoints, _ corev1.EndpointsList, _ EndpointsListTraits) {}

type JobListTraits struct{}

func (w JobListTraits) GetItems(list *batchv1.JobList) []batchv1.Job {
	return list.Items
}

var JobSignature = func(_ batchv1.Job, _ batchv1.JobList, _ JobListTraits) {}

type VolumeSnapshotListTraits struct{}

func (w VolumeSnapshotListTraits) GetItems(list *snapshotv1.VolumeSnapshotList) []snapshotv1.VolumeSnapshot {
	return list.Items
}

var VolumeSnapshotSignature = func(_ snapshotv1.VolumeSnapshot, _ snapshotv1.VolumeSnapshotList, _ VolumeSnapshotListTraits) {}

type ClusterListTraits struct{}

func (w ClusterListTraits) GetItems(list *dbaasv1alpha1.ClusterList) []dbaasv1alpha1.Cluster {
	return list.Items
}

var ClusterSignature = func(_ dbaasv1alpha1.Cluster, _ dbaasv1alpha1.ClusterList, _ ClusterListTraits) {}

type ClusterVersionListTraits struct{}

func (w ClusterVersionListTraits) GetItems(list *dbaasv1alpha1.ClusterVersionList) []dbaasv1alpha1.ClusterVersion {
	return list.Items
}

var ClusterVersionSignature = func(_ dbaasv1alpha1.ClusterVersion, _ dbaasv1alpha1.ClusterVersionList, _ ClusterVersionListTraits) {}

type ClusterDefinitionListTraits struct{}

func (w ClusterDefinitionListTraits) GetItems(list *dbaasv1alpha1.ClusterDefinitionList) []dbaasv1alpha1.ClusterDefinition {
	return list.Items
}

var ClusterDefinitionSignature = func(_ dbaasv1alpha1.ClusterDefinition, _ dbaasv1alpha1.ClusterDefinitionList, _ ClusterDefinitionListTraits) {
}

type OpsRequestListTraits struct{}

func (w OpsRequestListTraits) GetItems(list *dbaasv1alpha1.OpsRequestList) []dbaasv1alpha1.OpsRequest {
	return list.Items
}

var OpsRequestSignature = func(_ dbaasv1alpha1.OpsRequest, _ dbaasv1alpha1.OpsRequestList, _ OpsRequestListTraits) {}

type ConfigConstraintListTraits struct{}

func (w ConfigConstraintListTraits) GetItems(list *dbaasv1alpha1.ConfigConstraintList) []dbaasv1alpha1.ConfigConstraint {
	return list.Items
}

var ConfigConstraintSignature = func(_ dbaasv1alpha1.ConfigConstraint, _ dbaasv1alpha1.ConfigConstraintList, _ ConfigConstraintListTraits) {
}

type BackupPolicyTemplateListTraits struct{}

func (w BackupPolicyTemplateListTraits) GetItems(list *dataprotectionv1alpha1.BackupPolicyTemplateList) []dataprotectionv1alpha1.BackupPolicyTemplate {
	return list.Items
}

var BackupPolicyTemplateSignature = func(_ dataprotectionv1alpha1.BackupPolicyTemplate, _ dataprotectionv1alpha1.BackupPolicyTemplateList, _ BackupPolicyTemplateListTraits) {
}

type BackupPolicyListTraits struct{}

func (w BackupPolicyListTraits) GetItems(list *dataprotectionv1alpha1.BackupPolicyList) []dataprotectionv1alpha1.BackupPolicy {
	return list.Items
}

var BackupPolicySignature = func(_ dataprotectionv1alpha1.BackupPolicy, _ dataprotectionv1alpha1.BackupPolicyList, _ BackupPolicyListTraits) {
}

type BackupListTraits struct{}

func (w BackupListTraits) GetItems(list *dataprotectionv1alpha1.BackupList) []dataprotectionv1alpha1.Backup {
	return list.Items
}
