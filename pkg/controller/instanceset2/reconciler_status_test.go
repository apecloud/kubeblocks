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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	instctrl "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
)

func TestStatusReconcilerAggregatesInstanceRestoreConditions(t *testing.T) {
	newFixture := func() (*workloads.InstanceSet, *kubebuilderx.ObjectTree) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 3},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{
					Name: "data",
					Annotations: map[string]string{
						constant.RestoreSourceKindAnnotationKey: "Backup",
					},
				}}},
			},
		}
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(its)
		return its, tree
	}
	newInstance := func(status metav1.ConditionStatus) *workloads.Instance {
		return &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"},
			Status: workloads.InstanceStatus2{Conditions: []metav1.Condition{{
				Type:    string(workloads.InstanceRestore),
				Status:  status,
				Message: "restore result",
			}}},
		}
	}

	t.Run("waits for the desired Instance", func(t *testing.T) {
		its, tree := newFixture()
		cond, err := buildRestoreCondition(tree, its, nil)
		if err != nil {
			t.Fatal(err)
		}
		if cond == nil || cond.Status != metav1.ConditionUnknown || cond.Reason != workloads.ReasonRestoreRunning {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})

	t.Run("publishes completed", func(t *testing.T) {
		its, tree := newFixture()
		cond, err := buildRestoreCondition(tree, its, []*workloads.Instance{newInstance(metav1.ConditionTrue)})
		if err != nil {
			t.Fatal(err)
		}
		if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != workloads.ReasonRestoreCompleted {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})

	t.Run("publishes failure first", func(t *testing.T) {
		its, tree := newFixture()
		cond, err := buildRestoreCondition(tree, its, []*workloads.Instance{newInstance(metav1.ConditionFalse)})
		if err != nil {
			t.Fatal(err)
		}
		if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != workloads.ReasonRestoreFailed {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})

	t.Run("keeps a terminal failure", func(t *testing.T) {
		its, tree := newFixture()
		meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
			Type:   string(workloads.InstanceRestore),
			Status: metav1.ConditionFalse,
			Reason: workloads.ReasonRestoreFailed,
		})
		if err := (&statusReconciler{}).reconcileRestoreCondition(tree, its, []*workloads.Instance{newInstance(metav1.ConditionTrue)}); err != nil {
			t.Fatal(err)
		}
		cond := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceRestore))
		if cond == nil || cond.Status != metav1.ConditionFalse {
			t.Fatalf("terminal Restore condition was overwritten: %#v", cond)
		}
	})
}

