/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package dataprotection

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestClusterRestoreBuildTargetClusterInjectsRestoreAPI(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	sourceCluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-cluster",
			Namespace: "source-ns",
			Labels:    map[string]string{"source": "label"},
		},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql",
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name: "data",
					Spec: corev1.PersistentVolumeClaimSpec{},
				}},
				SystemAccounts: []appsv1.ComponentSystemAccount{{Name: "root"}},
			}},
		},
	}
	clusterSnapshot, err := json.Marshal(sourceCluster)
	if err != nil {
		t.Fatal(err)
	}
	encryptedPassword, err := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey)).Encrypt([]byte("restored-password"))
	if err != nil {
		t.Fatal(err)
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-backup",
			Namespace: "backup-ns",
			Annotations: map[string]string{
				constant.ClusterSnapshotAnnotationKey:         string(clusterSnapshot),
				constant.EncryptedSystemAccountsAnnotationKey: `{"mysql":{"root":"` + encryptedPassword + `"}}`,
			},
		},
		Status: dpv1alpha1.BackupStatus{
			Targets: []dpv1alpha1.BackupStatusTarget{{
				BackupTarget: dpv1alpha1.BackupTarget{Name: "mysql"},
			}},
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "target-ns",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName:   "target-cluster",
			BackupRef:           dpv1alpha1.ClusterRestoreBackupRef{Name: backup.Name, Namespace: backup.Namespace},
			RestoreTime:         "2026-05-04T08:00:00Z",
			VolumeRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			Env:                 []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}},
			Parameters:          []dpv1alpha1.ParameterPair{{Name: "restore-param", Value: "restore-value"}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &ClusterRestoreReconciler{
		Client: cli,
		Scheme: scheme,
	}

	target, err := reconciler.buildTargetCluster(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore, backup, clusterRestore.Spec.RestoreTime)
	if err != nil {
		t.Fatal(err)
	}

	if target.Name != clusterRestore.Spec.TargetClusterName || target.Namespace != clusterRestore.Namespace {
		t.Fatalf("unexpected target cluster identity: %s/%s", target.Namespace, target.Name)
	}
	if len(target.OwnerReferences) != 0 {
		t.Fatalf("expected target Cluster not to be owned by ClusterRestore, got %#v", target.OwnerReferences)
	}
	if target.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] != string(clusterRestore.UID) {
		t.Fatalf("expected target Cluster to carry restore uid annotation, got %#v", target.Annotations)
	}
	vct := target.Spec.ComponentSpecs[0].VolumeClaimTemplates[0]
	if !dprestore.IsBackupDataSourceRef(vct.Spec.DataSourceRef) {
		t.Fatalf("expected Backup dataSourceRef, got %#v", vct.Spec.DataSourceRef)
	}
	if vct.Spec.DataSourceRef.Name != backup.Name {
		t.Fatalf("expected dataSourceRef name %q, got %q", backup.Name, vct.Spec.DataSourceRef.Name)
	}
	if vct.Spec.DataSourceRef.Namespace != nil {
		t.Fatalf("expected Backup namespace to stay in ClusterRestore spec, got PVC dataSourceRef namespace %q", *vct.Spec.DataSourceRef.Namespace)
	}
	if vct.Labels[dptypes.ClusterRestoreLabelKey] != clusterRestore.Name {
		t.Fatalf("expected cluster restore label, got %#v", vct.Labels)
	}
	if vct.Annotations[dptypes.VolumeSourceAnnotationKey] != vct.Name ||
		vct.Annotations[dptypes.SourceTargetNameAnnotationKey] != "mysql" ||
		vct.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] != string(clusterRestore.UID) {
		t.Fatalf("unexpected restore annotations: %#v", vct.Annotations)
	}
	account := target.Spec.ComponentSpecs[0].SystemAccounts[0]
	if account.SecretRef == nil {
		t.Fatal("expected restored account secretRef to be injected")
	}
	passwordKey := clusterRestoreSourceAccountPasswordKey("mysql", "root")
	if account.SecretRef.Name != clusterRestoreSourceAccountSecretName(target.Name, clusterRestore.Name) ||
		account.SecretRef.Namespace != clusterRestore.Namespace ||
		account.SecretRef.Password != passwordKey {
		t.Fatalf("unexpected account secretRef: %#v", account.SecretRef)
	}
	sourceSecret := &corev1.Secret{}
	if err = cli.Get(context.Background(), client.ObjectKey{Namespace: clusterRestore.Namespace, Name: account.SecretRef.Name}, sourceSecret); err != nil {
		t.Fatal(err)
	}
	if string(sourceSecret.Data[passwordKey]) != "restored-password" {
		t.Fatalf("expected restored password, got %q", string(sourceSecret.Data[passwordKey]))
	}
	if sourceSecret.Annotations[constant.SystemAccountProvisionedAnnotationKey] != "true" {
		t.Fatalf("expected source secret to be marked provisioned, got %#v", sourceSecret.Annotations)
	}
	if !metav1.IsControlledBy(sourceSecret, clusterRestore) {
		t.Fatalf("expected source secret to be owned by ClusterRestore, got %#v", sourceSecret.OwnerReferences)
	}
	if sourceSecret.Labels[constant.KBAppComponentLabelKey] != "" {
		t.Fatalf("expected source secret not to look like a component account secret, got labels %#v", sourceSecret.Labels)
	}
	if sourceSecret.Labels["apps.kubeblocks.io/system-account"] != "" {
		t.Fatalf("expected source secret not to carry apps system-account selector, got labels %#v", sourceSecret.Labels)
	}
}

