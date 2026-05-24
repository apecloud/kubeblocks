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

package workloads

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
)

func newCleanupScheme(t *testing.T) *k8sruntime.Scheme {
	scheme := k8sruntime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := workloadsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return scheme
}

func newCleanupInstanceSet(namespace, name, exclusiveRole string) *workloadsv1.InstanceSet {
	return &workloadsv1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: workloadsv1.InstanceSetSpec{
			Roles: []workloadsv1.ReplicaRole{
				{Name: exclusiveRole, ParticipatesInQuorum: true, UpdatePriority: 5, IsExclusive: true},
				{Name: "secondary", ParticipatesInQuorum: true, UpdatePriority: 3},
			},
		},
	}
}

func newCleanupPod(namespace, name, itsName, role, annotation string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:          constant.AppName,
				instanceset.WorkloadsManagedByLabelKey: workloadsv1.InstanceSetKind,
				instanceset.WorkloadsInstanceLabelKey:  itsName,
			},
		},
	}
	if role != "" {
		pod.Labels[constant.RoleLabelKey] = role
	}
	if annotation != "" {
		pod.Annotations = map[string]string{
			constant.LastRoleEventVersionAnnotationKey: annotation,
		}
	}
	return pod
}

// Legacy event whose EventTime is strictly newer than a peer's bare-EventTime
// annotation must strip the peer's exclusive role label and advance the peer
// annotation to the source event's EventTime micros. The previous
// implementation hard-coded the legacy comparison EventTime to 0, which made
// every legacy peer cleanup look stale.
func TestCleanupExclusiveRolePeersLegacyEventAdvancesOlderPeer(t *testing.T) {
	scheme := newCleanupScheme(t)
	const (
		ns        = "default"
		itsName   = "redis-0"
		newPod    = "redis-0-0"
		peerName  = "redis-0-1"
		roleName  = "primary"
		newMicros = int64(1779550700000000)
	)
	its := newCleanupInstanceSet(ns, itsName, roleName)
	newPodObj := newCleanupPod(ns, newPod, itsName, roleName, fmt.Sprintf("%d", newMicros))
	peer := newCleanupPod(ns, peerName, itsName, roleName, "1779550500000000") // older EventTime
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, newPodObj, peer).Build()

	r := &InstanceEventReconciler{Client: cli, Scheme: scheme}
	parsed := roleProbeOutput{role: roleName, mode: roleProbeVersionModeNone}
	if err := r.cleanupExclusiveRolePeers(context.Background(), logr.Discard(), newPodObj, parsed, newMicros); err != nil {
		t.Fatalf("cleanupExclusiveRolePeers err = %v", err)
	}
	updated := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peerName}, updated); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if _, ok := updated.Labels[constant.RoleLabelKey]; ok {
		t.Fatalf("peer role label still present after legacy cleanup: %v", updated.Labels)
	}
	want := fmt.Sprintf("%d", newMicros)
	if got := updated.Annotations[constant.LastRoleEventVersionAnnotationKey]; got != want {
		t.Fatalf("peer annotation = %q, want %q", got, want)
	}
}

// Legacy event whose EventTime is older than a peer's bare-EventTime
// annotation must leave the peer alone (no label strip, no annotation
// rewrite), preserving the peer's recorded state.
func TestCleanupExclusiveRolePeersLegacyEventDoesNotStripNewerPeer(t *testing.T) {
	scheme := newCleanupScheme(t)
	const (
		ns        = "default"
		itsName   = "redis-0"
		newPod    = "redis-0-0"
		peerName  = "redis-0-1"
		roleName  = "primary"
		newMicros = int64(1779550500000000)
		peerAnnot = "1779550700000000"
	)
	its := newCleanupInstanceSet(ns, itsName, roleName)
	newPodObj := newCleanupPod(ns, newPod, itsName, roleName, fmt.Sprintf("%d", newMicros))
	peer := newCleanupPod(ns, peerName, itsName, roleName, peerAnnot)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, newPodObj, peer).Build()

	r := &InstanceEventReconciler{Client: cli, Scheme: scheme}
	parsed := roleProbeOutput{role: roleName, mode: roleProbeVersionModeNone}
	if err := r.cleanupExclusiveRolePeers(context.Background(), logr.Discard(), newPodObj, parsed, newMicros); err != nil {
		t.Fatalf("cleanupExclusiveRolePeers err = %v", err)
	}
	updated := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peerName}, updated); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if got, ok := updated.Labels[constant.RoleLabelKey]; !ok || got != roleName {
		t.Fatalf("peer role label was stripped or changed: %v", updated.Labels)
	}
	if got := updated.Annotations[constant.LastRoleEventVersionAnnotationKey]; got != peerAnnot {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", got, peerAnnot)
	}
}