func TestSetInstanceStatusReadsCurrentStateFromInstance(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 3},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		},
		Status: workloads.InstanceSetStatus{ObservedGeneration: 3},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Generation: 1},
		Spec:       workloads.InstanceSpec{InstanceTemplateName: ""},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 1,
			CurrentState:       workloads.InstanceCurrentStateAbsent,
			UpdateRevision:     "pod-revision",
			UpToDate:           true,
			Configs:            []workloads.InstanceConfigStatus{{Name: "config"}},
			VolumeExpansion:    true,
		},
	}
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	desiredPodRevision, err := instctrl.BuildPodRevision(desiredInstances[inst.Name])
	if err != nil {
		t.Fatal(err)
	}
	inst.Status.UpdateRevision = desiredPodRevision
	instanceSpecRevision := stampInstanceRevision(inst)
	its.Status.UpdateRevisions = map[string]string{inst.Name: instanceSpecRevision}

	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	if len(its.Status.InstanceStatus) != 1 {
		t.Fatalf("unexpected status: %#v", its.Status.InstanceStatus)
	}
	status := its.Status.InstanceStatus[0]
	if status.TemplateName == nil || *status.TemplateName != "" || status.DesiredState != workloads.InstanceDesiredStateActive || status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatalf("Instance was not Active+Absent: %#v", status)
	}
	if status.UpdateRevision != inst.Status.UpdateRevision || status.CurrentRevision != "" || status.UpToDate || status.Configs != nil || status.VolumeExpansion {
		t.Fatalf("Absent Instance retained runtime fields: %#v", status)
	}

	inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
	inst.Status.CurrentRevision = inst.Status.UpdateRevision
	inst.Status.Ready = true
	inst.Status.Available = true
	inst.Status.Conditions = []metav1.Condition{
		{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
		{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
		{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue},
	}
	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	status = its.Status.InstanceStatus[0]
	if status.CurrentState != workloads.InstanceCurrentStatePresent || status.CurrentRevision != inst.Status.CurrentRevision || status.UpdateRevision != inst.Status.UpdateRevision || !status.UpToDate || !status.Ready || !status.Available || !status.Failed || len(status.Configs) != 1 || !status.VolumeExpansion {
		t.Fatalf("Present Instance did not refresh runtime fields: %#v", status)
	}

	inst.Status.UpToDate = false
	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	status = its.Status.InstanceStatus[0]
	if status.UpToDate || !status.Ready || !status.Available {
		t.Fatalf("Ready and Available must be independent from UpToDate: %#v", status)
	}

	inst.Status.CurrentState = workloads.InstanceCurrentStateTerminating
	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	status = its.Status.InstanceStatus[0]
	if status.CurrentState != workloads.InstanceCurrentStateTerminating || status.CurrentRevision != inst.Status.CurrentRevision || status.UpdateRevision != inst.Status.UpdateRevision || status.UpToDate || status.Ready || status.Available || status.Failed || status.Configs != nil || status.VolumeExpansion {
		t.Fatalf("Terminating Instance retained current runtime fields: %#v", status)
	}
}

func TestSetInstanceStatusRetainsOfflineWithoutInstance(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas:         ptr.To[int32](1),
			Selector:         &metav1.LabelSelector{},
			OfflineInstances: []string{"demo-fast-0"},
			Instances: []workloads.InstanceTemplate{{
				Name:     "fast",
				Replicas: ptr.To[int32](1),
			}},
		},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
			PodName:      "demo-fast-0",
			TemplateName: ptr.To("fast"),
		}}},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if err := setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	status := its.Status.InstanceStatus[0]
	if status.PodName != "demo-fast-0" || status.TemplateName == nil || *status.TemplateName != "fast" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatalf("offline identity was not retained: %#v", status)
	}
}

func TestSetInstanceStatusTreatsUnreportedInstanceAsAbsent(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	instanceName := "demo-0"
	desired, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	desiredPodRevision, err := instctrl.BuildPodRevision(desired[instanceName])
	if err != nil {
		t.Fatal(err)
	}

	if err := setInstanceStatus(tree, its, nil); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(instanceName)
	if status == nil || status.DesiredState != workloads.InstanceDesiredStateActive || status.CurrentState != workloads.InstanceCurrentStateAbsent || status.UpdateRevision != desiredPodRevision {
		t.Fatalf("unreported Instance was not published as Active+Absent: %#v", status)
	}
}

func TestSetInstanceStatusKeepsRuntimeStateIndependentFromConvergence(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{},
			Roles:    []workloads.ReplicaRole{{Name: "leader"}},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Generation: 2},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 1,
			CurrentState:       workloads.InstanceCurrentStatePresent,
			CurrentRevision:    "current",
			UpdateRevision:     "stale-target",
			UpToDate:           true,
			Ready:              true,
			Available:          true,
			Role:               "leader",
			Conditions: []metav1.Condition{{
				Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue,
			}},
		},
	}
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	desiredPodRevision, err := instctrl.BuildPodRevision(desiredInstances[inst.Name])
	if err != nil {
		t.Fatal(err)
	}

	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(inst.Name)
	if status == nil || status.CurrentRevision != "current" || status.UpdateRevision != desiredPodRevision || status.UpToDate ||
		!status.Ready || !status.Available || !status.Failed || status.Role != "leader" {
		t.Fatalf("runtime state was coupled to stale desired-state convergence: %#v", status)
	}
}

