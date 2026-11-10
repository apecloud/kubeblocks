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
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestStatusReconcilerPublishesInstanceCurrentState(t *testing.T) {
	tests := []struct {
		name                 string
		pod                  *corev1.Pod
		want                 workloads.InstanceCurrentState
		wantCurrentRevision  string
		wantReady            bool
		wantAvailable        bool
		wantFailure          bool
		wantReadyMessage     string
		wantAvailableMessage string
	}{
		{
			name: "absent", want: workloads.InstanceCurrentStateAbsent,
			wantReadyMessage: "demo-0", wantAvailableMessage: "demo-0",
		},
		{
			name:                 "terminating",
			pod:                  terminatingPod("current"),
			want:                 workloads.InstanceCurrentStateTerminating,
			wantCurrentRevision:  "current",
			wantReadyMessage:     "demo-0",
			wantAvailableMessage: "demo-0",
		},
		{
			name: "present but not ready",
			pod:  currentPod("current", corev1.PodRunning),
			want: workloads.InstanceCurrentStatePresent, wantCurrentRevision: "current",
			wantReadyMessage: "demo-0",
		},
		{
			name: "present ready and available",
			pod:  readyPod("current"),
			want: workloads.InstanceCurrentStatePresent, wantCurrentRevision: "current",
			wantReady: true, wantAvailable: true,
		},
		{
			name: "present failed",
			pod:  currentPod("current", corev1.PodFailed),
			want: workloads.InstanceCurrentStatePresent, wantCurrentRevision: "current",
			wantFailure: true, wantReadyMessage: "demo-0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &workloads.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", Generation: 1},
				Status: workloads.InstanceStatus2{
					CurrentRevision: "stale",
					UpdateRevision:  "target",
					UpToDate:        true,
					Ready:           true,
					Available:       true,
					Role:            "leader",
					VolumeExpansion: true,
					Configs:         []workloads.InstanceConfigStatus{{Name: "stale"}},
					Conditions: []metav1.Condition{
						{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue, Reason: workloads.ReasonReady},
						{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue, Reason: workloads.ReasonAvailable},
						{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue, Reason: workloads.ReasonInstanceFailure},
						{Type: string(workloads.InstanceUpdateRestricted), Status: metav1.ConditionTrue, Reason: workloads.ReasonInstanceUpdateRestricted},
					},
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
			if inst.Status.CurrentRevision != tt.wantCurrentRevision {
				t.Fatalf("expected current revision %q, got %#v", tt.wantCurrentRevision, inst.Status)
			}
			if inst.Status.Ready != tt.wantReady || inst.Status.Available != tt.wantAvailable {
				t.Fatalf("unexpected ready/available status: %#v", inst.Status)
			}
			if inst.Status.UpToDate || inst.Status.Role != "" || inst.Status.VolumeExpansion || inst.Status.Configs != nil {
				t.Fatalf("instance retained stale runtime fields: %#v", inst.Status)
			}
			assertCondition(t, inst, workloads.InstanceReady, tt.wantReady, tt.wantReadyMessage)
			assertCondition(t, inst, workloads.InstanceAvailable, tt.wantAvailable, tt.wantAvailableMessage)
			if meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceFailure)) != tt.wantFailure {
				t.Fatalf("unexpected failure condition: %#v", inst.Status.Conditions)
			}
			if !meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceUpdateRestricted)) {
				t.Fatalf("status reconciliation removed an independently owned condition: %#v", inst.Status.Conditions)
			}
		})
	}
}

