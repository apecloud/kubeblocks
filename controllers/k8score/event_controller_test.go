/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package k8score

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	lorryutil "github.com/apecloud/kubeblocks/pkg/lorry/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Event Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		testapps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.EventSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
	}

	var (
		beforeLastTS = time.Date(2021, time.January, 1, 12, 0, 0, 0, time.UTC)
		initLastTS   = time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC)
		afterLastTS  = time.Date(2023, time.January, 1, 12, 0, 0, 0, time.UTC)
	)

	createRoleChangedEvent := func(podName, role string, podUid types.UID) *corev1.Event {
		seq, _ := password.Generate(16, 16, 0, true, true)
		objectRef := corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  testCtx.DefaultNamespace,
			Name:       podName,
			UID:        podUid,
		}
		eventName := strings.Join([]string{podName, seq}, ".")
		return builder.NewEventBuilder(testCtx.DefaultNamespace, eventName).
			SetInvolvedObject(objectRef).
			SetMessage(fmt.Sprintf("{\"event\":\"roleChanged\",\"originalRole\":\"secondary\",\"role\":\"%s\"}", role)).
			SetReason(string(lorryutil.CheckRoleOperation)).
			SetType(corev1.EventTypeNormal).
			SetFirstTimestamp(metav1.NewTime(initLastTS)).
			SetLastTimestamp(metav1.NewTime(initLastTS)).
			SetEventTime(metav1.NewMicroTime(initLastTS)).
			SetReportingController("lorry").
			SetReportingInstance(podName).
			SetAction("mock-create-event-action").
			GetObject()
	}

	createInvolvedPod := func(name, clusterName, componentName string) *corev1.Pod {
		return builder.NewPodBuilder(testCtx.DefaultNamespace, name).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			AddLabels(constant.KBAppComponentLabelKey, componentName).
			SetContainers([]corev1.Container{
				{
					Image: "foo",
					Name:  "bar",
				},
			}).
			GetObject()
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("When receiving role changed event", func() {
		It("should handle it properly", func() {
			By("create cluster & clusterDef")
			clusterDefName := "foo"
			consensusCompName := "consensus"
			consensusCompDefName := "consensus"
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, consensusCompDefName).
				Create(&testCtx).GetObject()
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, "",
				clusterDefObj.Name, "").WithRandomName().
				AddComponent(consensusCompName, consensusCompDefName).
				Create(&testCtx).GetObject()
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(clusterObj), &appsv1alpha1.Cluster{}, true)).Should(Succeed())

			rsmName := fmt.Sprintf("%s-%s", clusterObj.Name, consensusCompName)
			rsm := testapps.NewRSMFactory(clusterObj.Namespace, rsmName, clusterObj.Name, consensusCompName).
				SetReplicas(int32(3)).
				AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
				Create(&testCtx).GetObject()
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(rsm), func(tmpRSM *workloads.ReplicatedStateMachine) {
				tmpRSM.Spec.Roles = []workloads.ReplicaRole{
					{
						Name:       "leader",
						IsLeader:   true,
						AccessMode: workloads.ReadWriteMode,
						CanVote:    true,
					},
					{
						Name:       "follower",
						IsLeader:   false,
						AccessMode: workloads.ReadonlyMode,
						CanVote:    true,
					},
				}
			})()).Should(Succeed())
			By("create involved pod")
			var uid types.UID
			podName := fmt.Sprintf("%s-%d", rsmName, 0)
			pod := createInvolvedPod(podName, clusterObj.Name, consensusCompName)
			Expect(testCtx.CreateObj(ctx, pod)).Should(Succeed())
			Eventually(func() error {
				p := &corev1.Pod{}
				defer func() {
					uid = p.UID
				}()
				return k8sClient.Get(ctx, types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				}, p)
			}).Should(Succeed())
			Expect(uid).ShouldNot(BeNil())

			By("send role changed event")
			role := "leader"
			sndEvent := createRoleChangedEvent(podName, role, uid)
			Expect(testCtx.CreateObj(ctx, sndEvent)).Should(Succeed())
			Eventually(func() string {
				event := &corev1.Event{}
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndEvent.Namespace,
					Name:      sndEvent.Name,
				}, event); err != nil {
					return err.Error()
				}
				return event.InvolvedObject.Name
			}).Should(Equal(sndEvent.InvolvedObject.Name))

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pod), func(g Gomega, p *corev1.Pod) {
				g.Expect(p).ShouldNot(BeNil())
				g.Expect(p.Labels).ShouldNot(BeNil())
				g.Expect(p.Labels[constant.RoleLabelKey]).Should(Equal(role))
				g.Expect(p.Annotations[constant.LastRoleSnapshotVersionAnnotationKey]).Should(Equal(sndEvent.EventTime.Time.Format(time.RFC3339Nano)))
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sndEvent), func(g Gomega, e *corev1.Event) {
				g.Expect(e).ShouldNot(BeNil())
				g.Expect(e.Annotations).ShouldNot(BeNil())
				g.Expect(e.Annotations[roleChangedAnnotKey]).Should(Equal("count-0"))
			})).Should(Succeed())

			By("check whether the duration and number of events reach the threshold")
			Expect(IsOvertimeEvent(sndEvent, 5*time.Second)).Should(BeFalse())

			By("send role changed event with beforeLastTS earlier than pod last role changes event timestamp annotation should not be update successfully")
			role = "follower"
			sndInvalidEvent := createRoleChangedEvent(podName, role, uid)
			sndInvalidEvent.EventTime = metav1.NewMicroTime(beforeLastTS)
			Expect(testCtx.CreateObj(ctx, sndInvalidEvent)).Should(Succeed())
			Eventually(func() string {
				event := &corev1.Event{}
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndInvalidEvent.Namespace,
					Name:      sndInvalidEvent.Name,
				}, event); err != nil {
					return err.Error()
				}
				return event.InvolvedObject.Name
			}).Should(Equal(sndInvalidEvent.InvolvedObject.Name))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pod), func(g Gomega, p *corev1.Pod) {
				g.Expect(p).ShouldNot(BeNil())
				g.Expect(p.Labels).ShouldNot(BeNil())
				g.Expect(p.Labels[constant.RoleLabelKey]).ShouldNot(Equal(role))
				g.Expect(p.Annotations[constant.LastRoleSnapshotVersionAnnotationKey]).ShouldNot(Equal(sndInvalidEvent.EventTime.Time.Format(time.RFC3339Nano)))
			})).Should(Succeed())

			By("send role changed event with afterLastTS later than pod last role changes event timestamp annotation should be update successfully")
			role = "follower"
			sndValidEvent := createRoleChangedEvent(podName, role, uid)
			sndValidEvent.EventTime = metav1.NewMicroTime(afterLastTS)
			Expect(testCtx.CreateObj(ctx, sndValidEvent)).Should(Succeed())
			Eventually(func() string {
				event := &corev1.Event{}
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndValidEvent.Namespace,
					Name:      sndValidEvent.Name,
				}, event); err != nil {
					return err.Error()
				}
				return event.InvolvedObject.Name
			}).Should(Equal(sndValidEvent.InvolvedObject.Name))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pod), func(g Gomega, p *corev1.Pod) {
				g.Expect(p).ShouldNot(BeNil())
				g.Expect(p.Labels).ShouldNot(BeNil())
				g.Expect(p.Labels[constant.RoleLabelKey]).Should(Equal(role))
				g.Expect(p.Annotations[constant.LastRoleSnapshotVersionAnnotationKey]).Should(Equal(sndValidEvent.EventTime.Time.Format(time.RFC3339Nano)))
			})).Should(Succeed())
		})
	})

	Context("ParseProbeEventMessage function", func() {
		It("should work well", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: log.FromContext(ctx).WithValues("event", testCtx.DefaultNamespace),
			}
			event := createRoleChangedEvent("foo", "", "bar")
			event.Message = "not-a-role-message"
			eventMessage := ParseProbeEventMessage(reqCtx, event)
			Expect(eventMessage).Should(BeNil())
		})
	})

	Context("IsOvertimeEvent function", func() {
		It("should work well", func() {
			event := createRoleChangedEvent("foo", "", "bar")
			timeout := 50 * time.Millisecond
			event.FirstTimestamp = metav1.NewTime(time.Now())
			event.LastTimestamp = metav1.NewTime(time.Now())
			Expect(IsOvertimeEvent(event, timeout)).Should(BeFalse())
			event.LastTimestamp = metav1.NewTime(event.LastTimestamp.Time.Add(2 * timeout))
			Expect(IsOvertimeEvent(event, timeout)).Should(BeTrue())

			event.EventTime = metav1.NewMicroTime(time.Now())
			event.Series = &corev1.EventSeries{LastObservedTime: metav1.NewMicroTime(time.Now())}
			Expect(IsOvertimeEvent(event, timeout)).Should(BeFalse())
			event.Series = &corev1.EventSeries{LastObservedTime: metav1.NewMicroTime(time.Now().Add(2 * timeout))}
			Expect(IsOvertimeEvent(event, timeout)).Should(BeTrue())
		})
	})
})