func TestSetInstanceStatusUsesActiveFlatTemplateOverStaleInstanceTemplate(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](1), FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{{
				Name: "fast", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{0}},
			}},
		},
		Status: workloads.InstanceSetStatus{
			AssignedOrdinals: map[string]workloads.Ordinals{"": {Discrete: []int32{0}}},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Labels: map[string]string{constant.KBAppInstanceTemplateLabelKey: ""}},
		Spec:       workloads.InstanceSpec{InstanceTemplateName: ""},
	}

	if err := setInstanceStatus(tree, its, []*workloads.Instance{inst}); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(inst.Name)
	if status == nil || status.TemplateName == nil || *status.TemplateName != "fast" || status.DesiredState != workloads.InstanceDesiredStateActive {
		t.Fatalf("active allocation did not override stale template observations: %#v", status)
	}
}

func TestIsInstanceUpdated(t *testing.T) {
	newInstance := func(generationAnnotation string, upToDate bool) *workloads.Instance {
		return &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-its-0",
				Generation: 2,
				Annotations: map[string]string{
					constant.KubeBlocksGenerationKey: generationAnnotation,
				},
			},
			Spec: workloads.InstanceSpec{
				MinReadySeconds: 1,
			},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 2,
				UpToDate:           upToDate,
			},
		}
	}
	latestInst := newInstance("2", true)
	latestRevision := stampInstanceRevision(latestInst)
	updateRevisions, err := revisionmap.Encode(map[string]string{
		latestInst.Name: latestRevision,
	})
	if err != nil {
		t.Fatalf("build revisions: %v", err)
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 3,
		},
		Status: workloads.InstanceSetStatus{
			UpdateRevisions: updateRevisions,
		},
	}

	tests := []struct {
		name string
		inst *workloads.Instance
		want bool
	}{
		{
			name: "true when instance revision matches even if parent generation changed",
			inst: func() *workloads.Instance {
				inst := newInstance("1", true)
				inst.Annotations[instanceSetRevisionAnnotationKey] = latestRevision
				return inst
			}(),
			want: true,
		},
		{
			name: "false when instance spec is latest but pod status is not up to date",
			inst: func() *workloads.Instance {
				inst := newInstance("3", false)
				inst.Annotations[instanceSetRevisionAnnotationKey] = latestRevision
				return inst
			}(),
			want: false,
		},
		{
			name: "false when instance revision annotation differs",
			inst: func() *workloads.Instance {
				inst := newInstance("3", true)
				inst.Annotations[instanceSetRevisionAnnotationKey] = "stale"
				return inst
			}(),
			want: false,
		},
		{
			name: "false when instance revision annotation is missing",
			inst: func() *workloads.Instance {
				inst := newInstance("3", true)
				delete(inst.Annotations, instanceSetRevisionAnnotationKey)
				return inst
			}(),
			want: false,
		},
		{
			name: "false when instance status has not observed latest generation",
			inst: func() *workloads.Instance {
				inst := newInstance("3", true)
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

func TestBuildInstanceRevisionIgnoresInstanceMetadata(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
			Labels: map[string]string{
				"managed-label": "desired",
			},
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "1",
				instanceSetRevisionAnnotationKey: "revision-1",
				"managed-annotation":             "desired",
			},
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
		},
	}
	revision := buildInstanceRevision(inst)

	inst.Annotations[constant.KubeBlocksGenerationKey] = "2"
	inst.Annotations[instanceSetRevisionAnnotationKey] = "revision-2"
	inst.Annotations["managed-annotation"] = "changed"
	inst.Labels["managed-label"] = "changed"
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected instance metadata to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.MinReadySeconds = 2
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected spec change to alter revision")
	}
}

func TestBuildInstanceRevisionIgnoresAssistantObjectLiveState(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
			InstanceAssistantObjects: []workloads.InstanceAssistantObject{
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
			},
		},
	}
	revision := buildInstanceRevision(inst)

	inst.Spec.InstanceAssistantObjects[0].Service.Spec.ClusterIP = "10.0.0.2"
	inst.Spec.InstanceAssistantObjects[0].Service.Spec.ClusterIPs = []string{"10.0.0.2"}
	inst.Spec.InstanceAssistantObjects[0].Service.ResourceVersion = "2"
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected assistant object live state to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.MinReadySeconds = 2
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected non-assistant spec change to alter revision")
	}
}