func TestClusterRestoreBuildsTargetClusterFromTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-backup",
			Namespace: "backup-ns",
		},
		Status: dpv1alpha1.BackupStatus{
			Targets: []dpv1alpha1.BackupStatusTarget{{
				BackupTarget: dpv1alpha1.BackupTarget{Name: "mysql"},
			}},
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "target-ns",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: backup.Name, Namespace: backup.Namespace},
			TargetClusterTemplate: &dpv1alpha1.ClusterRestoreTargetClusterTemplate{
				Labels:      map[string]string{"user-label": "user-value"},
				Annotations: map[string]string{"user-annotation": "user-value"},
				Spec: appsv1.ClusterSpec{
					ClusterDef:        "mysql",
					TerminationPolicy: appsv1.Delete,
					ComponentSpecs: []appsv1.ClusterComponentSpec{{
						Name:           "mysql",
						ServiceVersion: "8.0.30",
						Replicas:       3,
						VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
							Name: "data",
							Spec: corev1.PersistentVolumeClaimSpec{},
						}},
					}},
				},
			},
			VolumeRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &ClusterRestoreReconciler{
		Client: cli,
		Scheme: scheme,
	}

	target, err := reconciler.buildTargetCluster(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore, backup, clusterRestore.Spec.RestoreTime)
	if err != nil {
		t.Fatal(err)
	}

	if target.Name != clusterRestore.Spec.TargetClusterName || target.Namespace != clusterRestore.Namespace {
		t.Fatalf("unexpected target cluster identity: %s/%s", target.Namespace, target.Name)
	}
	if target.Labels["user-label"] != "user-value" || target.Labels[dptypes.ClusterRestoreLabelKey] != clusterRestore.Name {
		t.Fatalf("unexpected target labels: %#v", target.Labels)
	}
	if target.Annotations["user-annotation"] != "user-value" ||
		target.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] != string(clusterRestore.UID) {
		t.Fatalf("unexpected target annotations: %#v", target.Annotations)
	}
	if target.Spec.ClusterDef != "mysql" ||
		target.Spec.ComponentSpecs[0].ServiceVersion != "8.0.30" ||
		target.Spec.ComponentSpecs[0].Replicas != 3 {
		t.Fatalf("expected target spec to come from template, got %#v", target.Spec)
	}
	vct := target.Spec.ComponentSpecs[0].VolumeClaimTemplates[0]
	if !dprestore.IsBackupDataSourceRef(vct.Spec.DataSourceRef) {
		t.Fatalf("expected Backup dataSourceRef, got %#v", vct.Spec.DataSourceRef)
	}
	if vct.Annotations[dptypes.SourceTargetNameAnnotationKey] != "mysql" ||
		vct.Annotations[dptypes.VolumeSourceAnnotationKey] != "data" {
		t.Fatalf("unexpected restore annotations: %#v", vct.Annotations)
	}
}

func TestVolumePopulatorWaitForPVCSelectedNodeReturnsStorageClassSentinel(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
	storageClassName := "missing-sc"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
		},
	}

	wait, nodeName, err := reconciler.waitForPVCSelectedNode(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	if wait {
		t.Fatal("expected missing StorageClass to stop waiting and return an error")
	}
	if nodeName != "" {
		t.Fatalf("nodeName = %q, want empty", nodeName)
	}
	scErr, ok := dprestore.IsStorageClassNotFoundError(err)
	if !ok || scErr == nil {
		t.Fatalf("expected StorageClassNotFoundError, got %T: %v", err, err)
	}
	if scErr.Name != storageClassName {
		t.Fatalf("StorageClassNotFoundError.Name = %q, want %q", scErr.Name, storageClassName)
	}
}