// Engine event whose version is strictly larger than a peer's engine
// annotation must strip the peer and advance the peer annotation to the new
// engine version.
func TestCleanupExclusiveRolePeersEngineEventAdvancesOlderPeer(t *testing.T) {
	scheme := newCleanupScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peerName   = "redis-0-1"
		roleName   = "primary"
		newVersion = uint64(20)
		peerAnnot  = "engine:10"
	)
	its := newCleanupInstanceSet(ns, itsName, roleName)
	newPodObj := newCleanupPod(ns, newPod, itsName, roleName, fmt.Sprintf("engine:%d", newVersion))
	peer := newCleanupPod(ns, peerName, itsName, roleName, peerAnnot)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, newPodObj, peer).Build()

	r := &InstanceEventReconciler{Client: cli, Scheme: scheme}
	parsed := roleProbeOutput{role: roleName, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := r.cleanupExclusiveRolePeers(context.Background(), logr.Discard(), newPodObj, parsed, 0); err != nil {
		t.Fatalf("cleanupExclusiveRolePeers err = %v", err)
	}
	updated := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peerName}, updated); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if _, ok := updated.Labels[constant.RoleLabelKey]; ok {
		t.Fatalf("peer role label still present after engine cleanup: %v", updated.Labels)
	}
	if got := updated.Annotations[constant.LastRoleEventVersionAnnotationKey]; got != fmt.Sprintf("engine:%d", newVersion) {
		t.Fatalf("peer annotation = %q, want %q", got, fmt.Sprintf("engine:%d", newVersion))
	}
}

// Engine event whose version is smaller than a peer's engine annotation must
// leave the peer's exclusive role label intact, even though the new event was
// itself accepted on a different Pod.
func TestCleanupExclusiveRolePeersEngineEventDoesNotStripNewerPeer(t *testing.T) {
	scheme := newCleanupScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peerName   = "redis-0-1"
		roleName   = "primary"
		newVersion = uint64(10)
		peerAnnot  = "engine:20"
	)
	its := newCleanupInstanceSet(ns, itsName, roleName)
	newPodObj := newCleanupPod(ns, newPod, itsName, roleName, fmt.Sprintf("engine:%d", newVersion))
	peer := newCleanupPod(ns, peerName, itsName, roleName, peerAnnot)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, newPodObj, peer).Build()

	r := &InstanceEventReconciler{Client: cli, Scheme: scheme}
	parsed := roleProbeOutput{role: roleName, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := r.cleanupExclusiveRolePeers(context.Background(), logr.Discard(), newPodObj, parsed, 0); err != nil {
		t.Fatalf("cleanupExclusiveRolePeers err = %v", err)
	}
	updated := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peerName}, updated); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if got, ok := updated.Labels[constant.RoleLabelKey]; !ok || got != roleName {
		t.Fatalf("peer role label was stripped: %v", updated.Labels)
	}
	if got := updated.Annotations[constant.LastRoleEventVersionAnnotationKey]; got != peerAnnot {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", got, peerAnnot)
	}
}

// Non-exclusive role events must not touch peers regardless of version.
func TestCleanupExclusiveRolePeersNonExclusiveRoleSkipsPeer(t *testing.T) {
	scheme := newCleanupScheme(t)
	const (
		ns         = "default"
		itsName    = "redis-0"
		newPod     = "redis-0-0"
		peerName   = "redis-0-1"
		roleName   = "secondary"
		newVersion = uint64(20)
	)
	its := newCleanupInstanceSet(ns, itsName, "primary") // secondary is not exclusive in this matrix
	newPodObj := newCleanupPod(ns, newPod, itsName, roleName, fmt.Sprintf("engine:%d", newVersion))
	peer := newCleanupPod(ns, peerName, itsName, roleName, "engine:10")
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its, newPodObj, peer).Build()

	r := &InstanceEventReconciler{Client: cli, Scheme: scheme}
	parsed := roleProbeOutput{role: roleName, version: newVersion, mode: roleProbeVersionModeEngine}
	if err := r.cleanupExclusiveRolePeers(context.Background(), logr.Discard(), newPodObj, parsed, 0); err != nil {
		t.Fatalf("cleanupExclusiveRolePeers err = %v", err)
	}
	updated := &corev1.Pod{}
	if err := cli.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: peerName}, updated); err != nil {
		t.Fatalf("get peer pod: %v", err)
	}
	if got, ok := updated.Labels[constant.RoleLabelKey]; !ok || got != roleName {
		t.Fatalf("peer role label changed for non-exclusive role: %v", updated.Labels)
	}
	want := "engine:10"
	if got := updated.Annotations[constant.LastRoleEventVersionAnnotationKey]; got != want {
		t.Fatalf("peer annotation = %q, want %q (unchanged)", got, want)
	}
}
