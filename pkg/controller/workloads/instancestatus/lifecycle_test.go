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

package instancestatus

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func knownReference(uid string) *workloads.InstanceObjectReference {
	return &workloads.InstanceObjectReference{Name: "data-demo-0", Namespace: "default", UID: types.UID(uid), Cluster: ptr.To("")}
}
func persistentIdentity(uid string) *workloads.InstanceStorageIdentity {
	return &workloads.InstanceStorageIdentity{Complete: true, Volumes: []workloads.InstanceVolumeIdentity{{Name: "data", Claim: knownReference(uid), VolumeName: "pv-data"}}}
}
func initialized(storage *workloads.InstanceStorageIdentity) *workloads.InstanceDataStatus {
	return &workloads.InstanceDataStatus{State: workloads.InstanceDataInitialized, Identity: &workloads.InstanceDataIdentity{Storage: *storage.DeepCopy(), Dataset: "dataset-1"}, ObservedAt: ptr.To(metav1.Now())}
}

func TestLifecycleMergeDataValidity(t *testing.T) {
	original := persistentIdentity("pvc-1")
	evidence := initialized(original)
	tests := []struct {
		name     string
		storage  *workloads.InstanceStorageIdentity
		state    workloads.InstanceDataState
		replaced bool
	}{
		{name: "persistent storage survives pod replacement and capacity changes", storage: original.DeepCopy(), state: workloads.InstanceDataInitialized},
		{name: "missing storage", state: workloads.InstanceDataUnknown},
		{name: "unresolved location", storage: func() *workloads.InstanceStorageIdentity {
			s := original.DeepCopy()
			s.Complete = false
			s.Volumes[0].Claim.Cluster = nil
			return s
		}(), state: workloads.InstanceDataUnknown},
		{name: "PVC replacement", storage: persistentIdentity("pvc-2"), state: workloads.InstanceDataUnknown, replaced: true},
		{name: "known replacement with another unresolved volume", storage: func() *workloads.InstanceStorageIdentity {
			s := persistentIdentity("pvc-2")
			s.Complete = false
			s.Volumes = append(s.Volumes, workloads.InstanceVolumeIdentity{Name: "missing"})
			return s
		}(), state: workloads.InstanceDataUnknown, replaced: true},
		{name: "PV rebinding", storage: func() *workloads.InstanceStorageIdentity {
			s := original.DeepCopy()
			s.Volumes[0].VolumeName = "pv-2"
			return s
		}(), state: workloads.InstanceDataUnknown, replaced: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeLifecycle(LifecycleObservation{Data: evidence}, LifecycleObservation{}, tt.storage)
			if got.Data.State != tt.state {
				t.Fatalf("data state = %s", got.Data.State)
			}
			if tt.replaced && (got.Data.Identity.Dataset != "" || got.Data.Reason != "StorageReplaced") {
				t.Fatalf("replacement retained old dataset: %#v", got.Data)
			}
			if !tt.replaced && !reflect.DeepEqual(got.Data.Identity, evidence.Identity) {
				t.Fatal("unknown observation discarded old identity")
			}
			if tt.state == workloads.InstanceDataInitialized && !reflect.DeepEqual(got.Data.ObservedAt, evidence.ObservedAt) {
				t.Fatal("rebuilding status manufactured a fresh observation time")
			}
		})
	}
	unknown := MergeLifecycle(LifecycleObservation{Data: evidence}, LifecycleObservation{}, nil)
	recovered := MergeLifecycle(unknown, LifecycleObservation{}, original)
	if recovered.Data.State != workloads.InstanceDataUnknown {
		t.Fatal("rediscovery manufactured initialization")
	}
	confirmed := MergeLifecycle(recovered, LifecycleObservation{Data: evidence}, original)
	if confirmed.Data.State != workloads.InstanceDataInitialized {
		t.Fatal("fresh explicit evidence was not accepted")
	}
	ephemeral := &workloads.InstanceStorageIdentity{Complete: true, EphemeralPod: knownReference("pod-1")}
	replaced := ephemeral.DeepCopy()
	replaced.EphemeralPod.UID = "pod-2"
	got := MergeLifecycle(LifecycleObservation{Data: initialized(ephemeral)}, LifecycleObservation{}, replaced)
	if got.Data.State != workloads.InstanceDataUnknown || got.Data.Identity.Dataset != "" {
		t.Fatal("ephemeral data survived Pod replacement")
	}
}