func TestVolumePopulatorWaitForPVCSelectedNodePropagatesOtherStorageClassGetErrors(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	rawErr := apierrors.NewForbidden(schema.GroupResource{Group: storagev1.GroupName, Resource: "storageclasses"}, "blocked-sc", nil)
	reconciler := &VolumePopulatorReconciler{
		Client: storageClassGetErrorClient{
			Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
			err:    rawErr,
		},
		Scheme: scheme,
	}
	storageClassName := "blocked-sc"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-pvc", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
		},
	}

	wait, nodeName, err := reconciler.waitForPVCSelectedNode(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	if wait {
		t.Fatal("expected raw StorageClass GET error to stop waiting and return the error")
	}
	if nodeName != "" {
		t.Fatalf("nodeName = %q, want empty", nodeName)
	}
	if err != rawErr {
		t.Fatalf("err = %v, want raw error %v", err, rawErr)
	}
	if scErr, ok := dprestore.IsStorageClassNotFoundError(err); ok || scErr != nil {
		t.Fatalf("raw GET error must not match StorageClassNotFoundError, got (%v, %v)", scErr, ok)
	}
}

type storageClassGetErrorClient struct {
	client.Client
	err error
}

func (c storageClassGetErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*storagev1.StorageClass); ok {
		return c.err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func TestClusterRestoreHandlesStorageClassNotFoundWithinBoundedWindow(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	clusterRestore := newStorageClassTestClusterRestore()
	targetRef := &dpv1alpha1.ClusterRestoreTargetClusterRef{
		Name:      "target-cluster",
		Namespace: "default",
		UID:       types.UID("target-cluster-uid"),
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}
	pvc := newStorageClassTestPVC()

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "missing-sc"},
		targetRef,
		pvc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != storageClassRequeueAfter {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, storageClassRequeueAfter)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != dpv1alpha1.ClusterRestorePhaseRestoring {
		t.Fatalf("phase = %q, want %q", latest.Status.Phase, dpv1alpha1.ClusterRestorePhaseRestoring)
	}
	if latest.Status.TargetClusterRef == nil || latest.Status.TargetClusterRef.Name != targetRef.Name {
		t.Fatalf("target ref not preserved: %#v", latest.Status.TargetClusterRef)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if other := meta.FindStatusCondition(latest.Status.Conditions, "OtherControllerCondition"); other == nil || other.Reason != "KeepMe" {
		t.Fatalf("expected unrelated conditions to be preserved, got %#v", latest.Status.Conditions)
	}
	if cond.Status != metav1.ConditionFalse ||
		cond.Reason != reasonWaitingForStorageClass ||
		cond.ObservedGeneration != latest.Generation {
		t.Fatalf("unexpected Ready condition: %#v", cond)
	}
	if cond.LastTransitionTime.IsZero() {
		t.Fatalf("expected LastTransitionTime to mark wait start: %#v", cond)
	}
	assertContainsAll(t, cond.Message,
		`StorageClass "missing-sc" not found`,
		"ClusterRestore default/restore-session",
		"target PVC default/data-target-cluster-mysql-0",
		"pre-create or sync the StorageClass",
	)
}

func TestClusterRestoreKeepsStorageClassWaitStartAcrossRetries(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	waitStart := metav1.NewTime(time.Now().Add(-2 * time.Minute).Truncate(time.Second))
	clusterRestore := newStorageClassTestClusterRestore()
	pvc := newStorageClassTestPVC()
	clusterRestore.Status.Conditions = []metav1.Condition{{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             reasonWaitingForStorageClass,
		Message:            storageClassWaitingMessage(clusterRestore, "missing-sc", pvc),
		ObservedGeneration: clusterRestore.Generation,
		LastTransitionTime: waitStart,
	}, {
		Type:               "OtherControllerCondition",
		Status:             metav1.ConditionTrue,
		Reason:             "KeepMe",
		Message:            "owned by another status writer",
		ObservedGeneration: 1,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second)),
	}}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "missing-sc"},
		clusterRestoreTargetRef(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default", UID: types.UID("target-cluster-uid")}}),
		pvc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != storageClassRequeueAfter {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, storageClassRequeueAfter)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if !cond.LastTransitionTime.Time.Equal(waitStart.Time) {
		t.Fatalf("LastTransitionTime = %s, want original wait start %s", cond.LastTransitionTime.Time, waitStart.Time)
	}
	if cond.Reason != reasonWaitingForStorageClass {
		t.Fatalf("Reason = %q, want %q", cond.Reason, reasonWaitingForStorageClass)
	}
	if other := meta.FindStatusCondition(latest.Status.Conditions, "OtherControllerCondition"); other == nil || other.Reason != "KeepMe" {
		t.Fatalf("expected unrelated conditions to be preserved, got %#v", latest.Status.Conditions)
	}
}

