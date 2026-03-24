/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package service

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("probe", func() {
	Context("probe", func() {
		var (
			probeName = "roleProbe"
			actions   = []proto.Action{
				{
					Name: probeName,
					Exec: &proto.ExecAction{
						Commands: []string{"/bin/bash", "-c", "echo -n leader"},
					},
				},
			}
			probes = []proto.Probe{
				{
					Action:              probeName,
					InitialDelaySeconds: 0,
					PeriodSeconds:       1,
					SuccessThreshold:    1,
					FailureThreshold:    1,
					ReportPeriodSeconds: 0,
				},
			}

			actionSvc *actionService
		)

		BeforeEach(func() {
			var err error
			actionSvc, err = newActionService(logr.New(nil), actions)
			Expect(err).Should(BeNil())
		})

		It("new", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())
			Expect(service.Kind()).Should(Equal(proto.ServiceProbe.Kind))
		})

		It("start", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			Expect(service.Start()).Should(Succeed())
			Expect(len(service.probes)).Should(Equal(len(service.runners)))
		})

		It("handle request", func() {
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			_, err = service.HandleRequest(ctx, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrNotImplemented)).Should(BeTrue())
		})

		It("initial delay seconds", func() {
			probes[0].InitialDelaySeconds = 60
			defer func() { probes[0].InitialDelaySeconds = 0 }()

			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			Expect(service.Start()).Should(Succeed())

			time.Sleep(1 * time.Second)
			r := service.runners[probeName]
			Expect(r).ShouldNot(BeNil())
			Expect(r.ticker).Should(BeNil())
		})

		It("send event", func() {
			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			By("mock send event function")
			eventChan := make(chan struct {
				reason  string
				message string
			}, 128)
			service.sendEventWithMessage = func(_ *logr.Logger, reason string, message string, _ bool) error {
				eventChan <- struct{ reason, message string }{reason, message}
				return nil
			}

			By("start probe service")
			Expect(service.Start()).Should(Succeed())

			By("check received event")
			var receivedData struct{ reason, message string }
			Eventually(eventChan).Should(Receive(&receivedData))
			Expect(receivedData.reason).Should(Equal(probeName))
			var event proto.ProbeEvent
			Expect(json.Unmarshal([]byte(receivedData.message), &event)).Should(Succeed())
			Eventually(event.Probe).Should(Equal(probeName))
			Eventually(event.Code).Should(Equal(int32(0)))
			Eventually(event.Output).Should(Equal([]byte("leader")))
		})

		It("send event - API server error", func() {
			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			By("mock send event function with error")
			var (
				count = 0
			)
			service.sendEventWithMessage = func(_ *logr.Logger, reason string, message string, _ bool) error {
				count += 1
				return fmt.Errorf("API server error")
			}
			retrySendEventInterval = 1 * time.Second
			defer func() { retrySendEventInterval = defaultRetrySendEventInterval }()

			By("start probe service")
			Expect(service.Start()).Should(Succeed())

			By("wait for probe to send event error")
			Eventually(func() int { return count }, 2*retrySendEventInterval).Should(BeNumerically(">", 1))
		})

		It("send event - after API server recover", func() {
			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			By("mock send event function with temporary error")
			var (
				count     = 0
				eventChan = make(chan struct {
					reason  string
					message string
				}, 128)
			)
			service.sendEventWithMessage = func(_ *logr.Logger, reason string, message string, _ bool) error {
				count += 1
				if count <= 2 {
					return fmt.Errorf("API server error")
				}
				eventChan <- struct{ reason, message string }{reason, message}
				return nil
			}
			retrySendEventInterval = 1 * time.Second
			defer func() { retrySendEventInterval = defaultRetrySendEventInterval }()

			By("start probe service")
			Expect(service.Start()).Should(Succeed())

			By("wait for probe to send event error")
			Eventually(func() int { return count }, 2*retrySendEventInterval).Should(BeNumerically(">", 1))

			By("check received event after recover")
			var receivedData struct{ reason, message string }
			Eventually(eventChan, 2*retrySendEventInterval).Should(Receive(&receivedData))
			Expect(receivedData.reason).Should(Equal(probeName))
			var event proto.ProbeEvent
			Expect(json.Unmarshal([]byte(receivedData.message), &event)).Should(Succeed())
			Eventually(event.Probe).Should(Equal(probeName))
			Eventually(event.Code).Should(Equal(int32(0)))
			Eventually(event.Output).Should(Equal([]byte("leader")))
		})

		// TODO: more test cases
	})
})
