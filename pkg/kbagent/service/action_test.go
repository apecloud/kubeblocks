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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("action", func() {
	Context("action", func() {
		It("exposes service metadata and no-op methods", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())
			Expect(svc.Kind()).Should(Equal(proto.ServiceAction.Kind))
			Expect(svc.URI()).Should(Equal(proto.ServiceAction.URI))
			Expect(svc.Start()).Should(Succeed())
			Expect(svc.HandleConn(ctx, nil)).Should(Succeed())
		})

		It("decodes and encodes action responses", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())

			req, err := svc.decode([]byte(`{"action":"backup"}`))
			Expect(err).Should(BeNil())
			Expect(req.Action).Should(Equal("backup"))

			_, err = svc.decode([]byte("{"))
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			data := svc.encode([]byte("ok"), nil)
			resp := &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Output).Should(Equal([]byte("ok")))

			data = svc.encode(nil, proto.ErrNotDefined)
			resp = &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("notDefined"))
			Expect(resp.Message).Should(ContainSubstring(proto.ErrNotDefined.Error()))
		})

		It("handles request errors through encoded responses", func() {
			svc, err := newActionService(logr.New(nil), nil)
			Expect(err).Should(BeNil())

			data, err := svc.HandleRequest(ctx, []byte("{"))
			Expect(err).Should(BeNil())
			resp := &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("badRequest"))

			data, err = svc.HandleRequest(ctx, []byte(`{"action":"missing"}`))
			Expect(err).Should(BeNil())
			resp = &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("notDefined"))
		})

		It("returns a declared semantic code while preserving the legacy error", func() {
			svc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "reconfigure",
				Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", "exit 42"}},
				ResultPolicy: &proto.ActionResultPolicy{FailureCodes: []proto.ActionFailureCode{{
					Code:         "InvalidParameter",
					ExecExitCode: 42,
					Retry:        false,
				}}},
			}})
			Expect(err).Should(BeNil())

			data, err := svc.HandleRequest(ctx, []byte(`{"action":"reconfigure"}`))
			Expect(err).Should(BeNil())
			resp := &proto.ActionResponse{}
			Expect(json.Unmarshal(data, resp)).Should(Succeed())
			Expect(resp.Error).Should(Equal("failed"))
			Expect(resp.Code).Should(Equal("InvalidParameter"))
			Expect(resp.Retryable).ShouldNot(BeNil())
			Expect(*resp.Retryable).Should(BeFalse())
		})

		It("rejects ambiguous or malformed result policies before serving actions", func() {
			newAction := func(failureCodes ...proto.ActionFailureCode) proto.Action {
				return proto.Action{
					Name:         "reconfigure",
					Exec:         &proto.ExecAction{Commands: []string{"true"}},
					ResultPolicy: &proto.ActionResultPolicy{FailureCodes: failureCodes},
				}
			}

			_, err := newActionService(logr.Discard(), []proto.Action{newAction(
				proto.ActionFailureCode{Code: "InvalidParameter", ExecExitCode: 2},
				proto.ActionFailureCode{Code: "UnsupportedParameter", ExecExitCode: 2},
			)})
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("mapped to both"))

			for _, action := range []proto.Action{
				newAction(proto.ActionFailureCode{Code: "invalid-parameter", ExecExitCode: 2}),
				newAction(proto.ActionFailureCode{Code: "InvalidParameter"}),
				newAction(proto.ActionFailureCode{Code: "InvalidParameter", ExecExitCode: 256}),
			} {
				_, err = newActionService(logr.Discard(), []proto.Action{action})
				Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
			}

			tooManyMappings := make([]proto.ActionFailureCode, maxActionFailureCodes+1)
			for i := range tooManyMappings {
				tooManyMappings[i] = proto.ActionFailureCode{
					Code:         fmt.Sprintf("Failure%d", i),
					ExecExitCode: int32(i + 1),
				}
			}
			_, err = newActionService(logr.Discard(), []proto.Action{newAction(tooManyMappings...)})
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			_, err = newActionService(logr.Discard(), []proto.Action{newAction(
				proto.ActionFailureCode{Code: "InvalidParameter", ExecExitCode: 2},
				proto.ActionFailureCode{Code: "InvalidParameter", ExecExitCode: 64},
			)})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = newActionService(logr.Discard(), []proto.Action{{
				Name:         "http-action",
				HTTP:         &proto.HTTPAction{Port: "8080"},
				ResultPolicy: &proto.ActionResultPolicy{},
			}})
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("only supported for exec actions"))
		})

		It("handles non-blocking in-progress and completion states", func() {
			svc, err := newActionService(logr.New(nil), []proto.Action{{
				Name: "async",
				Exec: &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "echo -n unused"}},
			}})
			Expect(err).Should(BeNil())

			resultChan := make(chan *asyncResult, 1)
			svc.runningActions["async"] = &runningAction{resultChan: resultChan}
			req := &proto.ActionRequest{Action: "async"}

			out, err := svc.handleRequestNonBlocking(ctx, req, svc.actions["async"], nil, nil)
			Expect(out).Should(BeNil())
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())

			resultChan <- &asyncResult{stdout: bytes.NewBufferString("done"), stderr: bytes.NewBuffer(nil)}
			out, err = svc.handleRequestNonBlocking(ctx, req, svc.actions["async"], nil, nil)
			Expect(err).Should(BeNil())
			Expect(string(out)).Should(Equal("done"))
			Expect(svc.runningActions).ShouldNot(HaveKey("async"))
		})

		It("rejects runtime arguments for non-exec actions in blocking and non-blocking calls", func() {
			action := &proto.Action{HTTP: &proto.HTTPAction{Port: "80"}}
			_, err := callActionWithRetry(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			_, err = nonBlockingCallActionWithRetry(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())
		})

		It("resolves timeout preference", func() {
			actionTimeout := int32(10)
			requestTimeout := int32(1)
			Expect(resolveTimeout(&actionTimeout, &requestTimeout)).Should(Equal(&requestTimeout))
			Expect(resolveTimeout(&actionTimeout, nil)).Should(Equal(&actionTimeout))
		})

		It("caps requested action timeout at 60 seconds", func() {
			timeout := int32(180)
			timedCtx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
			defer cancel()

			deadline, ok := timedCtx.Deadline()
			Expect(ok).Should(BeTrue())
			remaining := time.Until(deadline)
			Expect(remaining).Should(BeNumerically(">", 59*time.Second))
			Expect(remaining).Should(BeNumerically("<=", 60*time.Second))
		})

		newRetryAction := func(name string, counterPath string, maxRetries int) proto.Action {
			return proto.Action{
				Name: name,
				Exec: &proto.ExecAction{
					Commands: []string{
						"/bin/bash", "-c",
						`n=0; [ -f "$0" ] && n=$(cat "$0"); n=$((n+1)); echo "$n" > "$0"; if [ "$n" -lt 2 ]; then echo "retryable failure" >&2; exit 1; fi; printf ok`,
						counterPath,
					},
				},
				RetryPolicy: &proto.RetryPolicy{MaxRetries: maxRetries},
			}
		}

		It("uses the action retry policy when request retry policy is absent", func() {
			f, err := os.CreateTemp("", "kbagent-action-retry-*")
			Expect(err).Should(BeNil())
			counterPath := f.Name()
			Expect(f.Close()).Should(Succeed())
			defer os.Remove(counterPath)

			svc, err := newActionService(logr.Discard(), []proto.Action{
				newRetryAction("retry", counterPath, 1),
			})
			Expect(err).Should(BeNil())

			output, err := svc.handleRequest(ctx, &proto.ActionRequest{Action: "retry"})
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("ok")))

			counter, err := os.ReadFile(counterPath)
			Expect(err).Should(BeNil())
			Expect(string(counter)).Should(Equal("2\n"))
		})

		It("does not retry a failure explicitly declared non-retryable", func() {
			f, err := os.CreateTemp("", "kbagent-action-no-retry-*")
			Expect(err).Should(BeNil())
			counterPath := f.Name()
			Expect(f.Close()).Should(Succeed())
			defer os.Remove(counterPath)

			action := newRetryAction("no-retry", counterPath, 3)
			action.ResultPolicy = &proto.ActionResultPolicy{FailureCodes: []proto.ActionFailureCode{{
				Code:         "InvalidParameter",
				ExecExitCode: 1,
				Retry:        false,
			}}}
			svc, err := newActionService(logr.Discard(), []proto.Action{action})
			Expect(err).Should(BeNil())

			_, err = svc.handleRequest(ctx, &proto.ActionRequest{Action: "no-retry"})
			Expect(err).ShouldNot(BeNil())
			Expect(proto.ActionResultCode(err)).Should(Equal("InvalidParameter"))
			counter, err := os.ReadFile(counterPath)
			Expect(err).Should(BeNil())
			Expect(string(counter)).Should(Equal("1\n"))
		})

		It("retries a failure explicitly declared retryable", func() {
			f, err := os.CreateTemp("", "kbagent-action-declared-retry-*")
			Expect(err).Should(BeNil())
			counterPath := f.Name()
			Expect(f.Close()).Should(Succeed())
			defer os.Remove(counterPath)

			action := newRetryAction("declared-retry", counterPath, 1)
			action.ResultPolicy = &proto.ActionResultPolicy{FailureCodes: []proto.ActionFailureCode{{
				Code:         "TransientFailure",
				ExecExitCode: 1,
				Retry:        true,
			}}}
			svc, err := newActionService(logr.Discard(), []proto.Action{action})
			Expect(err).Should(BeNil())

			out, err := svc.handleRequest(ctx, &proto.ActionRequest{Action: "declared-retry"})
			Expect(err).Should(BeNil())
			Expect(out).Should(Equal([]byte("ok")))
			counter, err := os.ReadFile(counterPath)
			Expect(err).Should(BeNil())
			Expect(string(counter)).Should(Equal("2\n"))
		})

		It("lets the request retry policy override the action retry policy", func() {
			f, err := os.CreateTemp("", "kbagent-request-retry-*")
			Expect(err).Should(BeNil())
			counterPath := f.Name()
			Expect(f.Close()).Should(Succeed())
			defer os.Remove(counterPath)

			svc, err := newActionService(logr.Discard(), []proto.Action{
				newRetryAction("retry", counterPath, 1),
			})
			Expect(err).Should(BeNil())

			_, err = svc.handleRequest(ctx, &proto.ActionRequest{
				Action:      "retry",
				RetryPolicy: &proto.RetryPolicy{MaxRetries: 0},
			})
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("retryable failure"))

			counter, err := os.ReadFile(counterPath)
			Expect(err).Should(BeNil())
			Expect(string(counter)).Should(Equal("1\n"))
		})

		It("applies retry policy to non-blocking calls without runtime arguments", func() {
			f, err := os.CreateTemp("", "kbagent-nonblocking-retry-*")
			Expect(err).Should(BeNil())
			counterPath := f.Name()
			Expect(f.Close()).Should(Succeed())
			defer os.Remove(counterPath)

			svc, err := newActionService(logr.Discard(), []proto.Action{
				newRetryAction("retry", counterPath, 1),
			})
			Expect(err).Should(BeNil())

			nonBlocking := true
			req := &proto.ActionRequest{Action: "retry", NonBlocking: &nonBlocking}
			Eventually(func() string {
				output, err := svc.handleRequest(ctx, req)
				if err != nil {
					return err.Error()
				}
				return string(output)
			}, 2*time.Second, 50*time.Millisecond).Should(Equal("ok"))

			counter, err := os.ReadFile(counterPath)
			Expect(err).Should(BeNil())
			Expect(string(counter)).Should(Equal("2\n"))
		})
	})
})
