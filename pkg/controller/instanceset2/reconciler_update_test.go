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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestUpdateReconcilerAllowsStorageOnlyUpdateWhenInstanceUnavailable(t *testing.T) {
	tree, oldInstances := buildUpdateReconcilerTree("10Gi")
	tree.SetRoot(buildUpdateReconcilerInstanceSet("11Gi"))
	oldInstances[2].Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("11Gi")
	stampInstanceRevision(oldInstances[2])
	oldInstances[2].Status.ObservedGeneration = 1

	if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile update: %v", err)
	}

	for i := 0; i < 3; i++ {
		got, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("mysql-%d", i), Namespace: "default"}})
		if err != nil {
			t.Fatalf("get instance %d: %v", i, err)
		}
		inst := got.(*workloads.Instance)
		storage := inst.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
		if storage.String() != "11Gi" {
			t.Fatalf("instance %s storage = %s, want 11Gi", inst.Name, storage.String())
		}
	}
}

func TestUpdateReconcilerStillGatesPodTemplateUpdateWhenInstanceUnavailable(t *testing.T) {
	tree, oldInstances := buildUpdateReconcilerTree("10Gi")
	its := buildUpdateReconcilerInstanceSet("10Gi")
	its.Spec.Template.Spec.Containers[0].Image = "mysql:8.4"
	tree.SetRoot(its)
	oldInstances[2].Spec.Template.Spec.Containers[0].Image = "mysql:8.4"
	stampInstanceRevision(oldInstances[2])
	oldInstances[2].Status.ObservedGeneration = 1

	if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile update: %v", err)
	}

	got, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "mysql-0", Namespace: "default"}})
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	inst := got.(*workloads.Instance)
	if inst.Spec.Template.Spec.Containers[0].Image != "mysql:8.0" {
		t.Fatalf("image = %s, want update gated at mysql:8.0", inst.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestIsStorageOnlyUpdate(t *testing.T) {
	oldInst := buildUpdateReconcilerInstanceSet("10Gi")
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(oldInst)
	itsExt, err := instancetemplate.BuildInstanceSetExt(oldInst, tree)
	if err != nil {
		t.Fatalf("build instance set ext: %v", err)
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		t.Fatalf("build name builder: %v", err)
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		t.Fatalf("build name map: %v", err)
	}
	current, err := buildInstanceByTemplate(tree, "mysql-0", nameToTemplateMap["mysql-0"], oldInst)
	if err != nil {
		t.Fatalf("build current instance: %v", err)
	}

	newIts := buildUpdateReconcilerInstanceSet("11Gi")
	newTree := kubebuilderx.NewObjectTree()
	newTree.SetRoot(newIts)
	newItsExt, err := instancetemplate.BuildInstanceSetExt(newIts, newTree)
	if err != nil {
		t.Fatalf("build new instance set ext: %v", err)
	}
	newNameBuilder, err := instancetemplate.NewPodNameBuilder(newItsExt, nil)
	if err != nil {
		t.Fatalf("build new name builder: %v", err)
	}
	newNameToTemplateMap, err := newNameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		t.Fatalf("build new name map: %v", err)
	}
	newInst, err := buildInstanceByTemplate(newTree, "mysql-0", newNameToTemplateMap["mysql-0"], newIts)
	if err != nil {
		t.Fatalf("build storage instance: %v", err)
	}
	if !isStorageOnlyUpdate(current, newInst) {
		t.Fatal("expected VCT-only change to be storage-only")
	}

	newInst.Spec.Template.Spec.Containers[0].Image = "mysql:8.4"
	if isStorageOnlyUpdate(current, newInst) {
		t.Fatal("expected pod template change not to be storage-only")
	}
}

func buildUpdateReconcilerTree(storage string) (*kubebuilderx.ObjectTree, []*workloads.Instance) {
	tree := kubebuilderx.NewObjectTree()
	its := buildUpdateReconcilerInstanceSet(storage)
	tree.SetRoot(its)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		panic(err)
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		panic(err)
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		panic(err)
	}
	instances := make([]*workloads.Instance, 0, 3)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("mysql-%d", i)
		inst, err := buildInstanceByTemplate(tree, name, nameToTemplateMap[name], its)
		if err != nil {
			panic(err)
		}
		markUpdateReconcilerInstanceReady(inst)
		if err := tree.Add(inst); err != nil {
			panic(err)
		}
		instances = append(instances, inst)
	}
	return tree, instances
}

func buildUpdateReconcilerInstanceSet(storage string) *workloads.InstanceSet {
	its := builder.NewInstanceSetBuilder("default", "mysql").
		SetReplicas(3).
		SetTemplate(corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "mysql"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "mysql", Image: "mysql:8.0"}},
			},
		}).
		SetSelectorMatchLabel(map[string]string{"app": "mysql"}).
		SetRoles([]workloads.ReplicaRole{{Name: "primary", UpdatePriority: 2}, {Name: "secondary", UpdatePriority: 1}}).
		SetMemberUpdateStrategy(ptrTo(workloads.BestEffortParallelUpdateStrategy)).
		SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
			Type: appsv1.RollingUpdateStrategyType,
			RollingUpdate: &workloads.RollingUpdate{
				MaxUnavailable: ptrTo(intstr.FromInt32(1)),
			},
		}).
		SetVolumeClaimTemplates(corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(storage)},
				},
			},
		}).
		GetObject()
	its.Generation = 1
	return its
}

func markUpdateReconcilerInstanceReady(inst *workloads.Instance) {
	inst.Generation = 1
	inst.Status.ObservedGeneration = 1
	inst.Status.Conditions = []metav1.Condition{
		{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
		{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
	}
	inst.Status.Role = "secondary"
	if inst.Name == "mysql-2" {
		inst.Status.Role = "primary"
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
