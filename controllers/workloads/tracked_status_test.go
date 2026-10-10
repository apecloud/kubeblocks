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

package workloads

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
)

func TestInstanceSetReconcilePersistsTrackedFactsAfterRuntimeRemoval(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "Pod runtime", true: "Instance runtime"}[instanceAPI], func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloadsv1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			its := &workloadsv1.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "tracked", Namespace: "default", UID: "its", Generation: 1,
				Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}},
				Spec: workloadsv1.InstanceSetSpec{Replicas: ptr.To[int32](0), EnableInstanceAPI: ptr.To(instanceAPI),
					Selector:         &metav1.LabelSelector{MatchLabels: map[string]string{"app": "tracked"}},
					Template:         corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql"}}}},
					OfflineInstances: []string{"tracked-0", "tracked-1", "tracked-2"}}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloadsv1.InstanceSet{}, &workloadsv1.Instance{}, &corev1.Pod{}).WithObjects(its).Build()
			key := client.ObjectKeyFromObject(its)
			if err := cli.Get(ctx, key, its); err != nil {
				t.Fatal(err)
			}
			facts := []*bool{nil, ptr.To(false), ptr.To(true)}
			its.Status.ObservedGeneration = its.Generation
			for i, name := range its.Spec.OfflineInstances {
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloadsv1.InstanceStatus{PodName: name, DataLoaded: facts[i], MemberJoined: facts[i],
					CurrentState: workloadsv1.InstanceCurrentStatePresent, CurrentRevision: "stale", Ready: true, Available: true, Failed: true, Role: "stale"})
			}
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			owner := []metav1.OwnerReference{{APIVersion: workloadsv1.GroupVersion.String(), Kind: workloadsv1.InstanceSetKind, Name: its.Name, UID: its.UID, Controller: ptr.To(true)}}
			meta := metav1.ObjectMeta{Name: "tracked-0", Namespace: its.Namespace, Labels: instanceset.GetMatchLabels(its.Name), OwnerReferences: owner}
			var observed client.Object
			if instanceAPI {
				observed = &workloadsv1.Instance{ObjectMeta: meta}
			} else {
				meta.Labels[appsv1.ControllerRevisionHashLabelKey] = "observed"
				observed = &corev1.Pod{ObjectMeta: meta, Spec: its.Spec.Template.Spec}
			}
			if err := cli.Create(ctx, observed); err != nil {
				t.Fatal(err)
			}
			if instanceAPI {
				inst := observed.(*workloadsv1.Instance)
				inst.Status = workloadsv1.InstanceStatus2{CurrentState: workloadsv1.InstanceCurrentStatePresent, CurrentRevision: "observed", Ready: true, Available: true}
				if err := cli.Status().Update(ctx, inst); err != nil {
					t.Fatal(err)
				}
			}
			reconcile := func() {
				t.Helper()
				recorder := record.NewFakeRecorder(100)
				var err error
				if instanceAPI {
					_, err = (&InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key})
				} else {
					_, err = (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key})
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, key, its); err != nil {
					t.Fatal(err)
				}
			}
			reconcile()
			assertPersistedTrackedFacts(t, its, facts)
			status := its.FindInstanceStatus("tracked-0")
			if status == nil || !status.Provisioned || status.CurrentState != workloadsv1.InstanceCurrentStatePresent || status.CurrentRevision != "observed" {
				t.Fatalf("Reconcile did not commit the observed runtime fact: %#v", status)
			}
			if err := client.IgnoreNotFound(cli.Delete(ctx, observed)); err != nil {
				t.Fatal(err)
			}
			reconcile()
			assertPersistedTrackedFacts(t, its, facts)
			status = its.FindInstanceStatus("tracked-0")
			if status == nil || !status.Provisioned || status.CurrentState != workloadsv1.InstanceCurrentStateAbsent || status.CurrentRevision != "" || status.Ready || status.Available || status.Failed || status.Role != "" {
				t.Fatalf("fresh reconciler did not retain facts and clear runtime fields: %#v", status)
			}
			its.Spec.OfflineInstances = nil
			its.Generation++
			if err := cli.Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			reconcile()
			reconcile()
			if len(its.Status.InstanceStatus) != 1 {
				t.Fatalf("released nil/false membership retained history: %#v", its.Status.InstanceStatus)
			}
			joined := its.FindInstanceStatus("tracked-2")
			if joined == nil || joined.DesiredState != workloadsv1.InstanceDesiredStateReleased || joined.CurrentState != workloadsv1.InstanceCurrentStateAbsent || !ptr.Deref(joined.DataLoaded, false) || !ptr.Deref(joined.MemberJoined, false) {
				t.Fatalf("recorded membership was lost after allocation removal: %#v", joined)
			}
			joined.MemberJoined = ptr.To(false)
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			reconcile()
			if len(its.Status.InstanceStatus) != 0 {
				t.Fatalf("recorded leave did not release persisted history: %#v", its.Status.InstanceStatus)
			}
		})
	}
}

func assertPersistedTrackedFacts(t *testing.T, its *workloadsv1.InstanceSet, facts []*bool) {
	t.Helper()
	for i, name := range []string{"tracked-0", "tracked-1", "tracked-2"} {
		status := its.FindInstanceStatus(name)
		if status == nil || status.DesiredState != workloadsv1.InstanceDesiredStateOffline || !reflect.DeepEqual(status.DataLoaded, facts[i]) || !reflect.DeepEqual(status.MemberJoined, facts[i]) || status.Provisioned != (i == 0) {
			t.Fatalf("committed nullable facts or provisioning changed for %s: %#v", name, status)
		}
	}
}
