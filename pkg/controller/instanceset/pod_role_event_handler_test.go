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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

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

		Context("Bug B stale plain per-pod role event", func() {
			const (
				primaryRole   = "primary"
				secondaryRole = "secondary"
				// A ":" event version exercises the stale-check carve-out for
				// non-plain versions while the plain Event still uses EventTime.
				authoritativeRoleEventVersion = "term:10:gen:20"
				stalePlainEventTimeMicros     = int64(200)
			)

			newExclusiveRole := func() workloads.ReplicaRole {
				return workloads.ReplicaRole{
					Name:                 primaryRole,
					ParticipatesInQuorum: true,
					UpdatePriority:       5,
					IsExclusive:          true,
				}
			}

			newInstanceSet := func(role workloads.ReplicaRole) *workloads.InstanceSet {
				return &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					},
					Spec: workloads.InstanceSetSpec{
						Roles: []workloads.ReplicaRole{role},
					},
				}
			}

			newRolefulPod := func(ordinal int, uid types.UID, role string) *corev1.Pod {
				return builder.NewPodBuilder(namespace, getPodName(name, ordinal)).
					SetUID(uid).
					AddLabels(constant.AppManagedByLabelKey, constant.AppName).
					AddLabels(WorkloadsManagedByLabelKey, workloads.InstanceSetKind).
					AddLabels(WorkloadsInstanceLabelKey, name).
					AddLabels(RoleLabelKey, role).
					AddAnnotations(constant.LastRoleEventVersionAnnotationKey, authoritativeRoleEventVersion).
					GetObject()
			}

			newPlainRoleProbeEvent := func(pod *corev1.Pod, roleName string) *corev1.Event {
				message, err := json.Marshal(proto.ProbeEvent{
					Probe:   "roleProbe",
					Code:    0,
					Output:  []byte(roleName),
					Message: "mock role probe event",
				})
				Expect(err).ShouldNot(HaveOccurred())
				event := builder.NewEventBuilder(namespace, "bug-b-stale-plain-role").
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
					SetEventTime(metav1.MicroTime{Time: time.Unix(0, stalePlainEventTimeMicros*int64(time.Microsecond))}).
					GetObject()
				event.Count = 2
				return event
			}

			newBugBScenario := func() (client.Client, *corev1.Pod, *corev1.Pod) {
				role := newExclusiveRole()
				its := newInstanceSet(role)
				candidate := newRolefulPod(0, types.UID("candidate-pod-uid"), secondaryRole)
				holder := newRolefulPod(1, types.UID("holder-pod-uid"), primaryRole)
				event := newPlainRoleProbeEvent(candidate, primaryRole)
				cli := fake.NewClientBuilder().
					WithScheme(model.GetScheme()).
					WithObjects(its, candidate, holder, event).
					Build()
				reqCtx := intctrlutil.RequestCtx{
					Ctx: ctx,
					Log: logger,
				}
				handler := &PodRoleEventHandler{}
				Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
				return cli, candidate, holder
			}

			// This RED abstracts the shared Bug B contract: a plain per-pod old fact
			// must not acquire an exclusive role or evict the current holder by using
			// a fresh EventTime alone. A failing Go test demonstrates a controller
			// contract gap; it does not independently prove the live root cause final.
			It("should not promote the event pod from a stale plain per-pod exclusive role event", func() {
				cli, candidate, _ := newBugBScenario()
				gotCandidate := &corev1.Pod{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(candidate), gotCandidate)).Should(Succeed())
				Expect(gotCandidate.Labels[RoleLabelKey]).Should(Equal(secondaryRole))
				Expect(gotCandidate.Annotations[constant.LastRoleEventVersionAnnotationKey]).
					Should(Equal(authoritativeRoleEventVersion))
			})

			It("should not evict the existing exclusive role holder from a stale plain per-pod event", func() {
				cli, _, holder := newBugBScenario()
				gotHolder := &corev1.Pod{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(holder), gotHolder)).Should(Succeed())
				Expect(gotHolder.Labels[RoleLabelKey]).Should(Equal(primaryRole))
				Expect(gotHolder.Annotations[constant.LastRoleEventVersionAnnotationKey]).
					Should(Equal(authoritativeRoleEventVersion))
			})
		})
	})
})
