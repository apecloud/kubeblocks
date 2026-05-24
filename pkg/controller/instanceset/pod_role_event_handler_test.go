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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
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
		// kbagent roleProbe events are owned by InstanceEventReconciler in
		// controllers/workloads since the multi-cluster Instance API refactor
		// (#9697). PodRoleEventHandler must not race-write the Pod role
		// label on those events; the engine-authoritative kb-role-version
		// staleness gate lives only in InstanceEventReconciler. Earlier
		// versions of this handler wrote the role label here without the new
		// gate, which let stale role observations override a freshly-demoted
		// pod's label during failover.
		It("must not touch the Pod when handed a kbagent roleProbe event", func() {
			cli := k8sMock
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			pod := builder.NewPodBuilder(namespace, getPodName(name, 0)).SetUID(uid).GetObject()
			event := newRoleProbeEvent(pod, "kbagent-role-event", "primary", 0)

			handler := &PodRoleEventHandler{}
			// No client calls of any kind are expected: the handler must be a
			// silent no-op so InstanceEventReconciler stays the sole writer.
			k8sMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			Expect(handler.Handle(cli, reqCtx, nil, event)).Should(Succeed())
		})

		It("must be a no-op for non-kbagent events as well", func() {
			cli := k8sMock
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			otherEvent := builder.NewEventBuilder(namespace, "unrelated-event").
				SetReason("SomeOtherReason").
				SetReportingController("some-other-controller").
				GetObject()

			handler := &PodRoleEventHandler{}
			k8sMock.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			k8sMock.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			Expect(handler.Handle(cli, reqCtx, nil, otherEvent)).Should(Succeed())
		})
	})
})
