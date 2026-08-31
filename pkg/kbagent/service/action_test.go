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
	"os"
	"path/filepath"
	"sync"
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

		It("keeps the original terminal error in memory", func() {
			original := errors.Wrap(proto.ErrFailed, "action failed")
			result := newActionResult(nil, original)

			output, err := result.response()
			Expect(output).Should(BeNil())
			Expect(err).Should(Equal(original))
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
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

		It("returns in-progress and terminal results for a non-blocking Action", func() {
			svc, err := newActionService(logr.New(nil), []proto.Action{{
				Name:        "async",
				NonBlocking: true,
				Exec:        &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "sleep 0.05; echo -n done"}},
			}})
			Expect(err).Should(BeNil())

			req := &proto.ActionRequest{Action: "async"}
			out, err := svc.handleRequest(ctx, req)
			Expect(out).Should(BeNil())
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())

			Eventually(func() error {
				out, err = svc.handleRequest(ctx, req)
				return err
			}, 2*time.Second, 10*time.Millisecond).Should(Succeed())
			Expect(err).Should(BeNil())
			Expect(string(out)).Should(Equal("done"))

			out, err = svc.handleRequest(ctx, req)
			Expect(err).Should(BeNil())
			Expect(string(out)).Should(Equal("done"))
			Expect(svc.calls).Should(HaveKey("async"))
		})

		It("continues a non-blocking Action after the request is canceled", func() {
			svc, err := newActionService(logr.Discard(), []proto.Action{{
				Name:        "async",
				NonBlocking: true,
				Exec:        &proto.ExecAction{Commands: []string{"/bin/bash", "-c", "sleep 0.05; echo -n done"}},
			}})
			Expect(err).ShouldNot(HaveOccurred())

			requestCtx, cancel := context.WithCancel(ctx)
			_, err = svc.handleRequest(requestCtx, &proto.ActionRequest{Action: "async"})
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())
			cancel()

			Eventually(func() string {
				output, callErr := svc.handleRequest(ctx, &proto.ActionRequest{Action: "async"})
				if callErr != nil {
					return callErr.Error()
				}
				return string(output)
			}, 2*time.Second, 10*time.Millisecond).Should(Equal("done"))
		})

		It("rejects runtime arguments for non-exec actions in blocking and non-blocking calls", func() {
			action := &proto.Action{HTTP: &proto.HTTPAction{Port: "80"}}
			_, err := callActionWithRetry(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
			Expect(errors.Is(err, proto.ErrBadRequest)).Should(BeTrue())

			_, err = startNonBlockingActionCall(ctx, action, nil, [][]string{{"arg"}}, nil, nil)
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

		It("does not cap non-blocking Action timeouts", func() {
			timeout := int32(180)
			timedCtx, cancel := actionCallTimeoutContextWithCap(context.Background(), &timeout, false)
			defer cancel()

			deadline, ok := timedCtx.Deadline()
			Expect(ok).Should(BeTrue())
			remaining := time.Until(deadline)
			Expect(remaining).Should(BeNumerically(">", 179*time.Second))
			Expect(remaining).Should(BeNumerically("<=", 180*time.Second))
		})

		It("applies one timeout to the complete non-blocking argument sequence", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", `sleep "$0"; printf x`},
				},
			}
			timeout := int32(1)
			startedAt := time.Now()
			resultChan, err := startNonBlockingActionCall(
				ctx, action, nil, [][]string{{"0.25"}, {"1"}}, &timeout, nil)
			Expect(err).ShouldNot(HaveOccurred())

			result := <-resultChan
			Expect(errors.Is(result.err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(time.Since(startedAt)).Should(BeNumerically("<", 1500*time.Millisecond))
			Expect(result.stdout.String()).Should(Equal("x"))
		})

		It("counts retry intervals against the non-blocking Action timeout", func() {
			dir, err := os.MkdirTemp("", "kbagent-action-total-timeout-*")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)
			counterPath := filepath.Join(dir, "counter")
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{
						"/bin/bash", "-c",
						`n=0; [ -f "$0" ] && n=$(cat "$0"); n=$((n+1)); echo "$n" > "$0"; exit 1`,
						counterPath,
					},
				},
			}
			timeout := int32(1)
			retryPolicy := &proto.RetryPolicy{MaxRetries: 1, RetryInterval: 2 * time.Second}
			startedAt := time.Now()
			resultChan, err := startNonBlockingActionCall(
				ctx, action, nil, nil, &timeout, retryPolicy)
			Expect(err).ShouldNot(HaveOccurred())

			result := <-resultChan
			Expect(errors.Is(result.err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(time.Since(startedAt)).Should(BeNumerically("<", 1500*time.Millisecond))
			counter, err := os.ReadFile(counterPath)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(counter)).Should(Equal("1\n"))
		})

		It("stops zero-interval retries after the context is canceled", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/false"},
				},
			}
			retryPolicy := &proto.RetryPolicy{MaxRetries: 3}
			canceledCtx, cancel := context.WithCancel(ctx)
			cancel()

			_, err := callActionWithRetry(canceledCtx, action, nil, nil, nil, retryPolicy)
			Expect(errors.Is(err, context.Canceled)).Should(BeTrue())
		})

		It("normalizes equivalent requests when calculating their fingerprint", func() {
			timeout := int32(0)
			first := &proto.ActionRequest{
				Action:     "shardAdd",
				Parameters: map[string]string{"second": "2", "first": "1"},
			}
			second := &proto.ActionRequest{
				Action:     "shardAdd",
				Parameters: map[string]string{"first": "1", "second": "2"},
				Arguments:  [][]string{},
			}

			firstFingerprint, err := fingerprintActionRequest(first, &timeout, nil)
			Expect(err).ShouldNot(HaveOccurred())
			secondFingerprint, err := fingerprintActionRequest(second, &timeout, &proto.RetryPolicy{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(firstFingerprint).Should(Equal(secondFingerprint))

			second.TimeoutSeconds = func() *int32 {
				value := int32(30)
				return &value
			}()
			explicitDefaultFingerprint, err := fingerprintActionRequest(second, second.TimeoutSeconds, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(explicitDefaultFingerprint).Should(Equal(firstFingerprint))

			*second.TimeoutSeconds = 31
			differentFingerprint, err := fingerprintActionRequest(second, second.TimeoutSeconds, nil)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(differentFingerprint).ShouldNot(Equal(firstFingerprint))
		})

		It("serializes a single running request and honors rerun after completion", func() {
			dir, err := os.MkdirTemp("", "kbagent-action-rerun-*")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)
			counterPath := filepath.Join(dir, "counter")
			action := proto.Action{
				Name:        "async",
				NonBlocking: true,
				Exec: &proto.ExecAction{Commands: []string{
					"/bin/bash", "-c",
					`n=0; [ -f "$0" ] && n=$(cat "$0"); n=$((n+1)); echo "$n" > "$0"; sleep 0.1; printf "$n"`,
					counterPath,
				}},
			}
			svc, err := newActionService(logr.Discard(), []proto.Action{action})
			Expect(err).ShouldNot(HaveOccurred())
			req := &proto.ActionRequest{Action: "async"}

			_, err = svc.handleRequest(ctx, req)
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())
			_, err = svc.handleRequest(ctx, req)
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())
			_, err = svc.handleRequest(ctx, &proto.ActionRequest{Action: "async", Rerun: true})
			Expect(errors.Is(err, proto.ErrBusy)).Should(BeTrue())
			_, err = svc.handleRequest(ctx, &proto.ActionRequest{
				Action: "async", Parameters: map[string]string{"different": "request"},
			})
			Expect(errors.Is(err, proto.ErrBusy)).Should(BeTrue())

			Eventually(func() string {
				output, callErr := svc.handleRequest(ctx, req)
				if callErr != nil {
					return callErr.Error()
				}
				return string(output)
			}, 2*time.Second, 10*time.Millisecond).Should(Equal("1"))

			output, err := svc.handleRequest(ctx, req)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(output)).Should(Equal("1"))

			_, err = svc.handleRequest(ctx, &proto.ActionRequest{Action: "async", Rerun: true})
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())
			Eventually(func() string {
				output, callErr := svc.handleRequest(ctx, req)
				if callErr != nil {
					return callErr.Error()
				}
				return string(output)
			}, 2*time.Second, 10*time.Millisecond).Should(Equal("2"))

			differentReq := &proto.ActionRequest{
				Action:     "async",
				Parameters: map[string]string{"different": "request"},
			}
			_, err = svc.handleRequest(ctx, differentReq)
			Expect(errors.Is(err, proto.ErrInProgress)).Should(BeTrue())
			Eventually(func() string {
				output, callErr := svc.handleRequest(ctx, differentReq)
				if callErr != nil {
					return callErr.Error()
				}
				return string(output)
			}, 2*time.Second, 10*time.Millisecond).Should(Equal("3"))
		})

		It("starts only one process for concurrent equivalent requests", func() {
			dir, err := os.MkdirTemp("", "kbagent-action-concurrent-*")
			Expect(err).ShouldNot(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)
			counterPath := filepath.Join(dir, "counter")
			svc, err := newActionService(logr.Discard(), []proto.Action{{
				Name:        "async",
				NonBlocking: true,
				Exec: &proto.ExecAction{Commands: []string{
					"/bin/bash", "-c", `echo start >> "$0"; sleep 0.2; printf done`, counterPath,
				}},
			}})
			Expect(err).ShouldNot(HaveOccurred())

			const callers = 16
			errs := make(chan error, callers)
			var wg sync.WaitGroup
			for range callers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, callErr := svc.handleRequest(ctx, &proto.ActionRequest{Action: "async"})
					errs <- callErr
				}()
			}
			wg.Wait()
			close(errs)
			for callErr := range errs {
				Expect(errors.Is(callErr, proto.ErrInProgress)).Should(BeTrue())
			}

			Eventually(func() string {
				output, callErr := svc.handleRequest(ctx, &proto.ActionRequest{Action: "async"})
				if callErr != nil {
					return callErr.Error()
				}
				return string(output)
			}, 2*time.Second, 10*time.Millisecond).Should(Equal("done"))
			counter, err := os.ReadFile(counterPath)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(counter)).Should(Equal("start\n"))
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
			svc.actions["retry"].NonBlocking = true

			req := &proto.ActionRequest{Action: "retry"}
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