func TestBuildInstanceRevisionIgnoresTemplateObjectMetadata(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
		},
		Spec: workloads.InstanceSpec{
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
	}
	revision := buildInstanceRevision(inst)

	inst.Spec.Template.CreationTimestamp = metav1.Now()
	inst.Spec.Template.ResourceVersion = "2"
	inst.Spec.Template.Generation = 2
	inst.Spec.VolumeClaimTemplates[0].CreationTimestamp = metav1.Now()
	inst.Spec.VolumeClaimTemplates[0].ResourceVersion = "2"
	inst.Spec.VolumeClaimTemplates[0].Generation = 2
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected template object metadata to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.Template.Labels["pod-label"] = "changed"
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected pod template label change to alter revision")
	}

	inst.Spec.Template.Labels["pod-label"] = "desired"
	inst.Spec.VolumeClaimTemplates[0].Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected PVC template spec change to alter revision")
	}
}

func TestBuildInstanceRevisionIgnoresLifecycleActions(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "postgres", Image: "postgres:16"}},
				},
			},
			LifecycleActions: &workloads.LifecycleActions{
				TemplateVars: map[string]string{"instance": "test-its-0"},
			},
		},
	}
	revision := buildInstanceRevision(inst)

	inst.Spec.LifecycleActions.TemplateVars["instance"] = "test-its-1"
	if got := buildInstanceRevision(inst); got != revision {
		t.Fatalf("expected lifecycle action context to be ignored, got %s want %s", got, revision)
	}

	inst.Spec.Template.Spec.Containers[0].Image = "postgres:17"
	if got := buildInstanceRevision(inst); got == revision {
		t.Fatalf("expected pod template spec change to alter revision")
	}
}

func TestStampInstanceRevisionCarriesDesiredRevision(t *testing.T) {
	desired := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-its-0",
			Labels: map[string]string{
				constant.KBAppInstanceTemplateLabelKey: "tpl",
				"managed-label":                        "desired",
			},
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "3",
				"managed-annotation":             "desired",
			},
		},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 1,
		},
	}
	revision := stampInstanceRevision(desired)
	if revision == "" {
		t.Fatalf("expected revision to be stamped")
	}
	if got := desired.Annotations[instanceSetRevisionAnnotationKey]; got != revision {
		t.Fatalf("expected revision annotation %s, got %s", revision, got)
	}
	if got := buildInstanceRevision(desired); got != revision {
		t.Fatalf("expected stamped annotation to be ignored by revision hash, got %s want %s", got, revision)
	}

	desired.Annotations[instanceSetRevisionAnnotationKey] = "changed"
	if got := buildInstanceRevision(desired); got != revision {
		t.Fatalf("expected revision annotation changes to be ignored, got %s want %s", got, revision)
	}

	desired.Annotations["managed-annotation"] = "changed"
	if got := buildInstanceRevision(desired); got != revision {
		t.Fatalf("expected instance metadata annotation changes to be ignored, got %s want %s", got, revision)
	}

	desired.Annotations["managed-annotation"] = "desired"
	desired.Labels["managed-label"] = "changed"
	if got := buildInstanceRevision(desired); got != revision {
		t.Fatalf("expected instance metadata label changes to be ignored, got %s want %s", got, revision)
	}

	desired.Spec.Template.Labels = map[string]string{"pod-label": "desired"}
	if got := buildInstanceRevision(desired); got == revision {
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
			Replicas:            ptr.To[int32](1),
			FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{
				{Name: "tpl", Replicas: ptr.To[int32](1)},
			},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)

	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	inst := desiredInstances["test-its-0"]
	if inst == nil {
		t.Fatalf("expected desired instance test-its-0, got %#v", desiredInstances)
	}
	if got := getInstanceRevision(inst); got == "" {
		t.Fatalf("expected desired instance to carry revision annotation")
	} else if want := buildInstanceRevision(inst); got != want {
		t.Fatalf("expected carried revision to match desired revision, got %s want %s", got, want)
	}
}