func TestStatusReconcilerAggregatesRestorePVCConditionsWithoutPod(t *testing.T) {
	newFixture := func() (*workloads.Instance, *kubebuilderx.ObjectTree, string) {
		claim := corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{
			Name: "data",
			Annotations: map[string]string{
				constant.RestoreSourceKindAnnotationKey: "Backup",
			},
		}}
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", Generation: 2},
			Spec: workloads.InstanceSpec{
				InstanceSetName:      "demo",
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{claim},
			},
		}
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(inst)
		return inst, tree, intctrlutil.ComposePVCName(corev1.PersistentVolumeClaim{ObjectMeta: claim.ObjectMeta}, "demo", "demo-0")
	}
	addPVC := func(t *testing.T, tree *kubebuilderx.ObjectTree, name string, status corev1.ConditionStatus) {
		t.Helper()
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Status: corev1.PersistentVolumeClaimStatus{Conditions: []corev1.PersistentVolumeClaimCondition{{
				Type:    corev1.PersistentVolumeClaimConditionType(workloads.InstanceRestore),
				Status:  status,
				Message: "restore result",
			}}},
		}
		if err := tree.Add(pvc); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("waits for the PVC before the Pod exists", func(t *testing.T) {
		inst, tree, _ := newFixture()
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		cond := meta.FindStatusCondition(inst.Status.Conditions, string(workloads.InstanceRestore))
		if cond == nil || cond.Status != metav1.ConditionUnknown || cond.Reason != workloads.ReasonRestoreRunning {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})

	t.Run("publishes completed", func(t *testing.T) {
		inst, tree, pvcName := newFixture()
		addPVC(t, tree, pvcName, corev1.ConditionTrue)
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		cond := meta.FindStatusCondition(inst.Status.Conditions, string(workloads.InstanceRestore))
		if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != workloads.ReasonRestoreCompleted {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})

	t.Run("publishes failure first", func(t *testing.T) {
		inst, tree, pvcName := newFixture()
		addPVC(t, tree, pvcName, corev1.ConditionFalse)
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		cond := meta.FindStatusCondition(inst.Status.Conditions, string(workloads.InstanceRestore))
		if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != workloads.ReasonRestoreFailed {
			t.Fatalf("unexpected Restore condition: %#v", cond)
		}
	})
}

func TestStatusReconcilerKeepsUpToDateFalseUntilPVCExpansionCompletes(t *testing.T) {
	claim := corev1.PersistentVolumeClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},
		}},
	}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", Generation: 1},
		Spec: workloads.InstanceSpec{
			InstanceSetName:      "demo",
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{claim},
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "db", Image: "mysql:old",
			}}}},
		},
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	pod, err := buildInstancePod(inst, inst.Status.UpdateRevision)
	if err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodRunning
	if err := tree.Add(pod); err != nil {
		t.Fatal(err)
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      intctrlutil.ComposePVCName(corev1.PersistentVolumeClaim{ObjectMeta: claim.ObjectMeta}, inst.Spec.InstanceSetName, inst.Name),
			Namespace: inst.Namespace,
			Labels:    map[string]string{constant.KBAppPodNameLabelKey: inst.Name},
		},
		Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
		}},
		Status: corev1.PersistentVolumeClaimStatus{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
		},
	}
	if err := tree.Add(pvc); err != nil {
		t.Fatal(err)
	}

	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if inst.Status.UpToDate || inst.Status.VolumeExpansion {
		t.Fatalf("desired expansion must invalidate convergence before the PVC spec is patched: %#v", inst.Status)
	}

	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if inst.Status.UpToDate || !inst.Status.VolumeExpansion {
		t.Fatalf("running expansion must remain stale and publish VolumeExpansion: %#v", inst.Status)
	}

	pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("2Gi")
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if !inst.Status.UpToDate || inst.Status.VolumeExpansion {
		t.Fatalf("completed expansion must restore convergence: %#v", inst.Status)
	}
	for _, missing := range []string{"PVC", "capacity"} {
		for _, previous := range []bool{false, true} {
			t.Run(fmt.Sprintf("missing %s preserves %t", missing, previous), func(t *testing.T) {
				desired := inst.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
				capacity := desired.DeepCopy()
				if !previous {
					capacity.Sub(resource.MustParse("1Gi"))
				}
				pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: capacity}
				if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
				if inst.Status.UpToDate != previous {
					t.Fatalf("initial UpToDate = %t, want %t", inst.Status.UpToDate, previous)
				}

				if missing == "PVC" {
					if err := tree.Delete(pvc); err != nil {
						t.Fatal(err)
					}
				} else {
					pvc.Status.Capacity = nil
				}
				if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
				if inst.Status.UpToDate != previous {
					t.Errorf("missing %s changed UpToDate from %t to %t", missing, previous, inst.Status.UpToDate)
				}

				inst.Generation++
				desired.Add(resource.MustParse("1Gi"))
				inst.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = desired
				if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
				if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
				if inst.Status.UpToDate {
					t.Error("the previous result must not confirm a new capacity target")
				}

				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = desired
				pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: desired}
				if err := tree.Add(pvc); err != nil {
					t.Fatal(err)
				}
				if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
					t.Fatal(err)
				}
				if !inst.Status.UpToDate {
					t.Error("observing the new capacity must restore convergence")
				}
			})
		}
	}

}

