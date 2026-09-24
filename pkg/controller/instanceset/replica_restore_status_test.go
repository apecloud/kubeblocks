/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package instanceset

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/replicarestore"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestBuildReplicaRestoreStatusCountsInstancesAfterEveryVCT(t *testing.T) {
	group := "dataprotection.kubeblocks.io"
	intent := &appsv1.ReplicaRestoreProjection{
		StartOrdinal: 3, EndOrdinal: 5, Fingerprint: "restore-1",
		SourceRef: corev1.TypedObjectReference{APIGroup: &group, Kind: "Backup", Name: "backup"},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql", Namespace: "default", Generation: 6,
			Labels: map[string]string{constant.KBAppComponentLabelKey: "mysql"}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](5), ReplicaRestore: intent,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "scratch"}},
			},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	for _, instanceName := range []string{"mysql-3", "mysql-4"} {
		for _, vct := range its.Spec.VolumeClaimTemplates {
			if instanceName == "mysql-4" && vct.Name == "scratch" {
				continue
			}
			pvc := completedReplicaRestorePVC(intent, its, instanceName, vct)
			if err := tree.Add(pvc); err != nil {
				t.Fatal(err)
			}
		}
	}

	status, err := buildReplicaRestoreStatus(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	if status.Phase != appsv1.ReplicaRestorePending || status.RestoredReplicas != 1 || status.TargetReplicas != 5 {
		t.Fatalf("missing one VCT should leave one target instance restored: %#v", status)
	}

	missing := completedReplicaRestorePVC(intent, its, "mysql-4", its.Spec.VolumeClaimTemplates[1])
	if err := tree.Add(missing); err != nil {
		t.Fatal(err)
	}
	status, err = buildReplicaRestoreStatus(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	if status.Phase != appsv1.ReplicaRestoreCompleted || status.RestoredReplicas != 2 {
		t.Fatalf("all target VCTs should complete both target instances: %#v", status)
	}

	its.Spec.VolumeClaimTemplates = nil
	status, err = buildReplicaRestoreStatus(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	if status.Phase != appsv1.ReplicaRestorePending || status.RestoredReplicas != 0 {
		t.Fatalf("targets without effective VCTs must not complete: %#v", status)
	}
}

func completedReplicaRestorePVC(intent *appsv1.ReplicaRestoreProjection, its *workloads.InstanceSet,
	instanceName string, vct corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: intctrlutil.ComposePVCName(vct, its.Name, instanceName), Namespace: its.Namespace,
		Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: vct.Name},
	}}
	replicarestore.ApplyToPVC(pvc, intent, instanceName)
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type: corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore), Status: corev1.ConditionTrue,
	}}
	return pvc
}