func TestLifecycleMergeProtectsUnresolvedIdentities(t *testing.T) {
	old := LifecycleObservation{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipPresent, Identity: &workloads.InstanceMemberIdentity{Group: "group", Member: "member-old"}}, Execution: &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionRunning, Executor: *knownReference("pod-old"), Member: &workloads.InstanceMemberIdentity{Group: "group", Member: "member-old"}}}
	fresh := LifecycleObservation{Membership: old.Membership.DeepCopy(), Execution: old.Execution.DeepCopy()}
	fresh.Membership.Identity.Member = "member-new"
	fresh.Execution.Executor.UID = "pod-new"
	got := MergeLifecycle(old, fresh, nil)
	if got.Membership.State != workloads.InstanceMembershipUnknown || got.Membership.Identity.Member != "member-old" || got.Execution.State != workloads.InstanceExecutionUnknown || got.Execution.Executor.UID != "pod-old" {
		t.Fatalf("unresolved targets overwritten: %#v", got)
	}
	unknown := MergeLifecycle(old, LifecycleObservation{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipUnknown}}, nil)
	if unknown.Membership.Identity.Member != "member-old" {
		t.Fatal("Unknown discarded actual member identity")
	}
	absent := old.Membership.DeepCopy()
	absent.State = workloads.InstanceMembershipAbsent
	settled := MergeLifecycle(old, LifecycleObservation{Membership: absent}, nil)
	replacement := MergeLifecycle(settled, LifecycleObservation{Membership: fresh.Membership}, nil)
	if replacement.Membership.Identity.Member != "member-new" || replacement.Membership.State != workloads.InstanceMembershipPresent {
		t.Fatal("confirmed absence did not permit rebinding")
	}
	got.Membership.Identity.Member = "mutated"
	got.Execution.Executor.Cluster = ptr.To("other")
	if old.Membership.Identity.Member != "member-old" || *old.Execution.Executor.Cluster != "" {
		t.Fatal("merge aliases previous state")
	}
}

