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

package instanceset2

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestRevisionUpdatePublishesCompleteInstanceViewBeforeObservedGeneration(t *testing.T) {
	its := revisionTestInstanceSet()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if its.Status.ObservedGeneration != its.Generation {
		t.Fatalf("ObservedGeneration was not advanced: %#v", its.Status)
	}
	if len(its.Status.InstanceStatus) != 1 {
		t.Fatalf("complete InstanceStatus was not published: %#v", its.Status.InstanceStatus)
	}
	status := its.Status.InstanceStatus[0]
	if status.TemplateName == nil || *status.TemplateName != "" || status.DesiredState != workloads.InstanceDesiredStateActive || status.CurrentPodState != workloads.CurrentPodStateAbsent {
		t.Fatalf("unexpected InstanceStatus: %#v", status)
	}
}

func TestRevisionUpdateDoesNotAdvanceObservedGenerationOnInvalidInstanceView(t *testing.T) {
	its := revisionTestInstanceSet()
	its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "demo-0"}, {PodName: "demo-0"}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err == nil {
		t.Fatal("expected invalid duplicate status to fail")
	}
	if its.Status.ObservedGeneration == its.Generation {
		t.Fatal("ObservedGeneration advanced despite an incomplete instance view")
	}
	if len(its.Status.InstanceStatus) != 2 {
		t.Fatal("invalid build partially replaced InstanceStatus")
	}
}

func revisionTestInstanceSet() *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 2},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		},
	}
}
