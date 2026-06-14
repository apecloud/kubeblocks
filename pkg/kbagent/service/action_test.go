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
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("action", func() {
	Context("action", func() {
		It("caps requested action timeout at 120 seconds", func() {
			timeout := int32(180)
			timedCtx, cancel := actionCallTimeoutContext(context.Background(), &timeout)
			defer cancel()

			deadline, ok := timedCtx.Deadline()
			Expect(ok).Should(BeTrue())
			remaining := time.Until(deadline)
			Expect(remaining).Should(BeNumerically(">", 119*time.Second))
			Expect(remaining).Should(BeNumerically("<=", 120*time.Second))
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
