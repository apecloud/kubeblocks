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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestIsInstanceUpdated(t *testing.T) {
	latestInst := newRevisionTestInstance("test-its-0")
	latestRevision := stampInstanceRevision(latestInst)
	its := &workloads.InstanceSet{
		Status: workloads.InstanceSetStatus{
			UpdateRevisions: map[string]string{
				latestInst.Name: latestRevision,
			},
		},
	}

	tests := []struct {
		name string
		inst *workloads.Instance
		want bool
	}{
		{
			name: "true when instance revision annotation matches",
			inst: func() *workloads.Instance {
				inst := newRevisionTestInstance("test-its-0")
				inst.Annotations[instanceSetRevisionAnnotationKey] = latestRevision
				return inst
			}(),
			want: true,
		},
		{
			name: "false when instance status is not up to date",
			inst: func() *workloads.Instance {
				inst := newRevisionTestInstance("test-its-0")
				inst.Annotations[instanceSetRevisionAnnotationKey] = latestRevision
				inst.Status.UpToDate = false
				return inst
			}(),
			want: false,
		},
		{
			name: "false when revision annotation differs",
			inst: func() *workloads.Instance {
				inst := newRevisionTestInstance("test-its-0")
				inst.Annotations[instanceSetRevisionAnnotationKey] = "stale"
				return inst
			}(),
			want: false,
		},
		{
			name: "false when revision annotation is missing",
			inst: func() *workloads.Instance {
				inst := newRevisionTestInstance("test-its-0")
				delete(inst.Annotations, instanceSetRevisionAnnotationKey)
				return inst
			}(),
			want: false,
		},
		{
			name: "false when instance status has not observed latest generation",
			inst: func() *workloads.Instance {
				inst := newRevisionTestInstance("test-its-0")
				inst.Annotations[instanceSetRevisionAnnotationKey] = latestRevision
				inst.Status.ObservedGeneration = 1
				return inst
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInstanceUpdated(its, tt.inst); got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBuildInstanceRevisionIgnoresNonIntentState(t *testing.T) {
	inst := newRevisionTestInstance("test-its-0")
	inst.Labels = map[string]string{"managed-label": "desired"}
	inst.Annotations[constant.KubeBlocksGenerationKey] = "1"
	inst.Annotations[instanceSetRevisionAnnotationKey] = "revision-1"
	inst.Annotations["managed-annotation"] = "desired"
	inst.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{
		{
			Service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-its-headless",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
				},
				Spec: corev1.ServiceSpec{
					ClusterIP:  "10.0.0.1",
					ClusterIPs: []string{"10.0.0.1"},
				},
			},
		},
	}
	inst.Spec.LifecycleActions = &workloads.LifecycleActions{
		TemplateVars: map[string]string{"instance": "test-its-0"},
	}
	revision := buildInstanceRevision(inst)

	inst.Labels["managed-label"] = "changed"
	inst.Annotations[constant.KubeBlocksGenerationKey] = "2"
	inst.Annotations[instanceSetRevisionAnnotationKey] = "revision-2"
	inst.Annotations["managed-annotation"] = "changed"
	inst.Spec.InstanceAssistantObjects[0].Service.Spec.ClusterIP = "10.0.0.2"
	inst.Spec.InstanceAssistantObjects[0].Service.Spec.ClusterIPs = []string{"10.0.0.2"}
	inst.Spec.InstanceAssistantObjects[0].Service.ResourceVersion = "2"
	inst.Spec.LifecycleActions.TemplateVars["instance"] = "test-its-1"
	inst.Spec.Template.CreationTimestamp = metav1.Now()
	inst.Spec.Template.ResourceVersion = "2"
	inst.Spec.Template.Generation = 2
	inst.Spec.VolumeClaimTemplates[0].CreationTimestamp = metav1.Now()
	inst.Spec.VolumeClaimTemplates[0].ResourceVersion = "2"
	inst.Spec.VolumeClaimTemplates[0].Generation = 2
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected non-intent state to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.Template.Labels["pod-label"] = "changed"
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected pod template label change to alter revision")
	}
}

func TestBuildInstanceByTemplateStampsRevisionAnnotation(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-its",
			Namespace:  "default",
			Generation: 3,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
		},
	}
	template := &instancetemplate.InstanceTemplateExt{
		Name: "tpl",
		PodTemplateSpec: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"pod-label": "desired"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "postgres", Image: "postgres:16"}},
			},
		},
	}

	inst, err := buildInstanceByTemplate(kubebuilderx.NewObjectTree(), "test-its-0", template, its)
	if err != nil {
		t.Fatalf("buildInstanceByTemplate() error = %v", err)
	}
	if got := getInstanceRevision(inst); got == "" {
		t.Fatalf("expected desired instance to carry revision annotation")
	} else if want := buildInstanceRevision(inst); got != want {
		t.Fatalf("expected revision annotation %s, got %s", want, got)
	}
}

func TestStatusReconcilerReadsCurrentRevisionFromInstanceAnnotation(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-its",
			Namespace: "default",
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
		},
	}
	inst := newRevisionTestInstance("test-its-0")
	revision := stampInstanceRevision(inst)
	its.Status.UpdateRevisions = map[string]string{inst.Name: revision}
	its.Status.UpdateRevision = revision

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if err := tree.Add(inst); err != nil {
		t.Fatalf("add instance: %v", err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("status reconcile: %v", err)
	}
	if got := its.Status.CurrentRevisions[inst.Name]; got != revision {
		t.Fatalf("expected current revision %s, got %s", revision, got)
	}
	if got := its.Status.UpdatedReplicas; got != 1 {
		t.Fatalf("expected updated replicas 1, got %d", got)
	}
	if got := its.Status.CurrentRevision; got != revision {
		t.Fatalf("expected current revision summary %s, got %s", revision, got)
	}
}

func TestStatusReconcilerDoesNotFallbackToLiveHashWhenRevisionAnnotationMissing(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-its",
			Namespace: "default",
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
		},
	}
	desired := newRevisionTestInstance("test-its-0")
	revision := stampInstanceRevision(desired)
	its.Status.UpdateRevisions = map[string]string{desired.Name: revision}
	its.Status.UpdateRevision = revision

	inst := desired.DeepCopy()
	delete(inst.Annotations, instanceSetRevisionAnnotationKey)

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if err := tree.Add(inst); err != nil {
		t.Fatalf("add instance: %v", err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("status reconcile: %v", err)
	}
	if got := its.Status.CurrentRevisions[inst.Name]; got != "" {
		t.Fatalf("expected empty current revision, got %s", got)
	}
	if got := its.Status.UpdatedReplicas; got != 0 {
		t.Fatalf("expected missing revision annotation to keep updated replicas at 0, got %d", got)
	}
}

func newRevisionTestInstance(name string) *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: 2,
			Labels: map[string]string{
				instancetemplate.TemplateNameLabelKey: "tpl",
			},
			Annotations: map[string]string{},
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"pod-label": "desired"},
					Annotations: map[string]string{"pod-annotation": "desired"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "postgres", Image: "postgres:16"}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "data",
						Labels:      map[string]string{"pvc-label": "desired"},
						Annotations: map[string]string{"pvc-annotation": "desired"},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				},
			},
		},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 2,
			UpToDate:           true,
		},
	}
}