func TestClusterRestoreResetsStorageClassWaitStartForDifferentMissingStorageClass(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	previousWaitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - time.Minute).Truncate(time.Second))
	clusterRestore := newStorageClassTestClusterRestore()
	previousPVC := newStorageClassTestPVC()
	clusterRestore.Status.Conditions = []metav1.Condition{{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             reasonWaitingForStorageClass,
		Message:            storageClassWaitingMessage(clusterRestore, "old-missing-sc", previousPVC),
		ObservedGeneration: clusterRestore.Generation,
		LastTransitionTime: previousWaitStart,
	}}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "new-missing-sc"},
		clusterRestoreTargetRef(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default", UID: types.UID("target-cluster-uid")}}),
		previousPVC,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != storageClassRequeueAfter {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, storageClassRequeueAfter)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if cond.Reason != reasonWaitingForStorageClass {
		t.Fatalf("Reason = %q, want %q", cond.Reason, reasonWaitingForStorageClass)
	}
	if !cond.LastTransitionTime.After(previousWaitStart.Time) {
		t.Fatalf("LastTransitionTime = %s, should reset after previous wait start %s", cond.LastTransitionTime.Time, previousWaitStart.Time)
	}
	assertContainsAll(t, cond.Message, `StorageClass "new-missing-sc" not found`)
}

func TestClusterRestoreKeepsStorageClassWaitStartForSameStorageClassAcrossDifferentPVCs(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	waitStart := metav1.NewTime(time.Now().Add(-2 * time.Minute).Truncate(time.Second))
	clusterRestore := newStorageClassTestClusterRestore()
	previousPVC := newStorageClassTestPVC()
	currentPVC := newStorageClassTestPVC()
	currentPVC.Name = "data-target-cluster-mysql-1"
	clusterRestore.Status.Conditions = []metav1.Condition{{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             reasonWaitingForStorageClass,
		Message:            storageClassWaitingMessage(clusterRestore, "missing-sc", previousPVC),
		ObservedGeneration: clusterRestore.Generation,
		LastTransitionTime: waitStart,
	}}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "missing-sc"},
		clusterRestoreTargetRef(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default", UID: types.UID("target-cluster-uid")}}),
		currentPVC,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != storageClassRequeueAfter {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, storageClassRequeueAfter)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if !cond.LastTransitionTime.Time.Equal(waitStart.Time) {
		t.Fatalf("LastTransitionTime = %s, want original wait start %s", cond.LastTransitionTime.Time, waitStart.Time)
	}
	assertContainsAll(t, cond.Message,
		`StorageClass "missing-sc" not found`,
		"target PVC default/data-target-cluster-mysql-1",
	)
}

