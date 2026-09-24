/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package dataprotection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestReplicaRestoreRequiresMatchingDataVolume(t *testing.T) {
	for _, kind := range []string{workloads.InstanceSetKind, "Instance"} {
		for _, hasDataVolume := range []bool{false, true} {
			name := kind + "/no matching volume"
			if hasDataVolume {
				name = kind + "/auxiliary before data PVC creation"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(scheme))
				require.NoError(t, workloads.AddToScheme(scheme))
				require.NoError(t, dpv1alpha1.AddToScheme(scheme))
				backup, actionSet := restoreBackupObjects()
				require.NotNil(t, actionSet.Spec.Restore.PrepareData)
				pvc := newPVCForRestoreDecision("logs", "mysql", "")
				pvc.UID = "replica-pvc"
				pvc.Annotations[constant.RestorePurposeAnnotationKey] = constant.RestorePurposeReplica
				pvc.Spec = corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					}},
					DataSourceRef: &corev1.TypedObjectReference{
						APIGroup: ptr.To(dptypes.DataprotectionAPIGroup), Kind: dptypes.BackupKind, Name: backup.Name,
					},
				}
				names := []string{"logs"}
				if hasDataVolume {
					names = append(names, "data")
				}
				meta := metav1.ObjectMeta{Namespace: pvc.Namespace, Name: "mysql", UID: "owner-uid"}
				var owner client.Object
				if kind == workloads.InstanceSetKind {
					its := &workloads.InstanceSet{ObjectMeta: meta}
					for _, name := range names {
						its.Spec.VolumeClaimTemplates = append(its.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name}})
					}
					owner = its
				} else {
					inst := &workloads.Instance{ObjectMeta: meta}
					for _, name := range names {
						inst.Spec.VolumeClaimTemplates = append(inst.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: name}})
					}
					owner = inst
				}
				pvc.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: workloads.GroupVersion.String(), Kind: kind,
					Name: owner.GetName(), UID: owner.GetUID(), Controller: ptr.To(true),
				}}
				worker := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: pvc.Namespace, Name: "worker"}}
				for key, value := range map[string]string{
					dptypes.CfgKeyWorkerServiceAccountName: worker.Name,
					dptypes.CfgKeyWorkerClusterRoleName:    "worker-role",
				} {
					previous := viper.Get(key)
					viper.Set(key, value)
					t.Cleanup(func() { viper.Set(key, previous) })
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc).
					WithObjects(backup, actionSet, worker, owner, pvc).Build()
				vp := &VolumePopulatorReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
				// Repeat using a fresh reconciler to verify failure survives retries.
				for i := 0; i < 2; i++ {
					_, err := vp.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
					require.NoError(t, err)
					vp = &VolumePopulatorReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
				}
				require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc))
				pvcs := &corev1.PersistentVolumeClaimList{}
				require.NoError(t, cli.List(ctx, pvcs))
				restores := &dpv1alpha1.RestoreList{}
				require.NoError(t, cli.List(ctx, restores))
				require.Empty(t, restores.Items)
				if hasDataVolume {
					require.Len(t, pvcs.Items, 2, "auxiliary volume can provision before the data PVC exists")
				} else {
					require.Len(t, pvcs.Items, 1, "no helper volume may be provisioned for an empty restore")
					condition := findPVCConditionByType(pvc, appsv1.ConditionTypeRestore)
					require.NotNil(t, condition)
					require.Equal(t, corev1.ConditionFalse, condition.Status)
					require.Contains(t, condition.Message, "no workload volume matching Backup")
				}
			})
		}
	}
}
