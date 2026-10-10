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
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func TestLegacyStatusPublishesActualMountedUIDAndRetainsCleanup(t *testing.T) {
	its, tree, pods := newLegacyDefaultInstanceStatusFixture(t, 1)
	tree.Context = multicluster.IntoContext(context.Background(), "worker-a")
	pod := pods["demo-0"]
	pod.UID = "actual-pod"
	pod.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "actual-claim"}}}}
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "actual-claim", Namespace: its.Namespace, UID: "actual-pvc", Labels: map[string]string{constant.KBAppPodNameLabelKey: pod.Name, constant.VolumeClaimTemplateNameLabelKey: "data"}}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "actual-pv"}}
	if err := tree.Add(claim); err != nil {
		t.Fatal(err)
	}
	if err := NewStatusReconciler(nil).setInstanceStatus(tree, its, []*corev1.Pod{pod}); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(pod.Name)
	if status.Pod.UID != "actual-pod" || *status.Pod.Cluster != "worker-a" || !status.Storage.Complete || status.Storage.Volumes[0].Claim.UID != "actual-pvc" {
		t.Fatalf("actual identity not published: %#v", status)
	}
	its.Spec.Replicas = ptr.To[int32](0)
	if err := NewStatusReconciler(nil).setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	status = its.FindInstanceStatus(pod.Name)
	if status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.CurrentState != workloads.InstanceCurrentStateAbsent || status.Pod != nil {
		t.Fatal("actual delete-policy claim obligation pruned")
	}
	if err := tree.Delete(claim); err != nil {
		t.Fatal(err)
	}
	if err := NewStatusReconciler(nil).setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	if its.FindInstanceStatus(pod.Name) != nil {
		t.Fatal("settled historical row was not pruned")
	}
}

type missingClaimReader struct {
	client.Reader
	fail bool
}

func (r *missingClaimReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if r.fail {
		return errors.New("worker unavailable")
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}
func TestLegacyCleanupMissingListRetriesUntilPositiveAbsence(t *testing.T) {
	its := legacyInstanceStatusSet(0)
	its.Spec.Instances = nil
	its.UID = "its-owned"
	location := "worker-a"
	ref := &workloads.InstanceObjectReference{Name: "data-demo-0", Namespace: its.Namespace, UID: "pvc-owned", Cluster: &location}
	its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "demo-0", DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateAbsent, Storage: &workloads.InstanceStorageIdentity{Complete: true, Volumes: []workloads.InstanceVolumeIdentity{{Name: "data", OwnerUID: its.UID, Claim: ref, VolumeName: "pv"}}}}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	tree.Context = multicluster.IntoContext(context.Background(), "worker-a,worker-b")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reader := &missingClaimReader{Reader: fake.NewClientBuilder().WithScheme(scheme).Build(), fail: true}
	for i := 0; i < 2; i++ {
		r := NewStatusReconciler(reader)
		if err := r.setInstanceStatus(tree, its, nil); err != nil {
			t.Fatal(err)
		}
		status := its.FindInstanceStatus("demo-0")
		if status == nil || !r.ObservationPending() || status.Storage.Complete || status.Storage.Volumes[0].Claim.UID != "pvc-owned" {
			t.Fatal("read failure pruned or lost tracked owned claim")
		}
	}
	reader.fail = false
	r := NewStatusReconciler(reader)
	if err := r.setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	if its.FindInstanceStatus("demo-0") != nil || r.ObservationPending() {
		t.Fatal("positive NotFound did not settle cleanup")
	}
}
func TestLegacyCleanupReplacementUsesActualOwnership(t *testing.T) {
	for _, owned := range []bool{false, true} {
		its := legacyInstanceStatusSet(0)
		its.Spec.Instances = nil
		its.UID = "its-owned"
		location := "worker-a"
		its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "demo-0", Storage: &workloads.InstanceStorageIdentity{Complete: true, Volumes: []workloads.InstanceVolumeIdentity{{Name: "data", OwnerUID: its.UID, Claim: &workloads.InstanceObjectReference{Name: "data-demo-0", Namespace: its.Namespace, UID: "old", Cluster: &location}, VolumeName: "pv-old"}}}}}
		replacement := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-demo-0", Namespace: its.Namespace, UID: types.UID("new"), Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: "data"}}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-new"}}
		if owned {
			replacement.OwnerReferences = []metav1.OwnerReference{{APIVersion: workloads.GroupVersion.String(), Kind: "InstanceSet", Name: its.Name, UID: its.UID, Controller: ptr.To(true)}}
		}
		scheme := runtime.NewScheme()
		if err := corev1.AddToScheme(scheme); err != nil {
			t.Fatal(err)
		}
		reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(replacement).Build()
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(its)
		tree.Context = multicluster.IntoContext(context.Background(), "worker-a,worker-b")
		if err := NewStatusReconciler(reader).setInstanceStatus(tree, its, nil); err != nil {
			t.Fatal(err)
		}
		status := its.FindInstanceStatus("demo-0")
		if owned {
			if status == nil || !status.Storage.Complete || status.Storage.Volumes[0].Claim.UID != "new" || *status.Storage.Volumes[0].Claim.Cluster != "worker-a" {
				t.Fatal("current owned replacement cleanup lost")
			}
		} else if status != nil {
			t.Fatal("foreign replacement created cleanup obligation")
		}
		if len(tree.List(&corev1.PersistentVolumeClaim{})) != 0 {
			t.Fatal("read-only cleanup claim entered modifiable tree")
		}
	}
}