func TestClusterRestoreResetsStorageClassWaitStartForDifferentClusterRestore(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	previousWaitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - time.Minute).Truncate(time.Second))
	clusterRestore := newStorageClassTestClusterRestore()
	otherClusterRestore := clusterRestore.DeepCopy()
	otherClusterRestore.Name = "other-restore-session"
	pvc := newStorageClassTestPVC()
	clusterRestore.Status.Conditions = []metav1.Condition{{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             reasonWaitingForStorageClass,
		Message:            storageClassWaitingMessage(otherClusterRestore, "missing-sc", pvc),
		ObservedGeneration: clusterRestore.Generation,
		LastTransitionTime: previousWaitStart,
	}}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "missing-sc"},
		clusterRestoreTargetRef(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default", UID: types.UID("target-cluster-uid")}}),
		pvc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != storageClassRequeueAfter {
		t.Fatalf("RequeueAfter = %s, want %s", result.RequeueAfter, storageClassRequeueAfter)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if !cond.LastTransitionTime.After(previousWaitStart.Time) {
		t.Fatalf("LastTransitionTime = %s, should reset after previous wait start %s", cond.LastTransitionTime.Time, previousWaitStart.Time)
	}
	assertContainsAll(t, cond.Message,
		`StorageClass "missing-sc" not found`,
		"ClusterRestore default/restore-session",
	)
}

func TestClusterRestoreFailsStorageClassNotFoundAfterBoundedWindow(t *testing.T) {
	scheme := newClusterRestoreUnitScheme(t)
	waitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - time.Second).Truncate(time.Second))
	clusterRestore := newStorageClassTestClusterRestore()
	pvc := newStorageClassTestPVC()
	clusterRestore.Status.Conditions = []metav1.Condition{{
		Type:               dpv1alpha1.ClusterRestoreReadyCondition,
		Status:             metav1.ConditionFalse,
		Reason:             reasonWaitingForStorageClass,
		Message:            storageClassWaitingMessage(clusterRestore, "missing-sc", pvc),
		ObservedGeneration: clusterRestore.Generation,
		LastTransitionTime: waitStart,
	}, {
		Type:               "OtherControllerCondition",
		Status:             metav1.ConditionTrue,
		Reason:             "KeepMe",
		Message:            "owned by another status writer",
		ObservedGeneration: 1,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second)),
	}}
	recorder := record.NewFakeRecorder(8)
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: recorder,
	}

	result, err := reconciler.handleStorageClassNotFound(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		clusterRestore,
		&dprestore.StorageClassNotFoundError{Name: "missing-sc"},
		clusterRestoreTargetRef(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default", UID: types.UID("target-cluster-uid")}}),
		pvc,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("result = %#v, want reconciled empty result", result)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != dpv1alpha1.ClusterRestorePhaseFailed {
		t.Fatalf("phase = %q, want %q", latest.Status.Phase, dpv1alpha1.ClusterRestorePhaseFailed)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil {
		t.Fatalf("Ready condition missing: %#v", latest.Status.Conditions)
	}
	if other := meta.FindStatusCondition(latest.Status.Conditions, "OtherControllerCondition"); other == nil || other.Reason != "KeepMe" {
		t.Fatalf("expected unrelated conditions to be preserved, got %#v", latest.Status.Conditions)
	}
	if cond.Reason != reasonStorageClassMissing || cond.Status != metav1.ConditionFalse {
		t.Fatalf("unexpected Ready condition: %#v", cond)
	}
	assertContainsAll(t, cond.Message,
		`StorageClass "missing-sc" not found after 5m0s`,
		"re-apply ClusterRestore default/restore-session",
	)

	select {
	case event := <-recorder.Events:
		assertContainsAll(t, event,
			corev1.EventTypeWarning,
			dprestore.ReasonRestoreFailed,
			`StorageClass "missing-sc" not found after 5m0s`,
		)
	default:
		t.Fatal("expected Warning event for bounded StorageClass wait timeout")
	}
}

func newClusterRestoreUnitScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func newStorageClassTestClusterRestore() *dpv1alpha1.ClusterRestore {
	return &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "restore-session",
			Namespace:  "default",
			UID:        types.UID("restore-session-uid"),
			Generation: 2,
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: "backup", Namespace: "default"},
		},
		Status: dpv1alpha1.ClusterRestoreStatus{
			Phase: dpv1alpha1.ClusterRestorePhaseRestoring,
			Conditions: []metav1.Condition{{
				Type:               "OtherControllerCondition",
				Status:             metav1.ConditionTrue,
				Reason:             "KeepMe",
				Message:            "owned by another status writer",
				ObservedGeneration: 1,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second)),
			}},
		},
	}
}

func newStorageClassTestPVC() *corev1.PersistentVolumeClaim {
	storageClassName := "missing-sc"
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-target-cluster-mysql-0",
			Namespace: "default",
			Annotations: map[string]string{
				dptypes.VolumeSourceAnnotationKey: "data",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClassName,
		},
	}
}

func assertContainsAll(t *testing.T, got string, wantSubstrings ...string) {
	t.Helper()
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to contain %q", got, want)
		}
	}
}

func TestClusterRestoreRequiresBackupSnapshotWithoutTargetTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-backup",
			Namespace: "backup-ns",
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "target-ns",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: backup.Name, Namespace: backup.Namespace},
		},
	}
	reconciler := &ClusterRestoreReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	if _, err := reconciler.buildTargetCluster(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore, backup, clusterRestore.Spec.RestoreTime); err == nil {
		t.Fatal("expected missing backup snapshot to fail when target template is omitted")
	}
}

func TestClusterRestoreFailsWhenTargetClusterAlreadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: "backup"},
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "default",
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
		},
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRestore.Spec.TargetClusterName,
			Namespace: clusterRestore.Namespace,
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore, backup, cluster).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(clusterRestore)}); err != nil {
		t.Fatal(err)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != dpv1alpha1.ClusterRestorePhaseFailed {
		t.Fatalf("expected ClusterRestore to fail when target exists, got %q", latest.Status.Phase)
	}
}

