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
	"errors"
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type fakeTask struct {
	statusCalled chan struct{}
}

func (t fakeTask) run(context.Context) (chan error, error) {
	ch := make(chan error, 1)
	ch <- nil
	return ch, nil
}

func (t fakeTask) status(_ context.Context, _ *proto.TaskEvent) {
	t.statusCalled <- struct{}{}
}

var _ = Describe("task", func() {
	Context("task", func() {
		It("runs no-op task lists through exported helper", func() {
			GinkgoT().Setenv("KB_AGENT_POD_NAME", "pod-0")
			actionSvc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())
			Expect(RunTasks(logr.New(nil), actionSvc, []proto.Task{{Replicas: "pod-1"}})).Should(Succeed())
		})

		It("handles wait channel states", func() {
			svc := &taskService{}
			Expect(svc.wait(nil)).Should(Succeed())

			ch := make(chan error, 1)
			ch <- errors.New("failed")
			Expect(svc.wait(ch)).Should(MatchError("failed"))

			closed := make(chan error)
			close(closed)
			Expect(svc.wait(closed)).Should(MatchError(ContainSubstring("error chan closed unexpectedly")))
		})

		It("creates new replica tasks and returns stable validation errors", func() {
			GinkgoT().Setenv("KB_AGENT_POD_NAME", "pod-0")
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: newReplicaDataLoad,
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "cat"}},
			}})
			Expect(err).Should(BeNil())
			svc := &taskService{logger: logr.New(nil), actionService: actionSvc}

			Expect(svc.newTask(proto.Task{})).Should(BeNil())
			Expect(svc.newTask(proto.Task{NewReplica: &proto.NewReplicaTask{}})).ShouldNot(BeNil())

			err = svc.runTask(ctx, proto.Task{
				Instance: "inst",
				Task:     "new-replica",
				UID:      "u1",
				NewReplica: &proto.NewReplicaTask{
					Port: 3502,
				},
			})
			Expect(err).Should(MatchError("remote server is required"))
		})

		It("runs matching replica tasks and propagates task errors", func() {
			GinkgoT().Setenv("KB_AGENT_POD_NAME", "pod-0")
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: newReplicaDataLoad,
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "cat"}},
			}})
			Expect(err).Should(BeNil())
			svc := &taskService{
				logger:        logr.New(nil),
				actionService: actionSvc,
				tasks: []proto.Task{{
					Instance:   "inst",
					Task:       "new-replica",
					UID:        "u1",
					Replicas:   "pod-0,pod-1",
					NewReplica: &proto.NewReplicaTask{Port: 3502},
				}},
			}

			Expect(svc.runTasks(ctx)).Should(MatchError("remote server is required"))
		})

		It("reports task status until stopped", func() {
			svc := &taskService{logger: logr.New(nil)}
			fake := fakeTask{statusCalled: make(chan struct{}, 1)}
			exit, exited := svc.report(ctx, proto.Task{ReportPeriodSeconds: 1}, fake, proto.TaskEvent{})
			Expect(exit).ShouldNot(BeNil())
			Expect(exited).ShouldNot(BeNil())

			select {
			case <-fake.statusCalled:
			case <-time.After(2 * time.Second):
				Fail("timed out waiting for status report")
			}
			close(exit)
			Eventually(exited).Should(BeClosed())

			exit, exited = svc.report(ctx, proto.Task{}, fake, proto.TaskEvent{})
			Expect(exit).Should(BeNil())
			Expect(exited).Should(BeNil())
		})

		It("keeps the run error when finish notification also fails", func() {
			GinkgoT().Setenv("KB_AGENT_POD_NAME", "pod-0")
			GinkgoT().Setenv("KUBECONFIG", "/path/to/missing/kubeconfig")
			actionSvc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: newReplicaDataLoad,
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "cat"}},
			}})
			Expect(err).Should(BeNil())
			svc := &taskService{logger: logr.New(nil), actionService: actionSvc}

			err = svc.runTask(ctx, proto.Task{
				Instance:       "inst",
				Task:           "new-replica",
				UID:            "u1",
				NotifyAtFinish: true,
				NewReplica:     &proto.NewReplicaTask{Port: 3502},
			})
			Expect(err).Should(MatchError("remote server is required"))
		})

		It("writes a new-replica handshake packet to the remote server", func() {
			GinkgoT().Setenv("KB_AGENT_POD_NAME", "pod-0")
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).Should(BeNil())
			defer listener.Close()

			accepted := make(chan proto.ActionRequest, 1)
			go func() {
				defer GinkgoRecover()
				conn, err := listener.Accept()
				Expect(err).Should(BeNil())
				defer conn.Close()
				req := proto.ActionRequest{}
				Expect(json.NewDecoder(conn).Decode(&req)).Should(Succeed())
				accepted <- req
			}()

			_, port, err := net.SplitHostPort(listener.Addr().String())
			Expect(err).Should(BeNil())
			var portNumber int
			_, err = fmt.Sscanf(port, "%d", &portNumber)
			Expect(err).Should(BeNil())

			task := &newReplicaTask{
				task: &proto.NewReplicaTask{
					Remote:     "127.0.0.1",
					Port:       int32(portNumber),
					Parameters: map[string]string{"foo": "bar"},
				},
			}
			conn, err := task.handshake(ctx)
			Expect(err).Should(BeNil())
			Expect(conn.Close()).Should(Succeed())

			req := <-accepted
			Expect(req.Action).Should(Equal(newReplicaDataDump))
			Expect(req.Parameters).Should(HaveKeyWithValue("foo", "bar"))
			Expect(req.Parameters).Should(HaveKeyWithValue(targetPodNameEnv, "pod-0"))

			event := &proto.TaskEvent{Code: -1, Message: "old", Output: []byte("old")}
			task.status(ctx, event)
			Expect(event.Code).Should(BeZero())
			Expect(event.Message).Should(BeEmpty())
			Expect(event.Output).Should(BeNil())
		})

		It("validates remote connection settings", func() {
			task := &newReplicaTask{task: &proto.NewReplicaTask{Remote: "127.0.0.1"}}
			conn, err := task.connectToRemote(ctx)
			Expect(conn).Should(BeNil())
			Expect(err).Should(MatchError("remote port is required"))
		})
	})
})
