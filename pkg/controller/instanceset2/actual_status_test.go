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

package instanceset2

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestITS2StatusCopiesChildFactsAndRetainsAbsentChild(t *testing.T) {
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	location := "worker-a"
	storage := &workloads.InstanceStorageIdentity{Complete: true, Volumes: []workloads.InstanceVolumeIdentity{{Name: "data", Claim: &workloads.InstanceObjectReference{Name: "actual-claim", Namespace: "default", UID: "pvc-uid", Cluster: &location}, VolumeName: "actual-pv"}}}
	child := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "instance-uid"}, Status: workloads.InstanceStatus2{CurrentState: workloads.InstanceCurrentStateAbsent, Storage: storage}}
	if err := setInstanceStatus(tree, its, []*workloads.Instance{child}); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(child.Name)
	if status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.CurrentState != workloads.InstanceCurrentStateAbsent || status.Pod != nil || status.Storage.Volumes[0].Claim.UID != "pvc-uid" {
		t.Fatalf("absent extant child facts lost: %#v", status)
	}
	status.Storage.Volumes[0].Claim.UID = "mutated"
	if child.Status.Storage.Volumes[0].Claim.UID != "pvc-uid" {
		t.Fatal("parent aliases child storage")
	}
	child.Status.CurrentState = workloads.InstanceCurrentStatePresent
	child.Status.Pod = &workloads.InstanceObjectReference{Name: "demo-0", Namespace: "default", UID: "pod-uid", Cluster: &location}
	if err := setInstanceStatus(tree, its, []*workloads.Instance{child}); err != nil {
		t.Fatal(err)
	}
	if its.FindInstanceStatus(child.Name).Pod.UID != "pod-uid" {
		t.Fatal("child Pod reference was inferred instead of copied")
	}
	if len(tree.List(&corev1.Pod{})) != 0 || len(tree.List(&corev1.PersistentVolumeClaim{})) != 0 {
		t.Fatal("adapter took ownership of child resources")
	}
	if err := setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	if its.FindInstanceStatus(child.Name) != nil {
		t.Fatal("gone child retained forever")
	}
}

func TestITS2InterpretsChildLocalReferencesAtActualWorker(t *testing.T) {
	local := ""
	inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-a"}}, Status: workloads.InstanceStatus2{Pod: &workloads.InstanceObjectReference{Name: "demo-0", Namespace: "default", UID: "pod", Cluster: &local}, Storage: &workloads.InstanceStorageIdentity{Complete: true, Volumes: []workloads.InstanceVolumeIdentity{{Name: "data", Claim: &workloads.InstanceObjectReference{Name: "data-demo-0", Namespace: "default", UID: "pvc", Cluster: &local}, VolumeName: "pv"}}, EphemeralPod: &workloads.InstanceObjectReference{Name: "demo-0", Namespace: "default", UID: "pod", Cluster: &local}}}}
	resource := childResourceObservation(context.Background(), inst)
	if *resource.Pod.Cluster != "worker-a" || *resource.Storage.Volumes[0].Claim.Cluster != "worker-a" || *resource.Storage.EphemeralPod.Cluster != "worker-a" {
		t.Fatal("worker local references pointed to control cluster")
	}
	if *inst.Status.Pod.Cluster != "" || *inst.Status.Storage.Volumes[0].Claim.Cluster != "" {
		t.Fatal("normalization mutated child facts")
	}
	inst.Status.Storage.Volumes[0].Claim.Cluster = nil
	unknown := childResourceObservation(context.Background(), inst)
	if unknown.Storage.Volumes[0].Claim.Cluster != nil {
		t.Fatal("unknown child location was guessed")
	}
	inst.Annotations = nil
	unplaced := childResourceObservation(context.Background(), inst)
	if unplaced.Pod.Cluster != nil || unplaced.Storage.Complete {
		t.Fatal("missing owner location was guessed local")
	}
	inst.Status.Storage.EphemeralPod = nil
	inst.Status.Storage.Volumes[0].Claim.Cluster = ptr.To("worker-b")
	explicit := childResourceObservation(context.Background(), inst)
	if !explicit.Storage.Complete || *explicit.Storage.Volumes[0].Claim.Cluster != "worker-b" {
		t.Fatal("explicit child storage location required unavailable Instance origin")
	}

}
