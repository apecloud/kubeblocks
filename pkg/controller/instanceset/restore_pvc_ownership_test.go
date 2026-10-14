/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestInstanceSetAdoptsMarkedRestorePVC(t *testing.T) {
	model.AddScheme(corev1.AddToScheme)
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", Namespace: "default", UID: "its-uid"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-0", Namespace: "default", UID: "pvc-uid",
		Annotations: map[string]string{constant.RestorePVCComponentUIDAnnotationKey: "component-uid"}}}
	if err := tryTakeOverExternalPVC(its, pvc); err != nil {
		t.Fatal(err)
	}
	if !metav1.IsControlledBy(pvc, its) {
		t.Fatal("InstanceSet did not adopt the marked restore claim")
	}
	if err := tryTakeOverExternalPVC(its, pvc); err != nil {
		t.Fatal(err)
	}
	if len(pvc.OwnerReferences) != 1 || pvc.Annotations[constant.RestorePVCComponentUIDAnnotationKey] != "component-uid" {
		t.Fatalf("adoption is not idempotent: %#v", pvc.ObjectMeta)
	}
}

func TestInstanceSetMarkedRestorePVCRetention(t *testing.T) {
	model.AddScheme(corev1.AddToScheme)
	model.AddScheme(workloads.AddToScheme)
	for _, retain := range []bool{false, true} {
		name := "delete"
		if retain {
			name = "retain"
		}
		t.Run(name, func(t *testing.T) {
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", Namespace: "default", UID: "its-uid"}}
			if retain {
				its.Spec.PersistentVolumeClaimRetentionPolicy = &appsv1.PersistentVolumeClaimRetentionPolicy{WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType}
			}
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-0", Namespace: "default", UID: "pvc-uid",
				Annotations: map[string]string{constant.RestorePVCComponentUIDAnnotationKey: "component-uid"}}}
			if err := tryTakeOverExternalPVC(its, pvc); err != nil {
				t.Fatal(err)
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			if err := tree.Add(pvc); err != nil {
				t.Fatal(err)
			}
			if _, err := NewDeletionReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			claims := tree.List(&corev1.PersistentVolumeClaim{})
			if retain {
				if len(claims) != 1 || len(claims[0].GetOwnerReferences()) != 0 {
					t.Fatal("Retain must keep the marked PVC and detach the deleted ITS")
				}
			} else if len(claims) != 0 {
				t.Fatal("default ITS deletion must still delete the marked PVC")
			}
		})
	}
}
