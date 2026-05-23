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

package instanceset

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var stableEventTimeBase = time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)

func nowMicroAfter(base time.Time, offsetSeconds int64) metav1.MicroTime {
	return metav1.MicroTime{Time: base.Add(time.Duration(offsetSeconds) * time.Second)}
}

var _ = Describe("pod role label event handler test", func() {
	newRoleProbeEvent := func(pod *corev1.Pod, eventName, role string, code int32) *corev1.Event {
		message, err := json.Marshal(proto.ProbeEvent{
			Probe:   "roleProbe",
			Code:    code,
			Output:  []byte(role),
			Message: "mock role probe event",
		})
		Expect(err).ShouldNot(HaveOccurred())
		return builder.NewEventBuilder(namespace, eventName).
			SetInvolvedObject(corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  pod.Namespace,
				Name:       pod.Name,
				UID:        pod.UID,
				FieldPath:  proto.ProbeEventFieldPath,
			}).
			SetReason("roleProbe").
			SetMessage(string(message)).
			SetReportingController(proto.ProbeEventReportingController).
			GetObject()
	}
	newRoleProbeEventWithObservationVersion := func(pod *corev1.Pod, eventName, role string, code int32, observationVersion uint64) *corev1.Event {
		message, err := json.Marshal(proto.ProbeEvent{
			Probe:              "roleProbe",
			Code:               code,
			Output:             []byte(role),
			Message:            "mock role probe event",
			ObservationVersion: observationVersion,
		})
		Expect(err).ShouldNot(HaveOccurred())
		return builder.NewEventBuilder(namespace, eventName).
			SetInvolvedObject(corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  pod.Namespace,
				Name:       pod.Name,
				UID:        pod.UID,
				FieldPath:  proto.ProbeEventFieldPath,
			}).
			SetReason("roleProbe").
			SetMessage(string(message)).
			SetReportingController(proto.ProbeEventReportingController).
			GetObject()
	}

	Context("Handle function", func() {
		It("should work well", func() {
			cli := k8sMock
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			pod := builder.NewPodBuilder(namespace, getPodName(name, 0)).SetUID(uid).GetObject()
			pod.ResourceVersion = "1"
			role := workloads.ReplicaRole{
				Name:                 "leader",
				ParticipatesInQuorum: true,
				UpdatePriority:       5,
			}

			By("build an expected message")
			event := newRoleProbeEvent(pod, "foo", role.Name, 0)

			handler := &PodRoleEventHandler{}
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					p.Namespace = objKey.Namespace
					p.Name = objKey.Name
					p.UID = pod.UID
					p.Labels = map[string]string{
						constant.AppInstanceLabelKey: name,
						WorkloadsInstanceLabelKey:    name,
					}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = []workloads.ReplicaRole{role}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd).ShouldNot(BeNil())
					Expect(pd.Labels).ShouldNot(BeNil())
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(role.Name))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, patch client.Patch, _ ...client.PatchOption) error {
					Expect(evt).ShouldNot(BeNil())
					Expect(evt.Annotations).ShouldNot(BeNil())
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)
			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())

			By("build an unexpected message")
			event = newRoleProbeEvent(pod, "foo", role.Name, 0)
			event.Message = "unexpected message"
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, patch client.Patch, _ ...client.PatchOption) error {
					Expect(evt).ShouldNot(BeNil())
					Expect(evt.Annotations).ShouldNot(BeNil())
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)
			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())

			By("read a stale pod")
			event = newRoleProbeEvent(pod, "foo", role.Name, 0)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					p.Namespace = objKey.Namespace
					p.ResourceVersion = "0"
					p.Name = objKey.Name
					p.UID = pod.UID
					p.Labels = map[string]string{
						constant.AppInstanceLabelKey: name,
						WorkloadsInstanceLabelKey:    name,
					}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = []workloads.ReplicaRole{role}
					return nil
				}).Times(1)
			updateErr := fmt.Errorf("the object has been modified; please apply your changes to the latest version and try again")
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd).ShouldNot(BeNil())
					Expect(pd.Labels).ShouldNot(BeNil())
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(role.Name))
					if pd.ResourceVersion <= pod.ResourceVersion {
						return updateErr
					}
					return nil
				}).Times(1)
			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Equal(updateErr))
		})
	})

	Context("exclusive role", func() {
		var (
			cli     client.Client
			reqCtx  intctrlutil.RequestCtx
			pod     *corev1.Pod
			handler *PodRoleEventHandler
		)

		BeforeEach(func() {
			cli = k8sMock
			reqCtx = intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			pod = builder.NewPodBuilder(namespace, getPodName(name, 0)).SetUID(uid).GetObject()
			pod.ResourceVersion = "1"
			handler = &PodRoleEventHandler{}
		})

		It("should remove exclusive role labels from other pods when a new pod claims exclusive role", func() {
			// Create an exclusive role
			exclusiveRole := workloads.ReplicaRole{
				Name:                 "leader",
				ParticipatesInQuorum: true,
				UpdatePriority:       5,
				IsExclusive:          true, // Exclusive role
			}

			// Create an event for the new pod claiming the exclusive role
			event := newRoleProbeEvent(pod, "foo", exclusiveRole.Name, 0)

			// Mock other pods with the same exclusive role label
			otherPod1 := builder.NewPodBuilder(namespace, getPodName(name, 1)).
				SetUID("uid-other-1").
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, exclusiveRole.Name).
				GetObject()
			otherPod2 := builder.NewPodBuilder(namespace, getPodName(name, 2)).
				SetUID("uid-other-2").
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, exclusiveRole.Name).
				GetObject()

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					p.Namespace = objKey.Namespace
					p.Name = objKey.Name
					p.UID = pod.UID
					p.Labels = map[string]string{
						constant.AppManagedByLabelKey: constant.AppName,
						WorkloadsManagedByLabelKey:    workloads.InstanceSetKind,
						WorkloadsInstanceLabelKey:     name,
					}
					return nil
				}).Times(1)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = []workloads.ReplicaRole{exclusiveRole}
					return nil
				}).Times(1)

			// Expect update for the main pod (adding role label)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd).ShouldNot(BeNil())
					Expect(pd.Labels).ShouldNot(BeNil())
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(exclusiveRole.Name))
					return nil
				}).Times(1)

			// Expect list call to find other pods with the same exclusive role
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, podList *corev1.PodList, opts ...client.ListOption) error {
					podList.Items = []corev1.Pod{*otherPod1, *otherPod2}
					return nil
				}).Times(1)

			// Expect updates to remove role labels from other pods
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd).ShouldNot(BeNil())
					// Should be one of the other pods (not the main pod)
					Expect(pd.Name).ShouldNot(Equal(pod.Name))
					// Role label should be removed
					Expect(pd.Labels).ShouldNot(HaveKey(RoleLabelKey))
					return nil
				}).Times(2) // Two other pods

			// Expect event patch
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, patch client.Patch, _ ...client.PatchOption) error {
					Expect(evt).ShouldNot(BeNil())
					Expect(evt.Annotations).ShouldNot(BeNil())
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should not patch Pod when ObservationVersion is unchanged across periodic refresh", func() {
			roleName := "primary"
			// Pod already has annotation in new "obs:1" format and matching label.
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, roleName).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, "obs:1").
				GetObject()

			event := newRoleProbeEventWithObservationVersion(existingPod, "periodic-refresh", roleName, 0, 1)
			event.Count = 7
			event.EventTime = nowMicroAfter(stableEventTimeBase, 30)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			// Pod must NOT be updated when ObservationVersion is unchanged.
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				Times(0)
			// Event itself is still patched to mark it as handled.
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should patch Pod and advance annotation when ObservationVersion bumps with role change", func() {
			oldRole := "primary"
			newRole := "secondary"
			roles := []workloads.ReplicaRole{
				{Name: oldRole, ParticipatesInQuorum: true, UpdatePriority: 5, IsExclusive: true},
				{Name: newRole, ParticipatesInQuorum: true, UpdatePriority: 3},
			}
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, oldRole).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, "obs:1").
				GetObject()

			event := newRoleProbeEventWithObservationVersion(existingPod, "role-change", newRole, 0, 2)
			event.Count = 1
			event.EventTime = nowMicroAfter(stableEventTimeBase, 60)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = roles
					return nil
				}).Times(1)
			// Pod role label and annotation must transition to the bumped ObservationVersion.
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(newRole))
					Expect(pd.Annotations[constant.LastRoleEventVersionAnnotationKey]).Should(Equal("obs:2"))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
					podList.Items = nil
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should reject a stale lower ObservationVersion event so a parallel writer cannot re-acquire an exclusive role", func() {
			// Pod has already advanced past obs:10 via the new path; the
			// exclusive primary label was cleared on the prior reconcile.
			// A stale event from before that advance (obs:5, output=primary)
			// must not let the pod re-acquire primary, regardless of which
			// reconciler eventually delivers it. This pins the single-writer
			// contract: PodRoleEventHandler is the gate, and any other code
			// path that mutates the role label without going through the
			// ObservationVersion staleness check would violate the contract.
			roleName := "primary"
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, "obs:10").
				GetObject()

			event := newRoleProbeEventWithObservationVersion(existingPod, "stale-lower-obs", roleName, 0, 5)
			event.Count = 1
			event.EventTime = nowMicroAfter(stableEventTimeBase, 180)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			// Pod must NOT be updated; the stale lower obs event cannot grant
			// primary back to a pod the new path already de-promoted.
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				Times(0)
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should accept the upgrade path when ObservationVersion event arrives at a Pod still annotated with a legacy bare EventTime", func() {
			oldRole := "primary"
			newRole := "secondary"
			roles := []workloads.ReplicaRole{
				{Name: oldRole, ParticipatesInQuorum: true, UpdatePriority: 5, IsExclusive: true},
				{Name: newRole, ParticipatesInQuorum: true, UpdatePriority: 3},
			}
			// Pod has the previous legacy EventTime-micros annotation written by
			// an older controller; the next event from a new kbagent carries
			// ObservationVersion and must install the new "obs:<n>" format.
			legacyEventTimeMicros := stableEventTimeBase.UnixMicro()
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, oldRole).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, fmt.Sprintf("%d", legacyEventTimeMicros)).
				GetObject()

			postUpgradeVersion := uint64(1779550100000000)
			event := newRoleProbeEventWithObservationVersion(existingPod, "mixed-upgrade", newRole, 0, postUpgradeVersion)
			event.Count = 1
			event.EventTime = nowMicroAfter(stableEventTimeBase, 120)
			expectedVersion := fmt.Sprintf("obs:%d", postUpgradeVersion)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = roles
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(newRole))
					Expect(pd.Annotations[constant.LastRoleEventVersionAnnotationKey]).Should(Equal(expectedVersion))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
					podList.Items = nil
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should reject a legacy EventTime event when the Pod annotation already records a new obs format", func() {
			roleName := "primary"
			// Pod already records a new "obs:<n>" annotation from a prior
			// upgraded event. A legacy emitter (ObservationVersion=0) arrives
			// afterwards. The new format must not be overwritten by a bare
			// EventTime version.
			recordedObsVersion := uint64(1779550200000000)
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, roleName).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, fmt.Sprintf("obs:%d", recordedObsVersion)).
				GetObject()

			event := newRoleProbeEventWithObservationVersion(existingPod, "mixed-downgrade", roleName, 0, 0)
			event.Count = 3
			event.EventTime = nowMicroAfter(stableEventTimeBase, 150)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			// Pod must NOT be updated; the new "obs:<n>" annotation has to
			// survive a legacy event arriving out of order.
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				Times(0)
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should accept a higher ObservationVersion after kbagent restart even when the prior annotation has a small obs value", func() {
			oldRole := "primary"
			newRole := "secondary"
			roles := []workloads.ReplicaRole{
				{Name: oldRole, ParticipatesInQuorum: true, UpdatePriority: 5, IsExclusive: true},
				{Name: newRole, ParticipatesInQuorum: true, UpdatePriority: 3},
			}
			// Pod still carries a small-counter obs annotation from an earlier
			// kbagent (counter-style) build.
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, oldRole).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, "obs:5").
				GetObject()

			// Restart-safe ObservationVersion (wall-clock micros) is far larger
			// than the old counter value; controller must not treat it as stale.
			postRestartVersion := uint64(1779550000000000)
			event := newRoleProbeEventWithObservationVersion(existingPod, "post-restart", newRole, 0, postRestartVersion)
			event.Count = 1
			event.EventTime = nowMicroAfter(stableEventTimeBase, 600)
			expectedVersion := fmt.Sprintf("obs:%d", postRestartVersion)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = roles
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(newRole))
					Expect(pd.Annotations[constant.LastRoleEventVersionAnnotationKey]).Should(Equal(expectedVersion))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
					podList.Items = nil
					return nil
				}).AnyTimes()
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should fall back to EventTime-based behavior when ObservationVersion is zero", func() {
			roleName := "primary"
			role := workloads.ReplicaRole{
				Name:                 roleName,
				ParticipatesInQuorum: true,
				UpdatePriority:       5,
			}
			// Pod already has a legacy EventTime-micros annotation.
			legacyEventTimeMicros := stableEventTimeBase.UnixMicro()
			existingPod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
				AddLabels(WorkloadsInstanceLabelKey, name).
				AddLabels(RoleLabelKey, roleName).
				AddAnnotations(constant.LastRoleEventVersionAnnotationKey, fmt.Sprintf("%d", legacyEventTimeMicros)).
				GetObject()

			// Legacy ObservationVersion=0 event with a newer EventTime.
			event := newRoleProbeEventWithObservationVersion(existingPod, "legacy", roleName, 0, 0)
			event.Count = 9
			event.EventTime = nowMicroAfter(stableEventTimeBase, 90)
			expectedVersion := fmt.Sprintf("%d", event.EventTime.UnixMicro())

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					*p = *existingPod
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = []workloads.ReplicaRole{role}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(roleName))
					Expect(pd.Annotations[constant.LastRoleEventVersionAnnotationKey]).Should(Equal(expectedVersion))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, _ client.Patch, _ ...client.PatchOption) error {
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("should not remove role labels when role is not exclusive", func() {
			// Create a non-exclusive role
			nonExclusiveRole := workloads.ReplicaRole{
				Name:                 "follower",
				ParticipatesInQuorum: true,
				UpdatePriority:       3,
				IsExclusive:          false, // Not exclusive
			}

			// Create an event for the new pod claiming the non-exclusive role
			event := newRoleProbeEvent(pod, "foo", nonExclusiveRole.Name, 0)

			// Expectations
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, p *corev1.Pod, _ ...client.GetOption) error {
					p.Namespace = objKey.Namespace
					p.Name = objKey.Name
					p.UID = pod.UID
					p.Labels = map[string]string{
						constant.AppManagedByLabelKey: constant.AppName,
						WorkloadsManagedByLabelKey:    workloads.InstanceSetKind,
						WorkloadsInstanceLabelKey:     name,
					}
					return nil
				}).Times(1)

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, its *workloads.InstanceSet, _ ...client.GetOption) error {
					its.Namespace = objKey.Namespace
					its.Name = objKey.Name
					its.Spec.Roles = []workloads.ReplicaRole{nonExclusiveRole}
					return nil
				}).Times(1)

			// Expect update for the main pod only (no list or updates for other pods)
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, pd *corev1.Pod, _ ...client.UpdateOption) error {
					Expect(pd).ShouldNot(BeNil())
					Expect(pd.Labels).ShouldNot(BeNil())
					Expect(pd.Labels[RoleLabelKey]).Should(Equal(nonExclusiveRole.Name))
					return nil
				}).Times(1)

			// Should NOT call List (since role is not exclusive)
			// Should NOT call Update for other pods

			// Expect event patch
			k8sMock.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, evt *corev1.Event, patch client.Patch, _ ...client.PatchOption) error {
					Expect(evt).ShouldNot(BeNil())
					Expect(evt.Annotations).ShouldNot(BeNil())
					Expect(evt.Annotations[roleChangedAnnotKey]).Should(Equal(fmt.Sprintf("count-%d", evt.Count)))
					return nil
				}).Times(1)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})
	})
})