func TestBuildActualRetentionAndFreshRuntime(t *testing.T) {
	tests := []struct {
		name     string
		old      workloads.InstanceStatus
		resource ResourceObservation
		fresh    LifecycleObservation
		keep     bool
	}{
		{name: "extant absent Instance", resource: ResourceObservation{InstancePresent: true}, keep: true},
		{name: "delete-policy actual storage", resource: ResourceObservation{StorageCleanup: true}, keep: true},
		{name: "member present", old: workloads.InstanceStatus{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipPresent, Identity: &workloads.InstanceMemberIdentity{Group: "g", Member: "m"}}}, keep: true},
		{name: "member unknown", old: workloads.InstanceStatus{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipUnknown, Identity: &workloads.InstanceMemberIdentity{Group: "g", Member: "m"}}}, keep: true},
		{name: "running execution", old: workloads.InstanceStatus{Execution: &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionRunning, Executor: *knownReference("executor")}}, keep: true},
		{name: "unknown execution", old: workloads.InstanceStatus{Execution: &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionUnknown, Executor: *knownReference("executor")}}, keep: true},
		{name: "old initialized data alone", old: workloads.InstanceStatus{Data: initialized(persistentIdentity("pvc"))}},
		{name: "retained policy storage alone", resource: ResourceObservation{Storage: persistentIdentity("pvc")}},
		{name: "unobserved old row"},
		{name: "settled member", old: workloads.InstanceStatus{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipPresent, Identity: &workloads.InstanceMemberIdentity{Group: "g", Member: "m"}}}, fresh: LifecycleObservation{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipAbsent, Identity: &workloads.InstanceMemberIdentity{Group: "g", Member: "m"}}}},
		{name: "resolved execution", old: workloads.InstanceStatus{Execution: &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionRunning, Executor: *knownReference("executor")}}, fresh: LifecycleObservation{Execution: &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionSucceeded, Executor: *knownReference("executor")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.old.PodName = "demo-0"
			tt.old.Ready = true
			tt.old.Available = true
			tt.old.UpToDate = true
			tt.old.Role = "leader"
			tt.old.Configs = []workloads.InstanceConfigStatus{{Name: "old"}}
			tt.resource.InstanceName = "demo-0"
			tt.fresh.InstanceName = "demo-0"
			got, err := Build(Input{Previous: []workloads.InstanceStatus{tt.old}, Resources: []ResourceObservation{tt.resource}, Lifecycle: []LifecycleObservation{tt.fresh}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(got) == 1) != tt.keep {
				t.Fatalf("retention = %#v", got)
			}
			if tt.keep && (got[0].DesiredState != workloads.InstanceDesiredStateReleased || got[0].CurrentState != workloads.InstanceCurrentStateAbsent || got[0].Ready || got[0].Available || got[0].UpToDate || got[0].Role != "" || got[0].Configs != nil) {
				t.Fatalf("stale runtime survived: %#v", got[0])
			}
		})
	}
	for _, desired := range []workloads.InstanceDesiredState{workloads.InstanceDesiredStateActive, workloads.InstanceDesiredStateOffline, workloads.InstanceDesiredStateReleased} {
		for _, current := range []workloads.InstanceCurrentState{workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating, workloads.InstanceCurrentStateAbsent} {
			input := Input{Resources: []ResourceObservation{{InstanceName: "demo-0", InstancePresent: true, Pod: knownReference("pod"), Storage: persistentIdentity("pvc")}}}
			if desired == workloads.InstanceDesiredStateActive {
				input.DesiredAssignments = []TemplateAssignment{{InstanceName: "demo-0"}}
			}
			if desired == workloads.InstanceDesiredStateOffline {
				input.Offline = []string{"demo-0"}
			}
			if current != workloads.InstanceCurrentStateAbsent {
				input.Observations = []Observation{{InstanceName: "demo-0", State: current, Ready: true, Available: true, UpToDate: true}}
			}
			got, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if got[0].CurrentState != current || got[0].DesiredState != desired || (got[0].Pod == nil) != (current == workloads.InstanceCurrentStateAbsent) || got[0].Ready != (current == workloads.InstanceCurrentStatePresent) || got[0].UpToDate != (current == workloads.InstanceCurrentStatePresent && desired == workloads.InstanceDesiredStateActive) {
				t.Fatalf("matrix %s/%s: %#v", desired, current, got)
			}
			got[0].Storage.Volumes[0].Claim.UID = "changed"
			if input.Resources[0].Storage.Volumes[0].Claim.UID != "pvc" {
				t.Fatal("resource identity aliases input")
			}
		}
	}
}

func TestObserveStorageKeepsActualClaimsAndUnknownProvenance(t *testing.T) {
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-demo-0", Namespace: "default", UID: "pvc"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "pod"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.Name}}}}}}
	local := multicluster.IntoContext(context.Background(), "")
	got := ObserveStorage(local, pod, nil, []*corev1.PersistentVolumeClaim{claim})
	if !got.Complete || got.Volumes[0].Claim.UID != "pvc" || got.EphemeralPod != nil {
		t.Fatalf("actual identity missing: %#v", got)
	}
	unknown := ObserveStorage(context.Background(), pod, nil, []*corev1.PersistentVolumeClaim{claim})
	if unknown.Complete || unknown.Volumes[0].Claim != nil {
		t.Fatal("missing provenance guessed local")
	}
	stopped := ObserveStorage(local, nil, nil, []*corev1.PersistentVolumeClaim{claim})
	if !stopped.Complete || stopped.Volumes[0].Claim.UID != "pvc" {
		t.Fatal("stopped removed-template claim disappeared")
	}
	partial := ObserveStorage(local, nil, map[string]string{"missing": "logs"}, []*corev1.PersistentVolumeClaim{claim})
	if partial.Complete || len(partial.Volumes) != 2 {
		t.Fatal("missing expected claim became complete")
	}
	ephemeral := ObserveStorage(local, &corev1.Pod{ObjectMeta: pod.ObjectMeta}, nil, nil)
	if !ephemeral.Complete || ephemeral.EphemeralPod.UID != "pod" {
		t.Fatal("ephemeral storage missing actual Pod UID")
	}
}

