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

package parameters

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reconfigure combinedPolicy test", func() {
	var (
		rctx reconfigureContext
	)

	BeforeEach(func() {
		rctx = newMockReconfigureParams("combinedPolicy", k8sClient,
			withConfigSpec("test", map[string]string{
				"key": "value",
			}),
			withClusterComponent(2))
	})

	AfterEach(func() {
	})

	Context("combine reconfigure policy", func() {
		It("dummy policy", func() {
			policies := &combinedPolicy{
				policies: []reconfigurePolicy{
					&dummyPolicy{
						status: ESNone,
						err:    nil,
					}},
			}
			status, err := policies.Upgrade(rctx)
			Expect(err).Should(BeNil())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})

		It("error policy", func() {
			policies := &combinedPolicy{
				policies: []reconfigurePolicy{
					&dummyPolicy{
						status: ESFailedAndRetry,
						err:    fmt.Errorf("error"),
					}},
			}
			status, err := policies.Upgrade(rctx)
			Expect(err).ShouldNot(BeNil())
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})

		It("ok + error policy", func() {
			policies := &combinedPolicy{
				policies: []reconfigurePolicy{
					&dummyPolicy{
						status: ESNone,
						err:    nil,
					},
					&dummyPolicy{
						status: ESFailedAndRetry,
						err:    fmt.Errorf("error"),
					}},
			}
			status, err := policies.Upgrade(rctx)
			Expect(err).ShouldNot(BeNil())
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})

		It("error + ok policy", func() {
			policies := &combinedPolicy{
				policies: []reconfigurePolicy{
					&dummyPolicy{
						status: ESFailedAndRetry,
						err:    fmt.Errorf("error"),
					},
					&dummyPolicy{
						status: ESNone,
						err:    nil,
					},
				},
			}
			status, err := policies.Upgrade(rctx)
			Expect(err).ShouldNot(BeNil())
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})

		It("retry + none policy", func() {
			policies := &combinedPolicy{
				policies: []reconfigurePolicy{
					&dummyPolicy{
						status: ESRetry,
						err:    nil,
					},
					&dummyPolicy{
						status: ESNone,
						err:    nil,
					},
				},
			}
			status, err := policies.Upgrade(rctx)
			Expect(err).Should(BeNil())
			Expect(status.Status).Should(BeEquivalentTo(ESNone)) // TODO: should be ESRetry
		})
	})
})

type dummyPolicy struct {
	status ExecStatus
	err    error
}

func (t *dummyPolicy) Upgrade(reconfigureContext) (returnedStatus, error) {
	return makeReturnedStatus(t.status), t.err
}
