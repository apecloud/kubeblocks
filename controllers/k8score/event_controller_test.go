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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Event Controller", func() {
	cleanEnv := func() {
		By("clean resources")

		testapps.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, generics.EventSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
	}

	var eventSeq int

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
			SetEventTime(metav1.NewMicroTime(metav1.Now().Time)).
			SetReportingController(proto.ProbeEventReportingController).
			SetReportingInstance(podName).
			SetAction("mock-create-event-action").
			GetObject()
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("When receiving role probe event", func() {
		It("should leave role probe events for the workloads role event reconciler", func() {
			sndEvent := createRoleChangedEvent("pod-0", "leader", types.UID("pod-uid"))
			Expect(testCtx.CreateObj(ctx, sndEvent)).Should(Succeed())

			Consistently(func(g Gomega) {
				event := &corev1.Event{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sndEvent.Namespace,
					Name:      sndEvent.Name,
				}, event)).Should(Succeed())
				g.Expect(event.Annotations).ShouldNot(HaveKey(constant.EventHandledAnnotationKey))
			}).Should(Succeed())
		})
	})
})
