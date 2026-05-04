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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestBackupDataSourceRestoreCreatesPostReadyRestoreBeforeCleanup(t *testing.T) {
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

	restoreOptions := dprestore.DefaultRestoreOptions()
	restoreOptions.VolumeSource = "data"
	restoreOptions.SourceTargetName = "mysql"
	annotations, err := dprestore.SetRestoreOptions(nil, restoreOptions)
	if err != nil {
		t.Fatal(err)
	}
	cluster := &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restore-cluster",
			Namespace: "default",
			UID:       types.UID("restore-cluster-uid"),
		},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: "mysql",
				VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{
					Name:        "data",
					Annotations: annotations,
					Spec: corev1.PersistentVolumeClaimSpec{
						DataSourceRef: dprestore.BackupDataSourceRef("backup"),
					},
				}},
			}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-restore-cluster-mysql-0",
			Namespace: "default",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    cluster.Name,
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "default",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	reconciler := &BackupDataSourceRestoreReconciler{
		Client: cli,
		Scheme: scheme,
	}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	item := backupDataSourcePVC{pvc: pvc, backup: backup, options: restoreOptions}

	restores, err := reconciler.ensureInternalPostReadyRestores(reqCtx, cluster, []backupDataSourcePVC{item})
	if err != nil {
		t.Fatal(err)
	}
	if len(restores) != 1 {
		t.Fatalf("expected one internal restore, got %d", len(restores))
	}
	internalRestore := &dpv1alpha1.Restore{}
	if err = cli.Get(context.Background(), client.ObjectKey{Name: restores[0].Name, Namespace: cluster.Namespace}, internalRestore); err != nil {
		t.Fatal(err)
	}

	if err = reconciler.cleanupClusterRestoreInputs(reqCtx, cluster); err != nil {
		t.Fatal(err)
	}
	latestCluster := &appsv1.Cluster{}
	if err = cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), latestCluster); err != nil {
		t.Fatal(err)
	}
	vct := latestCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0]
	if vct.Spec.DataSourceRef != nil {
		t.Fatalf("expected dataSourceRef to be cleaned, got %#v", vct.Spec.DataSourceRef)
	}
	if _, ok := vct.Annotations[dptypes.RestoreOptionsAnnotationKey]; ok {
		t.Fatalf("expected restore options annotation to be cleaned")
	}

	restores, err = reconciler.listInternalPostReadyRestores(reqCtx, latestCluster)
	if err != nil {
		t.Fatal(err)
	}
	if len(restores) != 1 {
		t.Fatalf("expected internal restore to survive cleanup, got %d", len(restores))
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