func TestClusterRestoreFailsWhenTargetClusterCreateIsInvalid(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-backup",
			Namespace: "default",
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: backup.Name, Namespace: backup.Namespace},
			TargetClusterTemplate: &dpv1alpha1.ClusterRestoreTargetClusterTemplate{
				Spec: appsv1.ClusterSpec{
					ClusterDef:        "mysql",
					TerminationPolicy: appsv1.Delete,
					ComponentSpecs: []appsv1.ClusterComponentSpec{{
						Name:     "mysql",
						Replicas: 1,
					}},
				},
			},
		},
	}
	invalidErr := apierrors.NewInvalid(
		schema.GroupKind{Group: appsv1.GroupVersion.Group, Kind: "Cluster"},
		clusterRestore.Spec.TargetClusterName,
		field.ErrorList{field.Required(field.NewPath("spec", "componentSpecs", "0", "componentDef"), "test invalid cluster")},
	)
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore, backup).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   invalidClusterCreateClient{Client: baseClient, err: invalidErr},
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(clusterRestore)}); err != nil {
		t.Fatal(err)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err := baseClient.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != dpv1alpha1.ClusterRestorePhaseFailed {
		t.Fatalf("expected ClusterRestore to fail on invalid target Cluster, got %q", latest.Status.Phase)
	}
	cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
	if cond == nil || cond.Message == "" {
		t.Fatalf("expected failure condition message, got %#v", latest.Status.Conditions)
	}
}

type invalidClusterCreateClient struct {
	client.Client
	err error
}

func (c invalidClusterCreateClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*appsv1.Cluster); ok {
		return c.err
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestClusterRestoreTerminalPhaseDoesNotReconcileAgain(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: "deleted-backup"},
		},
		Status: dpv1alpha1.ClusterRestoreStatus{
			Phase: dpv1alpha1.ClusterRestorePhaseCompleted,
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore).
		Build()
	reconciler := &ClusterRestoreReconciler{
		Client:   cli,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(8),
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(clusterRestore)}); err != nil {
		t.Fatal(err)
	}

	latest := &dpv1alpha1.ClusterRestore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Phase != dpv1alpha1.ClusterRestorePhaseCompleted {
		t.Fatalf("expected terminal phase to stay completed, got %q", latest.Status.Phase)
	}
}

func TestClusterRestoreFailsWhenAccountSourceSecretAlreadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	sourceCluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "default"},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:           "mysql",
				SystemAccounts: []appsv1.ComponentSystemAccount{{Name: "root"}},
			}},
		},
	}
	clusterSnapshot, err := json.Marshal(sourceCluster)
	if err != nil {
		t.Fatal(err)
	}
	encryptedPassword, err := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey)).Encrypt([]byte("restored-password"))
	if err != nil {
		t.Fatal(err)
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "default",
			Annotations: map[string]string{
				constant.ClusterSnapshotAnnotationKey:         string(clusterSnapshot),
				constant.EncryptedSystemAccountsAnnotationKey: `{"mysql":{"root":"` + encryptedPassword + `"}}`,
			},
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef:         dpv1alpha1.ClusterRestoreBackupRef{Name: backup.Name, Namespace: backup.Namespace},
		},
	}
	conflictingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRestoreSourceAccountSecretName(clusterRestore.Spec.TargetClusterName, clusterRestore.Name),
			Namespace: clusterRestore.Namespace,
			Annotations: map[string]string{
				dptypes.ClusterRestoreUIDAnnotationKey: "another-restore-uid",
			},
		},
		Data: map[string][]byte{
			constant.AccountPasswdForSecret: []byte("do-not-overwrite"),
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conflictingSecret).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli, Scheme: scheme}

	_, err = reconciler.buildTargetCluster(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore, backup, clusterRestore.Spec.RestoreTime)
	if err == nil {
		t.Fatal("expected conflicting account source Secret to fail")
	}
	latestSecret := &corev1.Secret{}
	if getErr := cli.Get(context.Background(), client.ObjectKeyFromObject(conflictingSecret), latestSecret); getErr != nil {
		t.Fatal(getErr)
	}
	if string(latestSecret.Data[constant.AccountPasswdForSecret]) != "do-not-overwrite" {
		t.Fatalf("expected conflicting Secret not to be overwritten, got %q", string(latestSecret.Data[constant.AccountPasswdForSecret]))
	}
}

