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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
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
			probes []proto.Probe

			actionSvc *actionService
		)

		BeforeEach(func() {
			probes = []proto.Probe{{
				Action:           probeName,
				PeriodSeconds:    1,
				SuccessThreshold: 1,
				FailureThreshold: 1,
			}}
			var err error
			actionSvc, err = newActionService(logr.New(nil), actions)
			Expect(err).Should(BeNil())
		})

		startService := func(service *probeService) {
			probeCtx, cancel := context.WithCancel(ctx)
			DeferCleanup(func() {
				cancel()
				done := make(chan struct{})
				go func() {
					service.wg.Wait()
					close(done)
				}()
				Eventually(done).Should(BeClosed())
			})
			Expect(service.start(probeCtx)).Should(Succeed())
		}

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

			startService(service)
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

			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			events := make(chan struct{}, 1)
			service.sendEventWithMessage = func(_ *logr.Logger, _, _ string, _ bool) error {
				events <- struct{}{}
				return nil
			}
			startService(service)
			Consistently(events, time.Second).ShouldNot(Receive())
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
			startService(service)

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

		It("normalizes probe periods without modifying the shared configuration", func() {
			probes[0].PeriodSeconds = 0
			probes[0].ReportPeriodSeconds = 1
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			events := make(chan struct{}, 1)
			service.sendEventWithMessage = func(_ *logr.Logger, _, _ string, _ bool) error {
				events <- struct{}{}
				return nil
			}
			startService(service)
			Eventually(events).Should(Receive())
			Expect(probes[0].PeriodSeconds).Should(BeZero())
			Expect(probes[0].ReportPeriodSeconds).Should(Equal(int32(1)))
		})

		It("send event when report-on-file-change trigger fires with unchanged output", func() {
			tmpDir, err := os.MkdirTemp("", "kbagent-file-change-*")
			Expect(err).Should(BeNil())
			DeferCleanup(os.RemoveAll, tmpDir)

			triggerPath := filepath.Join(tmpDir, "trigger")
			Expect(os.WriteFile(triggerPath, []byte("current"), 0644)).Should(Succeed())
			probesWithTrigger := append([]proto.Probe(nil), probes...)
			probesWithTrigger[0].ReportOnFileChange = []string{triggerPath}

			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probesWithTrigger)
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
			startService(service)

			By("drain initial event")
			var receivedData struct{ reason, message string }
			Eventually(eventChan).Should(Receive(&receivedData))

			By("update watched file")
			Expect(os.WriteFile(triggerPath, []byte("changed"), 0644)).Should(Succeed())

			By("check received event even though probe output has not changed")
			Eventually(eventChan, 5*time.Second).Should(Receive(&receivedData))
			Expect(receivedData.reason).Should(Equal(probeName))
			var event proto.ProbeEvent
			Expect(json.Unmarshal([]byte(receivedData.message), &event)).Should(Succeed())
			Expect(event.Probe).Should(Equal(probeName))
			Expect(event.Code).Should(Equal(int32(0)))
			Expect(event.Output).Should(Equal([]byte("leader")))
		})

		It("matches file-change events by configured path type", func() {
			dir := filepath.Clean("/tmp/watch-dir")
			file := filepath.Join(dir, "watched")
			sibling := filepath.Join(dir, "sibling")
			child := filepath.Join(dir, "child")
			nested := filepath.Join(child, "nested")

			Expect(watchedFileChanged(fsnotify.Event{Name: file, Op: fsnotify.Write},
				[]fileChangeWatch{{path: file}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: file, Op: fsnotify.Create},
				[]fileChangeWatch{{path: file}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: file, Op: fsnotify.Remove},
				[]fileChangeWatch{{path: file}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: file, Op: fsnotify.Rename},
				[]fileChangeWatch{{path: file}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: sibling, Op: fsnotify.Write},
				[]fileChangeWatch{{path: file}})).Should(BeFalse())

			Expect(watchedFileChanged(fsnotify.Event{Name: child, Op: fsnotify.Create},
				[]fileChangeWatch{{path: dir, dir: true}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: child, Op: fsnotify.Remove},
				[]fileChangeWatch{{path: dir, dir: true}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: child, Op: fsnotify.Rename},
				[]fileChangeWatch{{path: dir, dir: true}})).Should(BeTrue())
			Expect(watchedFileChanged(fsnotify.Event{Name: child, Op: fsnotify.Write},
				[]fileChangeWatch{{path: dir, dir: true}})).Should(BeFalse())
			Expect(watchedFileChanged(fsnotify.Event{Name: nested, Op: fsnotify.Create},
				[]fileChangeWatch{{path: dir, dir: true}})).Should(BeFalse())
		})

		It("send event - API server error", func() {
			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			By("mock send event function with error")
			var (
				count atomic.Int64
			)
			service.sendEventWithMessage = func(_ *logr.Logger, reason string, message string, _ bool) error {
				count.Add(1)
				return fmt.Errorf("API server error")
			}
			service.retrySendEventInterval = time.Second

			By("start probe service")
			startService(service)

			By("wait for probe to send event error")
			Eventually(count.Load, 2*service.retrySendEventInterval).Should(BeNumerically(">", 1))
		})

		It("send event - after API server recover", func() {
			By("create probe service")
			service, err := newProbeService(logr.New(nil), actionSvc, probes)
			Expect(err).Should(BeNil())
			Expect(service).ShouldNot(BeNil())

			By("mock send event function with temporary error")
			var (
				count     atomic.Int64
				eventChan = make(chan struct {
					reason  string
					message string
				}, 128)
			)
			service.sendEventWithMessage = func(_ *logr.Logger, reason string, message string, _ bool) error {
				if count.Add(1) <= 2 {
					return fmt.Errorf("API server error")
				}
				eventChan <- struct{ reason, message string }{reason, message}
				return nil
			}
			service.retrySendEventInterval = time.Second

			By("start probe service")
			startService(service)

			By("wait for probe to send event error")
			Eventually(count.Load, 2*service.retrySendEventInterval).Should(BeNumerically(">", 1))

			By("check received event after recover")
			var receivedData struct{ reason, message string }
			Eventually(eventChan, 2*service.retrySendEventInterval).Should(Receive(&receivedData))
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