func TestStatusReconcilerReadsCurrentRevisionFromInstanceStatus(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-its",
			Namespace:  "default",
			Generation: 3,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To[int32](1),
			FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{
				{
					Name:     "tpl",
					Replicas: ptr.To[int32](1),
					Labels: map[string]string{
						"managed-label": "desired",
					},
					Annotations: map[string]string{
						"managed-annotation": "desired",
					},
				},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 3,
		},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	desired := desiredInstances["test-its-0"]
	if desired == nil {
		t.Fatalf("expected desired instance test-its-0, got %#v", desiredInstances)
	}
	desiredRevision := getInstanceRevision(desired)
	desiredPodRevision, err := instctrl.BuildPodRevision(desired)
	if err != nil {
		t.Fatalf("build desired Pod revision: %v", err)
	}
	updateRevisions, err := revisionmap.Encode(map[string]string{
		desired.Name: desiredRevision,
	})
	if err != nil {
		t.Fatalf("build revisions: %v", err)
	}
	its.Status.UpdateRevision = "update-revision"
	its.Status.UpdateRevisions = updateRevisions

	inst := desired.DeepCopy()
	inst.Annotations[constant.KubeBlocksGenerationKey] = "1"
	inst.Annotations[instanceSetRevisionAnnotationKey] = desiredRevision
	inst.Annotations["external-annotation"] = "ignored"
	inst.Labels["external-label"] = "ignored"
	inst.Generation = 2
	inst.Status = workloads.InstanceStatus2{
		ObservedGeneration: 2,
		CurrentState:       workloads.InstanceCurrentStatePresent,
		CurrentRevision:    "pod-revision",
		UpdateRevision:     "pod-revision",
		UpToDate:           true,
		Ready:              true,
		Available:          true,
		Conditions: []metav1.Condition{
			{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
			{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
		},
	}

	if err := tree.Add(inst); err != nil {
		t.Fatalf("add instance: %v", err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile status: %v", err)
	}

	got := tree.GetRoot().(*workloads.InstanceSet)
	currentRevisions, err := revisionmap.Decode(got.Status.CurrentRevisions)
	if err != nil {
		t.Fatalf("get current revisions: %v", err)
	}
	if currentRevisions[inst.Name] != desiredRevision {
		t.Fatalf("expected aggregate Instance spec revision, got %s want %s", currentRevisions[inst.Name], desiredRevision)
	}
	status := got.FindInstanceStatus(inst.Name)
	if status == nil || status.CurrentRevision != inst.Status.CurrentRevision || status.UpdateRevision != desiredPodRevision {
		t.Fatalf("expected observed current and desired target Pod revisions, got %#v", status)
	}
	if got.Status.UpdatedReplicas != 1 {
		t.Fatalf("expected updated replicas to stay at 1, got %d", got.Status.UpdatedReplicas)
	}
	if len(got.Status.TemplatesStatus) != 1 ||
		got.Status.TemplatesStatus[0].Name != "tpl" ||
		got.Status.TemplatesStatus[0].Replicas != 1 ||
		got.Status.TemplatesStatus[0].UpdatedReplicas != 1 ||
		got.Status.TemplatesStatus[0].CurrentReplicas != 1 {
		t.Fatalf("unexpected template status: %#v", got.Status.TemplatesStatus)
	}
	if got.Status.CurrentRevision != got.Status.UpdateRevision {
		t.Fatalf("expected aggregate current revision to advance to update revision")
	}

	// Dynamic config and PVC convergence are represented by UpToDate, but do not make a
	// healthy runtime unready or unavailable.
	inst.Status.UpToDate = false
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile non-converged runtime status: %v", err)
	}
	got = tree.GetRoot().(*workloads.InstanceSet)
	status = got.FindInstanceStatus(inst.Name)
	if status == nil || status.UpToDate || !status.Ready || !status.Available {
		t.Fatalf("runtime status was coupled to desired-state convergence: %#v", status)
	}
	if got.Status.ReadyReplicas != 1 || got.Status.AvailableReplicas != 1 || got.Status.UpdatedReplicas != 0 {
		t.Fatalf("unexpected aggregate counts while convergence is pending: ready=%d available=%d updated=%d",
			got.Status.ReadyReplicas, got.Status.AvailableReplicas, got.Status.UpdatedReplicas)
	}
}

func TestStatusReconcilerDoesNotDependOnRevisionAnnotationForCurrentRevision(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-its",
			Namespace:  "default",
			Generation: 3,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To[int32](1),
			FlatInstanceOrdinal: true,
			Instances: []workloads.InstanceTemplate{
				{Name: "tpl", Replicas: ptr.To[int32](1)},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 3,
		},
	}

	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatalf("build desired instances: %v", err)
	}
	desired := desiredInstances["test-its-0"]
	desiredRevision := getInstanceRevision(desired)
	desiredPodRevision, err := instctrl.BuildPodRevision(desired)
	if err != nil {
		t.Fatalf("build desired Pod revision: %v", err)
	}
	updateRevisions, err := revisionmap.Encode(map[string]string{
		desired.Name: desiredRevision,
	})
	if err != nil {
		t.Fatalf("build revisions: %v", err)
	}
	its.Status.UpdateRevision = "update-revision"
	its.Status.UpdateRevisions = updateRevisions

	inst := desired.DeepCopy()
	delete(inst.Annotations, instanceSetRevisionAnnotationKey)
	inst.Generation = 2
	inst.Status = workloads.InstanceStatus2{
		ObservedGeneration: 2,
		CurrentState:       workloads.InstanceCurrentStatePresent,
		CurrentRevision:    "pod-current",
		UpdateRevision:     "pod-target",
		UpToDate:           true,
		Ready:              true,
		Available:          true,
		Conditions: []metav1.Condition{
			{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
			{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
		},
	}

	if err := tree.Add(inst); err != nil {
		t.Fatalf("add instance: %v", err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatalf("reconcile status: %v", err)
	}

	got := tree.GetRoot().(*workloads.InstanceSet)
	currentRevisions, err := revisionmap.Decode(got.Status.CurrentRevisions)
	if err != nil {
		t.Fatalf("get current revisions: %v", err)
	}
	if currentRevisions[inst.Name] != "" {
		t.Fatalf("expected empty aggregate spec revision for missing annotation, got %#v", currentRevisions)
	}
	status := got.FindInstanceStatus(inst.Name)
	if status == nil || status.CurrentRevision != inst.Status.CurrentRevision || status.UpdateRevision != desiredPodRevision {
		t.Fatalf("expected desired Pod revision despite missing Instance-spec revision annotation, got %#v", status)
	}
	if got.Status.UpdatedReplicas != 0 {
		t.Fatalf("expected missing spec revision annotation to keep updated replicas at 0, got %d", got.Status.UpdatedReplicas)
	}
	if len(got.Status.TemplatesStatus) != 1 ||
		got.Status.TemplatesStatus[0].UpdatedReplicas != 0 ||
		got.Status.TemplatesStatus[0].CurrentReplicas != 1 {
		t.Fatalf("unexpected template status: %#v", got.Status.TemplatesStatus)
	}
}

func TestStatusReconcilerDoesNotPublishPartialFlatAllocation(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 3},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2), FlatInstanceOrdinal: true, Template: corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{
				{Name: "a", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
				{Name: "b", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 3,
			ReadyReplicas:      2,
			Conditions: []metav1.Condition{{
				Type:               string(workloads.InstanceReady),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
			}},
			AssignedOrdinals: map[string]workloads.Ordinals{
				"a": {Discrete: []int32{0}}, "b": {Discrete: []int32{1}},
			},
			InstanceStatus: []workloads.InstanceStatus{{PodName: "demo-a-0"}},
		},
	}
	before := its.DeepCopy().Status
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)

	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, its.Status) {
		t.Fatalf("partial allocation changed status:\nbefore: %#v\nafter:  %#v", before, its.Status)
	}
}