func TestObserveStorageUsesActualMountsInsteadOfDesiredTemplates(t *testing.T) {
	local := multicluster.IntoContext(context.Background(), "")
	mounted := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-old", Namespace: "default", UID: "old"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-old"}}
	unused := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-new", Namespace: "default", UID: "new"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-new"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "pod"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-old"}}}}}}
	storage := ObserveStorage(local, pod, map[string]string{"data-new": "new-template", "missing": "logs"}, []*corev1.PersistentVolumeClaim{mounted, unused})
	if !storage.Complete || len(storage.Volumes) != 1 || storage.Volumes[0].Claim.UID != "old" {
		t.Fatalf("desired or unused storage changed actual identity: %#v", storage)
	}
	external := ObserveStorage(local, pod, nil, nil)
	if external.Complete || len(external.Volumes) != 1 || external.Volumes[0].Claim != nil {
		t.Fatal("external missing PVC identity was fabricated")
	}
}

func TestStorageIdentityIgnoresOrderingAndIncompleteKnowledge(t *testing.T) {
	old := persistentIdentity("pvc-1")
	old.Volumes = append(old.Volumes, workloads.InstanceVolumeIdentity{Name: "logs", Claim: knownReference("pvc-logs"), VolumeName: "pv-logs"})
	reordered := old.DeepCopy()
	reordered.Volumes[0], reordered.Volumes[1] = reordered.Volumes[1], reordered.Volumes[0]
	if storageReplaced(old, reordered) || !sameStorageIdentity(old, reordered) {
		t.Fatal("volume ordering changed data identity")
	}
	incomplete := old.DeepCopy()
	incomplete.Complete = false
	if storageReplaced(incomplete, old) || !sameStorageIdentity(incomplete, old) {
		t.Fatal("completeness changed data identity")
	}
	unknownCluster := old.DeepCopy()
	unknownCluster.Complete = false
	unknownCluster.Volumes[0].Claim.Cluster = nil
	replaced := unknownCluster.DeepCopy()
	replaced.Volumes[0].Claim.UID = "replacement"
	if !storageReplaced(unknownCluster, replaced) {
		t.Fatal("known UID replacement was hidden by unknown location")
	}
	if storageReplaced(unknownCluster, old) {
		t.Fatal("learning location manufactured replacement")
	}
}

func TestLifecycleRejectsUnboundTerminalObservations(t *testing.T) {
	member := &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipPresent, Identity: &workloads.InstanceMemberIdentity{Group: "g", Member: "old"}}
	execution := &workloads.InstanceExecutionObservation{Action: "join", State: workloads.InstanceExecutionRunning, Executor: *knownReference("executor")}
	old := LifecycleObservation{Membership: member, Data: initialized(persistentIdentity("pvc")), Execution: execution}
	fresh := LifecycleObservation{Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipAbsent}, Data: &workloads.InstanceDataStatus{State: workloads.InstanceDataInitialized, Identity: &workloads.InstanceDataIdentity{Storage: *persistentIdentity("pvc")}}, Execution: &workloads.InstanceExecutionObservation{State: workloads.InstanceExecutionSucceeded, Action: "join"}}
	got := MergeLifecycle(old, fresh, persistentIdentity("pvc"))
	if got.Membership.State != workloads.InstanceMembershipUnknown || got.Membership.Identity.Member != "old" || got.Data.State != workloads.InstanceDataUnknown || got.Data.Identity.Dataset != "dataset-1" || got.Execution.State != workloads.InstanceExecutionUnknown || got.Execution.Executor.UID != "executor" {
		t.Fatalf("unbound success accepted: %#v", got)
	}
	retained, err := Build(Input{Lifecycle: []LifecycleObservation{{InstanceName: "demo-0", Membership: &workloads.InstanceMembershipStatus{State: workloads.InstanceMembershipUnknown}}}})
	if err != nil || len(retained) != 1 {
		t.Fatalf("explicit uncertainty pruned: %v %#v", err, retained)
	}
}