func currentPod(revision string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{appsv1.ControllerRevisionHashLabelKey: revision}},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

func terminatingPod(revision string) *corev1.Pod {
	pod := currentPod(revision, corev1.PodRunning)
	pod.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	return pod
}

func readyPod(revision string) *corev1.Pod {
	pod := currentPod(revision, corev1.PodRunning)
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	return pod
}

func assertCondition(t *testing.T, inst *workloads.Instance, conditionType workloads.ConditionType, expected bool, message string) {
	t.Helper()
	condition := meta.FindStatusCondition(inst.Status.Conditions, string(conditionType))
	if condition == nil {
		t.Fatalf("condition %q not found: %#v", conditionType, inst.Status.Conditions)
	}
	wantStatus := metav1.ConditionFalse
	if expected {
		wantStatus = metav1.ConditionTrue
	}
	if condition.Status != wantStatus || condition.Message != message || condition.ObservedGeneration != inst.Generation {
		t.Fatalf("unexpected condition %q: %#v", conditionType, condition)
	}
}

func TestStatusReconcilerRecordsProvisioningAndPreservesMembership(t *testing.T) {
	for _, member := range []*bool{nil, ptr.To(false), ptr.To(true)} {
		for _, stopped := range []bool{false, true} {
			inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"}, Spec: workloads.InstanceSpec{Stop: ptr.To(stopped)}}
			inst.Status.MemberJoined = member
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(inst)
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if inst.Status.Provisioned {
				t.Fatal("child object without Pod invented provisioning")
			}
			pod := currentPod("current", corev1.PodPending)
			pod.Name = inst.Name
			pod.Namespace = inst.Namespace
			if err := tree.Add(pod); err != nil {
				t.Fatal(err)
			}
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if !inst.Status.Provisioned || inst.Status.MemberJoined != member {
				t.Fatalf("runtime observation lost owned facts: %#v", inst.Status)
			}
			if err := tree.Delete(pod); err != nil {
				t.Fatal(err)
			}
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if !inst.Status.Provisioned || inst.Status.MemberJoined != member {
				t.Fatalf("runtime removal erased historical facts: %#v", inst.Status)
			}
		}
	}
}