func TestITS2RevisionUpdateTracksConfigAndPVCChanges(t *testing.T) {
	t.Run("dynamic config", func(t *testing.T) {
		configs := []workloads.ConfigTemplate{{Name: "mysql", ConfigHash: ptr.To("old")}}
		its, tree, _ := newITS2InstanceStatusFixture(t, configs)
		its.Generation++
		its.Spec.Configs[0].ConfigHash = ptr.To("new")
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertITS2UpToDate(t, its, "demo-0", false)
		assertITS2UpToDate(t, its, "demo-1", false)
	})

	t.Run("PVC expansion is scoped to one template", func(t *testing.T) {
		claim := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			}},
		}
		its := its2InstanceStatusSet(2)
		its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
		its, tree, _ := newITS2InstanceStatusFixtureFromSet(t, its)

		its.Generation++
		expanded := claim.DeepCopy()
		expanded.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		its.Spec.Instances[0].VolumeClaimTemplates = []corev1.PersistentVolumeClaim{*expanded}
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertITS2UpToDate(t, its, "demo-0", false)
		assertITS2UpToDate(t, its, "demo-1", true)
	})
}

func TestITS2AllocationChangesStayInDesiredAndCurrentState(t *testing.T) {
	its := its2InstanceStatusSet(2)
	its.Spec.FlatInstanceOrdinal = false
	its.Spec.Instances = nil
	its, tree, _ := newITS2InstanceStatusFixtureFromSet(t, its)

	its.Generation++
	its.Spec.Replicas = ptr.To[int32](3)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	added := its.FindInstanceStatus("demo-2")
	if added == nil || added.DesiredState != workloads.InstanceDesiredStateActive || added.CurrentState != workloads.InstanceCurrentStateAbsent || added.UpToDate {
		t.Fatalf("added identity was not expressed by desired/current state: %#v", added)
	}

	its.Generation++
	its.Spec.Replicas = ptr.To[int32](1)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	released := its.FindInstanceStatus("demo-1")
	if released == nil || released.DesiredState != workloads.InstanceDesiredStateReleased || released.CurrentState != workloads.InstanceCurrentStatePresent || released.UpToDate {
		t.Fatalf("removed identity was not expressed as Released/Present: %#v", released)
	}

	its.Generation++
	its.Spec.OfflineInstances = []string{"demo-0"}
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	offline := its.FindInstanceStatus("demo-0")
	if offline == nil || offline.DesiredState != workloads.InstanceDesiredStateOffline || offline.CurrentState != workloads.InstanceCurrentStatePresent || offline.UpToDate {
		t.Fatalf("offline identity was not expressed as Offline/Present: %#v", offline)
	}
}