func TestReadMountedClaimsIsReadOnlyAndIncompleteOnFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "default", UID: "external-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "external-pv"}}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(claim).Build()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod-uid"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "external"}}}}}}
	local := multicluster.IntoContext(context.Background(), "")
	claims, pending := ReadMountedClaims(local, reader, pod, nil)
	if pending || len(claims) != 1 || !ObserveStorage(local, pod, nil, claims).Complete {
		t.Fatal("scoped read failed to observe external identity")
	}
	if len(claims[0].OwnerReferences) != 0 {
		t.Fatal("read adopted external claim")
	}
	pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "missing"
	claims, pending = ReadMountedClaims(local, reader, pod, nil)
	if !pending || len(claims) != 0 || ObserveStorage(local, pod, nil, claims).Complete {
		t.Fatal("missing read became known storage")
	}
	_, pending = ReadMountedClaims(context.Background(), reader, pod, nil)
	if pending {
		t.Fatal("unknown location guessed a read context")
	}
	pod.Spec.Volumes = []corev1.Volume{{Name: "disk", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/data"}}}}
	storage := ObserveStorage(local, pod, nil, nil)
	if storage.Complete || storage.EphemeralPod != nil {
		t.Fatal("unsupported persistent storage became ephemeral identity")
	}
}

func TestExternalClaimUsesExactPodReadProvenance(t *testing.T) {
	location := "worker-a"
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: location}}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "external"}}}}}}
	claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "default", UID: "pvc"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv"}}
	storage := ObserveStorage(multicluster.IntoContext(context.Background(), "worker-a,worker-b"), pod, nil, []*corev1.PersistentVolumeClaim{claim})
	if !storage.Complete || storage.Volumes[0].Claim.Cluster == nil || *storage.Volumes[0].Claim.Cluster != location {
		t.Fatal("exact Pod location was lost for an external claim")
	}
}

func TestMountedClaimDoesNotUseSameNameFromAnotherCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	foreign := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "default", UID: "worker-a-pvc", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-a"}}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-a"}}
	actual := foreign.DeepCopy()
	actual.UID = "worker-b-pvc"
	actual.Annotations = nil
	actual.Spec.VolumeName = "pv-b"
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actual).Build()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod-b", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-b"}}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}}}}}
	ctx := multicluster.IntoContext(context.Background(), "worker-a,worker-b")
	claims, pending := ReadMountedClaims(ctx, reader, pod, []*corev1.PersistentVolumeClaim{foreign})
	storage := ObserveStorage(ctx, pod, nil, claims)
	if pending || !storage.Complete || storage.Volumes[0].Claim.UID != "worker-b-pvc" || *storage.Volumes[0].Claim.Cluster != "worker-b" {
		t.Fatalf("foreign same-name claim satisfied Pod: %#v", storage)
	}
	unknown := ObserveStorage(ctx, pod, nil, []*corev1.PersistentVolumeClaim{foreign})
	if unknown.Complete || unknown.Volumes[0].Claim != nil {
		t.Fatal("foreign identity became actual storage")
	}
}

func TestOwnershipChangesDoNotChangeDataOrExecutionTargets(t *testing.T) {
	storage := persistentIdentity("pvc")
	storage.Volumes[0].OwnerUID = "owner-old"
	changed := storage.DeepCopy()
	changed.Volumes[0].OwnerUID = "owner-new"
	if storageReplaced(storage, changed) || !sameStorageIdentity(storage, changed) {
		t.Fatal("owner change replaced data identity")
	}
	old := &workloads.InstanceExecutionObservation{State: workloads.InstanceExecutionRunning, Action: "join", Executor: *knownReference("executor"), Data: &workloads.InstanceDataIdentity{Storage: *storage, Dataset: "dataset"}}
	done := old.DeepCopy()
	done.State = workloads.InstanceExecutionSucceeded
	done.Data.Storage = *changed
	got := MergeLifecycle(LifecycleObservation{Execution: old}, LifecycleObservation{Execution: done}, changed)
	if got.Execution.State != workloads.InstanceExecutionSucceeded {
		t.Fatal("ownership change conflicted with actual execution target")
	}
}

func TestGenericEphemeralClaimIsIncompleteWhileEmptyDirBindsToPod(t *testing.T) {
	ctx := multicluster.IntoContext(context.Background(), "")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod-uid"}, Spec: corev1.PodSpec{Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{Ephemeral: &corev1.EphemeralVolumeSource{}}}}}}
	generic := ObserveStorage(ctx, pod, nil, nil)
	if generic.Complete || generic.EphemeralPod == nil || generic.EphemeralPod.UID != "pod-uid" {
		t.Fatal("generic ephemeral claim omitted actual PVC identity")
	}
	pod.Spec.Volumes[0].VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	emptyDir := ObserveStorage(ctx, pod, nil, nil)
	if !emptyDir.Complete || emptyDir.EphemeralPod.UID != "pod-uid" {
		t.Fatal("EmptyDir failed to bind to actual Pod UID")
	}
}