func TestClusterRestoreCleansUpAccountSourceSecretRefs(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
	}
	sourceSecretName := clusterRestoreSourceAccountSecretName("target-cluster", clusterRestore.Name)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-cluster",
			Namespace: "default",
		},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql",
				SystemAccounts: []appsv1.ComponentSystemAccount{
					{
						Name: "root",
						SecretRef: &appsv1.ProvisionSecretRef{
							Name:      sourceSecretName,
							Namespace: "default",
							Password:  clusterRestoreSourceAccountPasswordKey("mysql", "root"),
						},
					},
					{
						Name: "metrics",
						SecretRef: &appsv1.ProvisionSecretRef{
							Name:      "external-secret",
							Namespace: "default",
							Password:  constant.AccountPasswdForSecret,
						},
					},
				},
			}},
			Shardings: []appsv1.ClusterSharding{{
				Name: "shard",
				Template: appsv1.ClusterComponentSpec{
					SystemAccounts: []appsv1.ComponentSystemAccount{{
						Name: "root",
						SecretRef: &appsv1.ProvisionSecretRef{
							Name:      sourceSecretName,
							Namespace: "default",
							Password:  clusterRestoreSourceAccountPasswordKey("shard", "root"),
						},
					}},
				},
			}},
		},
	}
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecretName,
			Namespace: "default",
			Annotations: map[string]string{
				dptypes.ClusterRestoreUIDAnnotationKey: string(clusterRestore.UID),
			},
		},
		Data: map[string][]byte{
			clusterRestoreSourceAccountPasswordKey("mysql", "root"): []byte("restored-password"),
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, sourceSecret).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli, Scheme: scheme}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	if err := reconciler.cleanupClusterRestoreAccountRefs(reqCtx, clusterRestore, cluster); err != nil {
		t.Fatal(err)
	}
	latestCluster := &appsv1.Cluster{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), latestCluster); err != nil {
		t.Fatal(err)
	}
	if latestCluster.Spec.ComponentSpecs[0].SystemAccounts[0].SecretRef != nil {
		t.Fatalf("expected restore source secretRef to be removed, got %#v", latestCluster.Spec.ComponentSpecs[0].SystemAccounts[0].SecretRef)
	}
	if latestCluster.Spec.ComponentSpecs[0].SystemAccounts[1].SecretRef == nil ||
		latestCluster.Spec.ComponentSpecs[0].SystemAccounts[1].SecretRef.Name != "external-secret" {
		t.Fatalf("expected external secretRef to be preserved, got %#v", latestCluster.Spec.ComponentSpecs[0].SystemAccounts[1].SecretRef)
	}
	if latestCluster.Spec.Shardings[0].Template.SystemAccounts[0].SecretRef != nil {
		t.Fatalf("expected sharding restore source secretRef to be removed, got %#v", latestCluster.Spec.Shardings[0].Template.SystemAccounts[0].SecretRef)
	}
	if err := reconciler.cleanupSourceSystemAccounts(reqCtx, clusterRestore, latestCluster); err != nil {
		t.Fatal(err)
	}
	latestSecret := &corev1.Secret{}
	err := cli.Get(context.Background(), client.ObjectKeyFromObject(sourceSecret), latestSecret)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected source secret to be deleted, got err=%v secret=%#v", err, latestSecret)
	}
}

func TestClusterRestoreWaitsForClusterRunningBeforeAccountSourceCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
	}
	sourceSecretName := clusterRestoreSourceAccountSecretName("target-cluster", clusterRestore.Name)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-cluster",
			Namespace: "default",
		},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql",
				SystemAccounts: []appsv1.ComponentSystemAccount{{
					Name: "root",
					SecretRef: &appsv1.ProvisionSecretRef{
						Name:      sourceSecretName,
						Namespace: "default",
						Password:  clusterRestoreSourceAccountPasswordKey("mysql", "root"),
					},
				}},
			}},
		},
		Status: appsv1.ClusterStatus{Phase: appsv1.CreatingClusterPhase},
	}
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecretName,
			Namespace: "default",
			Annotations: map[string]string{
				dptypes.ClusterRestoreUIDAnnotationKey: string(clusterRestore.UID),
			},
		},
		Data: map[string][]byte{
			clusterRestoreSourceAccountPasswordKey("mysql", "root"): []byte("restored-password"),
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&dpv1alpha1.ClusterRestore{}).
		WithObjects(clusterRestore, cluster, sourceSecret).
		Build()
	reconciler := &ClusterRestoreReconciler{Client: cli, Scheme: scheme}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	if _, err := reconciler.updateClusterRestoreStatusFromRestores(reqCtx, clusterRestore, cluster, clusterRestoreTargetRef(cluster), nil); err != nil {
		t.Fatal(err)
	}
	latestRestore := &dpv1alpha1.ClusterRestore{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(clusterRestore), latestRestore); err != nil {
		t.Fatal(err)
	}
	if latestRestore.Status.Phase != dpv1alpha1.ClusterRestorePhaseRestoring {
		t.Fatalf("expected restore to wait for Cluster running, got %q", latestRestore.Status.Phase)
	}
	latestCluster := &appsv1.Cluster{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), latestCluster); err != nil {
		t.Fatal(err)
	}
	if latestCluster.Spec.ComponentSpecs[0].SystemAccounts[0].SecretRef == nil {
		t.Fatal("expected account source secretRef to be retained while Cluster is not running")
	}
	latestSecret := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(sourceSecret), latestSecret); err != nil {
		t.Fatal(err)
	}
}