func newITS2InstanceStatusFixture(t *testing.T, configs []workloads.ConfigTemplate) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*workloads.Instance) {
	t.Helper()
	its := its2InstanceStatusSet(2)
	its.Spec.Configs = configs
	return newITS2InstanceStatusFixtureFromSet(t, its)
}

func newITS2InstanceStatusFixtureFromSet(t *testing.T, its *workloads.InstanceSet) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*workloads.Instance) {
	t.Helper()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	desired, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	instances := make(map[string]*workloads.Instance, len(desired))
	for name, target := range desired {
		inst := target.DeepCopy()
		inst.Generation = 1
		inst.Status = workloads.InstanceStatus2{
			ObservedGeneration: 1,
			CurrentState:       workloads.InstanceCurrentStatePresent,
			CurrentRevision:    "pod-revision",
			UpdateRevision:     "pod-revision",
			UpToDate:           true,
		}
		for _, config := range inst.Spec.Configs {
			inst.Status.Configs = append(inst.Status.Configs, workloads.InstanceConfigStatus{Name: config.Name, ConfigHash: config.ConfigHash})
		}
		if err := tree.Add(inst); err != nil {
			t.Fatal(err)
		}
		instances[name] = inst
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	assertITS2UpToDate(t, its, "demo-0", true)
	assertITS2UpToDate(t, its, "demo-1", true)
	return its, tree, instances
}

func its2InstanceStatusSet(replicas int32) *workloads.InstanceSet {
	one := int32(1)
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To(replicas),
			FlatInstanceOrdinal: true,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "db", Image: "mysql:old",
			}}}},
			Instances: []workloads.InstanceTemplate{
				{Name: "a", Replicas: &one, Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
				{Name: "b", Replicas: &one, Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
			},
		},
	}
}

func assertITS2UpToDate(t *testing.T, its *workloads.InstanceSet, name string, want bool) {
	t.Helper()
	status := its.FindInstanceStatus(name)
	if status == nil || status.UpToDate != want {
		t.Fatalf("instance %s UpToDate = %#v, want %v", name, status, want)
	}
}
