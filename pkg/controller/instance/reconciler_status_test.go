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

package instance

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestStatusReconcilerPublishesInstanceCurrentState(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want workloads.InstanceCurrentState
	}{
		{name: "absent", want: workloads.InstanceCurrentStateAbsent},
		{name: "present", pod: &corev1.Pod{}, want: workloads.InstanceCurrentStatePresent},
		{name: "terminating", pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time}}}, want: workloads.InstanceCurrentStateTerminating},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &workloads.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", Generation: 1},
				Status: workloads.InstanceStatus2{
					CurrentRevision: "stale",
					UpToDate:        true,
					Ready:           true,
					Available:       true,
					Role:            "leader",
					VolumeExpansion: true,
					Configs:         []workloads.InstanceConfigStatus{{Name: "stale"}},
				},
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(inst)
			if tt.pod != nil {
				tt.pod.Name = inst.Name
				tt.pod.Namespace = inst.Namespace
				if err := tree.Add(tt.pod); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if inst.Status.CurrentState != tt.want {
				t.Fatalf("expected current state %q, got %#v", tt.want, inst.Status)
			}
			if tt.want != workloads.InstanceCurrentStatePresent && (inst.Status.CurrentRevision != "" || inst.Status.UpToDate || inst.Status.Ready || inst.Status.Available || inst.Status.Role != "" || inst.Status.VolumeExpansion || inst.Status.Configs != nil) {
				t.Fatalf("non-Present instance retained current runtime fields: %#v", inst.Status)
			}
		})
	}
}
