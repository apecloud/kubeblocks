/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instanceset

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestInstanceStatusRetainsOwnedPVCCleanup(t *testing.T) {
	for _, tt := range []struct {
		name        string
		policy      kbappsv1.PersistentVolumeClaimRetentionPolicyType
		owner       types.UID
		terminating bool
		want        bool
	}{
		{name: "delete", policy: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, owner: "its", want: true},
		{name: "retain", policy: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, owner: "its"},
		{name: "terminating retain", policy: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, owner: "its", terminating: true, want: true},
		{name: "no owner", policy: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{name: "wrong owner", policy: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, owner: "other"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "its"},
				Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0), PersistentVolumeClaimRetentionPolicy: &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: tt.policy}}}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-demo-0", Labels: map[string]string{constant.KBAppPodNameLabelKey: "demo-0"}}}
			if tt.owner != "" {
				pvc.OwnerReferences = []metav1.OwnerReference{{UID: tt.owner, Controller: ptr.To(true)}}
			}
			if tt.terminating {
				pvc.DeletionTimestamp = ptr.To(metav1.Now())
			}
			if err := tree.Add(pvc); err != nil {
				t.Fatal(err)
			}
			if err := setInstanceStatus(tree, its, nil); err != nil {
				t.Fatal(err)
			}
			status := its.FindInstanceStatus("demo-0")
			if !tt.want {
				if status != nil {
					t.Fatalf("PVC without cleanup obligation retained status: %#v", status)
				}
				return
			}
			if status == nil || status.Provisioned || status.DesiredState != workloads.InstanceDesiredStateReleased || status.CurrentState != workloads.InstanceCurrentStateAbsent {
				t.Fatalf("owned cleanup resource did not retain Released+Absent independently of provisioning: %#v", status)
			}
		})
	}
}
