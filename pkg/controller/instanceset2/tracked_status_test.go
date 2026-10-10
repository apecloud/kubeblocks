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
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestInstanceStatusTracksOwnedChildSeparatelyFromRuntime(t *testing.T) {
	for _, state := range []workloads.InstanceCurrentState{"", workloads.InstanceCurrentStateAbsent, workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating} {
		for _, tracked := range []*bool{nil, ptr.To(false), ptr.To(true)} {
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "its"},
				Spec:   workloads.InstanceSetSpec{Replicas: ptr.To[int32](0)},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: "demo-0", DataLoaded: tracked, MemberJoined: tracked}}}}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", OwnerReferences: []metav1.OwnerReference{{UID: "its", Controller: ptr.To(true)}}},
				Status: workloads.InstanceStatus2{CurrentState: state, Ready: true, Available: true}}
			if err := tree.Add(inst); err != nil {
				t.Fatal(err)
			}
			if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
				t.Fatal(err)
			}
			status := its.FindInstanceStatus(inst.Name)
			wantRuntime := state == workloads.InstanceCurrentStatePresent || state == workloads.InstanceCurrentStateTerminating
			if status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.Provisioned != wantRuntime || !reflect.DeepEqual(status.DataLoaded, tracked) || !reflect.DeepEqual(status.MemberJoined, tracked) {
				t.Fatalf("state=%s facts=%v: unexpected tracked status: %#v", state, tracked, status)
			}
			wantState := state
			if state == "" {
				wantState = workloads.InstanceCurrentStateAbsent
			}
			if status.CurrentState != wantState || status.Ready != (state == workloads.InstanceCurrentStatePresent) || status.Available != (state == workloads.InstanceCurrentStatePresent) {
				t.Fatalf("state=%s: child existence changed runtime status: %#v", state, status)
			}
			if tracked != nil && (status.DataLoaded == tracked || status.MemberJoined == tracked) {
				t.Fatal("caller retained previous status pointer aliases")
			}
		}
	}
}

func TestAbsentChildRequiresInstanceSetOwnershipToRetainStatus(t *testing.T) {
	for _, owned := range []bool{false, true} {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", UID: "its"}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0)}}
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(its)
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0"}}
		if owned {
			inst.OwnerReferences = []metav1.OwnerReference{{UID: "other", Controller: ptr.To(true)}}
		}
		if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
			t.Fatal(err)
		}
		if len(its.Status.InstanceStatus) != 0 {
			t.Fatalf("unowned absent child retained an identity: %#v", its.Status.InstanceStatus)
		}
	}
}

func TestInstanceStatusRetainsAssociatedMulticlusterChildWithoutRuntime(t *testing.T) {
	for _, tt := range []struct {
		name         string
		multicluster bool
		change       func(*workloads.Instance)
		wantRetained bool
	}{
		{name: "associated multicluster child", multicluster: true, wantRetained: true},
		{name: "local ownerless child"},
		{name: "different InstanceSetName", multicluster: true, change: func(inst *workloads.Instance) { inst.Spec.InstanceSetName = "other" }},
		{name: "different InstanceSet label", multicluster: true, change: func(inst *workloads.Instance) { inst.Labels[WorkloadsInstanceLabelKey] = "other" }},
	} {
		for _, state := range []workloads.InstanceCurrentState{"", workloads.InstanceCurrentStateAbsent} {
			for i, tracked := range []*bool{nil, ptr.To(false)} {
				t.Run(fmt.Sprintf("%s/state=%s/facts=%d", tt.name, state, i), func(t *testing.T) {
					its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "its"},
						Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1)}}
					if tt.multicluster {
						its.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "cluster-a"}
					}
					tree := kubebuilderx.NewObjectTree()
					tree.SetRoot(its)
					desired, _, err := buildDesiredInstancesByName(tree, its)
					if err != nil {
						t.Fatal(err)
					}
					inst := desired["demo-0"]
					if inst == nil {
						t.Fatal("builder did not produce the desired child")
					}
					if tt.multicluster && metav1.GetControllerOf(inst) != nil {
						t.Fatal("multicluster builder unexpectedly set a local controller reference")
					}
					inst.OwnerReferences = nil
					inst.Status.CurrentState = state
					if tt.change != nil {
						tt.change(inst)
					}
					if err := tree.Add(inst); err != nil {
						t.Fatal(err)
					}
					its.Spec.Replicas = ptr.To[int32](0)
					its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: inst.Name, DataLoaded: tracked, MemberJoined: tracked}}
					if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
						t.Fatal(err)
					}
					status := its.FindInstanceStatus(inst.Name)
					if !tt.wantRetained {
						if status != nil {
							t.Fatalf("unassociated ownerless child retained status: %#v", status)
						}
						return
					}
					if status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.CurrentState != workloads.InstanceCurrentStateAbsent || status.Provisioned || !reflect.DeepEqual(status.DataLoaded, tracked) || !reflect.DeepEqual(status.MemberJoined, tracked) {
						t.Fatalf("associated multicluster child did not retain tracked facts independently of runtime: %#v", status)
					}
				})
			}
		}
	}
}
