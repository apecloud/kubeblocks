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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestParseReplicasNMaxUnavailable(t *testing.T) {
	tests := []struct {
		name                   string
		totalReplicas          int
		replicas               intstr.IntOrString
		maxUnavailable         intstr.IntOrString
		expectedReplicas       int
		expectedMaxUnavailable int
	}{
		{
			name:                   "round replicas up and maxUnavailable down",
			totalReplicas:          3,
			replicas:               intstr.FromString("34%"),
			maxUnavailable:         intstr.FromString("34%"),
			expectedReplicas:       2,
			expectedMaxUnavailable: 1,
		},
		{
			name:                   "keep maxUnavailable at least one",
			totalReplicas:          3,
			replicas:               intstr.FromString("10%"),
			maxUnavailable:         intstr.FromString("10%"),
			expectedReplicas:       1,
			expectedMaxUnavailable: 1,
		},
		{
			name:                   "keep exact percentage results",
			totalReplicas:          4,
			replicas:               intstr.FromString("50%"),
			maxUnavailable:         intstr.FromString("50%"),
			expectedReplicas:       2,
			expectedMaxUnavailable: 2,
		},
		{
			name:                   "keep absolute values",
			totalReplicas:          3,
			replicas:               intstr.FromInt32(2),
			maxUnavailable:         intstr.FromInt32(1),
			expectedReplicas:       2,
			expectedMaxUnavailable: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &tt.replicas,
					MaxUnavailable: &tt.maxUnavailable,
				},
			}

			replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(strategy, tt.totalReplicas)
			if err != nil {
				t.Fatalf("parse rolling update quotas: %v", err)
			}
			if replicas != tt.expectedReplicas || maxUnavailable != tt.expectedMaxUnavailable {
				t.Fatalf("quotas = (%d, %d), want (%d, %d)",
					replicas, maxUnavailable, tt.expectedReplicas, tt.expectedMaxUnavailable)
			}
		})
	}
}

func TestUpdateReconcilerKeepsConvergingInstanceInRollingWindow(t *testing.T) {
	tests := []struct {
		name           string
		convergingName string
		pendingName    string
	}{
		{name: "converging Instance sorts first", convergingName: "demo-1", pendingName: "demo-0"},
		{name: "pending Instance sorts first", convergingName: "demo-0", pendingName: "demo-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its := its2InstanceStatusSet(2)
			its.Generation = 2
			its.Spec.Template.Spec.Containers[0].Image = "mysql:new"
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)

			desired, names, err := buildDesiredInstancesByName(tree, its)
			if err != nil {
				t.Fatal(err)
			}
			targetRevisions := make(map[string]string, len(names))
			for _, name := range names {
				targetRevisions[name] = getInstanceRevision(desired[name])
			}
			its.Status.UpdateRevisions, err = revisionmap.Encode(targetRevisions)
			if err != nil {
				t.Fatal(err)
			}

			readyAndAvailable := []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
			}
			converging := desired[tt.convergingName].DeepCopy()
			converging.Generation = 2
			converging.Status = workloads.InstanceStatus2{
				ObservedGeneration: 2,
				CurrentState:       workloads.InstanceCurrentStatePresent,
				UpToDate:           false,
				Conditions:         readyAndAvailable,
			}
			pending := desired[tt.pendingName].DeepCopy()
			pending.Spec.Template.Spec.Containers[0].Image = "mysql:old"
			stampInstanceRevision(pending)
			pending.Generation = 1
			pending.Status = workloads.InstanceStatus2{
				ObservedGeneration: 1,
				CurrentState:       workloads.InstanceCurrentStatePresent,
				UpToDate:           true,
				Conditions:         readyAndAvailable,
			}
			if err := tree.Add(converging, pending); err != nil {
				t.Fatal(err)
			}

			if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			object, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: pending.Name}})
			if err != nil {
				t.Fatal(err)
			}
			got := object.(*workloads.Instance)
			if image := got.Spec.Template.Spec.Containers[0].Image; image != "mysql:old" {
				t.Fatalf("maxUnavailable=1 admitted a second unconverged Instance: image=%q", image)
			}
			if !intctrlutil.IsInstanceReady(converging) || !intctrlutil.IsInstanceAvailable(converging) {
				t.Fatal("fixture must remain runtime healthy while desired-state convergence is pending")
			}
		})
	}
}
