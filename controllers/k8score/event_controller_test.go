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

package k8score

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
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
		initLastTS = time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC)
		eventSeq   = 0
	)

	createRoleChangedEvent := func(podName, role string, podUid types.UID) *corev1.Event {
		eventSeq++
		message, err := json.Marshal(proto.ProbeEvent{
			Probe:  "roleProbe",
			Code:   0,
			Output: []byte(role),
		})
		Expect(err).ShouldNot(HaveOccurred())
		objectRef := corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  testCtx.DefaultNamespace,
			Name:       podName,
			UID:        podUid,
			FieldPath:  proto.ProbeEventFieldPath,
		}
		eventName := fmt.Sprintf("%s.%d", podName, eventSeq)
		return builder.NewEventBuilder(testCtx.DefaultNamespace, eventName).
			SetInvolvedObject(objectRef).
			SetMessage(string(message)).
			SetReason("roleProbe").
			SetType(corev1.EventTypeNormal).
			SetFirstTimestamp(metav1.NewTime(initLastTS)).
			SetLastTimestamp(metav1.NewTime(initLastTS)).
			SetEventTime(metav1.NewMicroTime(initLastTS)).
			SetReportingController(proto.ProbeEventReportingController).
			SetReportingInstance(podName).
			SetAction("mock-create-event-action").
			GetObject()
	}

	createInvolvedPod := func(name, clusterName, componentName, itsName string) *corev1.Pod {
		return builder.NewPodBuilder(testCtx.DefaultNamespace, name).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			AddLabels(constant.KBAppComponentLabelKey, componentName).
			AddLabels(instanceset.WorkloadsInstanceLabelKey, itsName).
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
		// kbagent roleProbe events are owned by InstanceEventReconciler in
		// controllers/workloads since the multi-cluster Instance API refactor
		// (#9697). EventReconciler skips them at the outer guard so the
		// shared kubeblocks.io/event-handled annotation is not stamped here;
		// otherwise InstanceEventReconciler's outer short-circuit would see
		// the event already handled and silently drop it. Pod role label
		// writes for these events are now exclusively the responsibility of
		// InstanceEventReconciler and its engine-authoritative staleness
		// gate (covered by controllers/workloads tests).
		It("should skip kbagent roleProbe events without writing the pod role label or marking the event handled", func() {
			By("create cluster & compdef")
			compDefName := "test-compdef"
			clusterName := "test-cluster"
			defaultCompName := "mysql"
			compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
				SetDefaultSpec().
				Create(&testCtx).
				GetObject()
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddComponent(defaultCompName, compDefObj.GetName()).
				Create(&testCtx).GetObject()
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(clusterObj), &appsv1.Cluster{}, true)).Should(Succeed())

			itsName := fmt.Sprintf("%s-%s", clusterObj.Name, defaultCompName)
			its := testapps.NewInstanceSetFactory(clusterObj.Namespace, itsName, clusterObj.Name, defaultCompName).
				SetReplicas(int32(3)).
				AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
				Create(&testCtx).GetObject()
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(its), func(tmpITS *workloads.InstanceSet) {
				tmpITS.Spec.Roles = []workloads.ReplicaRole{
					{
						Name:                 "leader",
						ParticipatesInQuorum: true,
						UpdatePriority:       5,
					},
					{
						Name:                 "follower",
						ParticipatesInQuorum: true,
						UpdatePriority:       4,
					},
				}
			})()).Should(Succeed())

			By("create involved pod")
			var uid types.UID
			podName := fmt.Sprintf("%s-%d", itsName, 0)
			pod := createInvolvedPod(podName, clusterObj.Name, defaultCompName, itsName)
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

			By("send a kbagent roleProbe event")
			sndEvent := createRoleChangedEvent(podName, "leader", uid)
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

			By("the pod role label and last-role-snapshot-version annotation must stay untouched")
			Consistently(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pod), func(g Gomega, p *corev1.Pod) {
				g.Expect(p).ShouldNot(BeNil())
				if p.Labels != nil {
					g.Expect(p.Labels).ShouldNot(HaveKey(constant.RoleLabelKey))
				}
				if p.Annotations != nil {
					g.Expect(p.Annotations).ShouldNot(HaveKey(constant.LastRoleEventVersionAnnotationKey))
				}
			}), 2*time.Second, 200*time.Millisecond).Should(Succeed())

			By("the event must not be stamped with the EventReconciler handled annotation")
			Consistently(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sndEvent), func(g Gomega, e *corev1.Event) {
				g.Expect(e).ShouldNot(BeNil())
				if e.Annotations != nil {
					g.Expect(e.Annotations).ShouldNot(HaveKey(eventHandledAnnotationKey))
				}
			}), 2*time.Second, 200*time.Millisecond).Should(Succeed())
		})
	})
})