func TestDataResultRequiresAppliedTaskAndCurrentWorkerSuccess(t *testing.T) {
	for _, tc := range []struct {
		name     string
		actual   bool
		target   string
		remote   string
		worker   corev1.ContainerStatus
		complete bool
	}{
		{name: "missing task", worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		{name: "different applied input", actual: true, remote: "old-source", complete: true, worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		{name: "different target", actual: true, target: "demo-2", remote: "source", worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		{name: "unrelated init", actual: true, remote: "source", worker: corev1.ContainerStatus{Name: "other-init", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		{name: "running worker", actual: true, remote: "source", worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}, LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
		{name: "failed worker", actual: true, remote: "source", worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}},
		{name: "matching success", actual: true, remote: "source", worker: corev1.ContainerStatus{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}, complete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			taskSpec := func(remote, target string) corev1.PodSpec {
				if target == "" {
					target = "demo-1"
				}
				env, err := kbagent.BuildEnv4Worker([]proto.Task{{Instance: "demo", Task: "newReplica", UID: target, Replicas: target, NewReplica: &proto.NewReplicaTask{Remote: remote, Port: 3502, Replicas: target}}})
				if err != nil {
					t.Fatal(err)
				}
				return corev1.PodSpec{InitContainers: []corev1.Container{{Name: kbagent.ContainerName4Worker, Env: []corev1.EnvVar{*env}}}}
			}
			inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-1", Namespace: "default"}, Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: taskSpec("source", "demo-1")}}}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(inst)
			pod := &corev1.Pod{ObjectMeta: inst.ObjectMeta, Status: corev1.PodStatus{Phase: corev1.PodPending, InitContainerStatuses: []corev1.ContainerStatus{tc.worker}}}
			if tc.actual {
				pod.Spec = taskSpec(tc.remote, tc.target)
			}
			if err := tree.Add(pod); err != nil {
				t.Fatal(err)
			}
			result, err := NewStatusReconciler().Reconcile(tree)
			if err != nil || result != kubebuilderx.RetryAfter(time.Second) || inst.Status.DataLoaded == nil || *inst.Status.DataLoaded != tc.complete {
				t.Fatalf("result=%#v data=%v err=%v", result, inst.Status.DataLoaded, err)
			}
			if err := tree.Delete(pod); err != nil {
				t.Fatal(err)
			}
			inst.Spec.Stop = ptr.To(true)
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if inst.Status.DataLoaded == nil || *inst.Status.DataLoaded != tc.complete {
				t.Fatal("Stop erased tracked result")
			}
		})
	}
}

func TestInitialDataSatisfactionRequiresObservedNoCopyRuntime(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hooks      bool
		pod        bool
		actualTask bool
		previous   *bool
		want       *bool
	}{
		{name: "no hooks", pod: true},
		{name: "no runtime", hooks: true},
		{name: "initial runtime", hooks: true, pod: true, want: ptr.To(true)},
		{name: "actual pending task", hooks: true, pod: true, actualTask: true, want: ptr.To(false)},
		{name: "pending result without task", hooks: true, pod: true, previous: ptr.To(false), want: ptr.To(false)},
		{name: "pending result with task", hooks: true, pod: true, actualTask: true, previous: ptr.To(false), want: ptr.To(false)},
		{name: "completed result with task", hooks: true, pod: true, actualTask: true, previous: ptr.To(true), want: ptr.To(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-1", Namespace: "default"}, Status: workloads.InstanceStatus2{DataLoaded: tc.previous}}
			if tc.hooks {
				inst.Spec.LifecycleActions = &workloads.LifecycleActions{DataDump: &kbappsv1.Action{}, DataLoad: &kbappsv1.Action{}}
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(inst)
			if tc.pod {
				pod := currentPod("current", corev1.PodPending)
				pod.ObjectMeta = inst.ObjectMeta
				if tc.actualTask {
					env, err := kbagent.BuildEnv4Worker([]proto.Task{{Instance: "demo", Task: "newReplica", UID: inst.Name, Replicas: inst.Name, NewReplica: &proto.NewReplicaTask{Remote: "source", Port: 3502, Replicas: inst.Name}}})
					if err != nil {
						t.Fatal(err)
					}
					pod.Spec.InitContainers = []corev1.Container{{Name: kbagent.ContainerName4Worker, Env: []corev1.EnvVar{*env}}}
				}
				if err := tree.Add(pod); err != nil {
					t.Fatal(err)
				}
			}
			result, err := NewStatusReconciler().Reconcile(tree)
			if err != nil {
				t.Fatal(err)
			}
			if (inst.Status.DataLoaded == nil) != (tc.want == nil) || (tc.want != nil && *inst.Status.DataLoaded != *tc.want) {
				t.Fatalf("data=%v want=%v", inst.Status.DataLoaded, tc.want)
			}
			if tc.previous == nil && tc.want != nil && result != kubebuilderx.RetryAfter(time.Second) {
				t.Fatalf("initial satisfaction was not committed separately: %#v", result)
			}
		})
	}
}

func TestCompletedDataStillWaitsForMinReadySeconds(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"},
		Spec: workloads.InstanceSpec{
			MinReadySeconds: 3600,
			LifecycleActions: &workloads.LifecycleActions{
				DataDump: &kbappsv1.Action{}, DataLoad: &kbappsv1.Action{},
			},
		},
	}
	pod := readyPod("current")
	pod.ObjectMeta = inst.ObjectMeta
	pod.Status.Conditions[0].LastTransitionTime = metav1.Now()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(inst)
	if err := tree.Add(pod); err != nil {
		t.Fatal(err)
	}
	result, err := NewStatusReconciler().Reconcile(tree)
	if err != nil || result != kubebuilderx.RetryAfter(time.Second) || !ptr.Deref(inst.Status.DataLoaded, false) {
		t.Fatalf("initial data result was not recorded separately: result=%#v status=%#v err=%v", result, inst.Status, err)
	}
	result, err = NewStatusReconciler().Reconcile(tree)
	if err != nil || result != kubebuilderx.RetryAfter(time.Second) || !inst.Status.Ready || inst.Status.Available {
		t.Fatalf("completed data skipped readiness countdown: result=%#v status=%#v err=%v", result, inst.Status, err)
	}
	pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-2 * time.Hour))
	result, err = NewStatusReconciler().Reconcile(tree)
	if err != nil || result != kubebuilderx.Continue || !inst.Status.Ready || !inst.Status.Available {
		t.Fatalf("elapsed readiness countdown did not make the Instance available: result=%#v status=%#v err=%v", result, inst.Status, err)
	}
}