func TestClusterRestoreFiltersPVCsByRestoreUID(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			BackupRef: dpv1alpha1.ClusterRestoreBackupRef{Name: "backup"},
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
	}
	currentPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "current-pvc",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.ClusterRestoreLabelKey: clusterRestore.Name,
			},
			Annotations: map[string]string{
				dptypes.ClusterRestoreUIDAnnotationKey: string(clusterRestore.UID),
				dptypes.VolumeSourceAnnotationKey:      "data",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: dprestore.BackupDataSourceRef(backup.Name)},
	}
	stalePVC := currentPVC.DeepCopy()
	stalePVC.Name = "stale-pvc"
	stalePVC.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] = "another-restore-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, currentPVC, stalePVC).Build()
	reconciler := &ClusterRestoreReconciler{Client: cli, Scheme: scheme}

	items, err := reconciler.listClusterRestorePVCs(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].pvc.Name != currentPVC.Name {
		t.Fatalf("expected only current restore PVC, got %#v", items)
	}
}

func TestClusterRestoreInternalRestoresCheckRestoreUID(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "default",
			UID:       types.UID("restore-session-uid"),
		},
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "target-cluster", Namespace: "default"},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}},
		},
	}
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"}}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-target-cluster-mysql-0",
			Namespace: "default",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
	}
	reconciler := &ClusterRestoreReconciler{Scheme: scheme}
	item := backupDataSourcePVC{
		pvc:    pvc,
		backup: backup,
		options: backupDataSourceContext{
			sourceTargetName: "mysql",
		},
	}
	conflictingRestore := reconciler.buildInternalPostReadyRestore(clusterRestore, cluster, item, "mysql")
	conflictingRestore.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] = "another-restore-uid"
	conflictingRestore.OwnerReferences = nil
	currentRestore := conflictingRestore.DeepCopy()
	currentRestore.Name = "current-post-ready"
	currentRestore.Annotations[dptypes.ClusterRestoreUIDAnnotationKey] = string(clusterRestore.UID)
	currentRestore.Labels[dptypes.ClusterRestoreLabelKey] = clusterRestore.Name
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conflictingRestore, currentRestore).Build()
	reconciler.Client = cli

	if _, err := reconciler.ensureInternalPostReadyRestores(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore, cluster, []backupDataSourcePVC{item}); err == nil {
		t.Fatal("expected conflicting post-ready Restore to fail")
	}
	restores, err := reconciler.listInternalPostReadyRestores(intctrlutil.RequestCtx{Ctx: context.Background()}, clusterRestore)
	if err != nil {
		t.Fatal(err)
	}
	if len(restores) != 1 || restores[0].Name != currentRestore.Name {
		t.Fatalf("expected only current restore-owned post-ready Restore, got %#v", restores)
	}
}

func TestBackupRestoreInProgressFindsClusterRestoreAcrossNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-backup",
			Namespace: "backup-ns",
		},
	}
	clusterRestore := &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-session",
			Namespace: "target-ns",
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: "target-cluster",
			BackupRef: dpv1alpha1.ClusterRestoreBackupRef{
				Name:      backup.Name,
				Namespace: backup.Namespace,
			},
		},
		Status: dpv1alpha1.ClusterRestoreStatus{
			Phase: dpv1alpha1.ClusterRestorePhaseRestoring,
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(backup, clusterRestore).
		Build()
	reconciler := &BackupReconciler{Client: cli}

	inProgress, err := reconciler.checkRestoreInProgress(intctrlutil.RequestCtx{Ctx: context.Background()}, backup)
	if err != nil {
		t.Fatal(err)
	}
	if !inProgress {
		t.Fatal("expected backup to be marked in restore while referenced by ClusterRestore in another namespace")
	}
}

func TestRequiredPolicyForSourcePod(t *testing.T) {
	if policy := requiredPolicyForSourcePod(""); policy == nil || policy.DataRestorePolicy != dpv1alpha1.OneToOneRestorePolicy {
		t.Fatalf("expected default one-to-one policy, got %#v", policy)
	}
	policy := requiredPolicyForSourcePod("source-pod-0")
	if policy == nil || policy.DataRestorePolicy != dpv1alpha1.OneToManyRestorePolicy {
		t.Fatalf("expected one-to-many policy, got %#v", policy)
	}
	if policy.SourceOfOneToMany == nil || policy.SourceOfOneToMany.TargetPodName != "source-pod-0" {
		t.Fatalf("expected source target pod name to be preserved, got %#v", policy)
	}
}
