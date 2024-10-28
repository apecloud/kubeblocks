/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("Available", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("status", func() {
		It("ok", func() {
		})

		It("has no event", func() {
		})

		It("more then one event", func() {
		})

		It("event expired", func() {
		})

		It("has no new event and keep", func() {
		})

		It("multiple replicas - ok", func() {
		})

		It("multiple replicas - group event", func() {
		})
	})

	Context("evaluate condition", func() {
		It("all succeed - ok", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				All: &appsv1.ComponentAvailableConditionX{
					ActionCriteria: appsv1.ActionCriteria{
						Succeed: pointer.Bool(true),
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("ok"),
				},
				{
					Code:   0,
					Output: []byte("ok"),
				},
				{
					Code:   0,
					Output: []byte("ok"),
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeTrue())
		})

		It("all succeed - fail", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				All: &appsv1.ComponentAvailableConditionX{
					ActionCriteria: appsv1.ActionCriteria{
						Succeed: pointer.Bool(true),
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("ok"),
				},
				{
					Code:   0,
					Output: []byte("ok"),
				},
				{
					Code:    -1,
					Message: "command not found",
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeFalse())
		})

		It("has leader - ok", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				Any: &appsv1.ComponentAvailableConditionX{
					ActionCriteria: appsv1.ActionCriteria{
						Succeed: pointer.Bool(true),
						Stdout: &appsv1.ActionOutputMatcher{
							EqualTo: pointer.String("leader"),
						},
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("leader"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:    -1,
					Message: "host is unreachable",
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeTrue())
		})

		It("has leader - fail", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				Any: &appsv1.ComponentAvailableConditionX{
					ActionCriteria: appsv1.ActionCriteria{
						Succeed: pointer.Bool(true),
						Stdout: &appsv1.ActionOutputMatcher{
							EqualTo: pointer.String("leader"),
						},
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:    -1,
					Message: "operation is timed-out",
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeFalse())
		})

		It("has leader, majority replicas have roles - ok", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				And: []appsv1.ComponentAvailableConditionX{
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							Majority: &appsv1.ComponentAvailableConditionX{
								ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
									Or: []appsv1.ComponentAvailableConditionX{
										{
											ActionCriteria: appsv1.ActionCriteria{
												Succeed: pointer.Bool(true),
												Stdout: &appsv1.ActionOutputMatcher{
													EqualTo: pointer.String("leader"),
												},
											},
										},
										{
											ActionCriteria: appsv1.ActionCriteria{
												Succeed: pointer.Bool(true),
												Stdout: &appsv1.ActionOutputMatcher{
													EqualTo: pointer.String("follower"),
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							Any: &appsv1.ComponentAvailableConditionX{
								ActionCriteria: appsv1.ActionCriteria{
									Succeed: pointer.Bool(true),
									Stdout: &appsv1.ActionOutputMatcher{
										EqualTo: pointer.String("leader"),
									},
								},
							},
						},
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("leader"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeTrue())
		})

		It("has leader, majority replicas have roles - fail", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				And: []appsv1.ComponentAvailableConditionX{
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							Majority: &appsv1.ComponentAvailableConditionX{
								ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
									Or: []appsv1.ComponentAvailableConditionX{
										{
											ActionCriteria: appsv1.ActionCriteria{
												Succeed: pointer.Bool(true),
												Stdout: &appsv1.ActionOutputMatcher{
													EqualTo: pointer.String("leader"),
												},
											},
										},
										{
											ActionCriteria: appsv1.ActionCriteria{
												Succeed: pointer.Bool(true),
												Stdout: &appsv1.ActionOutputMatcher{
													EqualTo: pointer.String("follower"),
												},
											},
										},
									},
								},
							},
						},
					},
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							Any: &appsv1.ComponentAvailableConditionX{
								ActionCriteria: appsv1.ActionCriteria{
									Succeed: pointer.Bool(true),
									Stdout: &appsv1.ActionOutputMatcher{
										EqualTo: pointer.String("leader"),
									},
								},
							},
						},
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:   0,
					Output: []byte("learner"),
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeFalse())
		})

		It("has leader, has no FATAL errors", func() {
			h := &AvailableEventHandler{}

			cond := appsv1.ComponentAvailableCondition{
				And: []appsv1.ComponentAvailableConditionX{
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							Any: &appsv1.ComponentAvailableConditionX{
								ActionCriteria: appsv1.ActionCriteria{
									Succeed: pointer.Bool(true),
									Stdout: &appsv1.ActionOutputMatcher{
										EqualTo: pointer.String("leader"),
									},
								},
							},
						},
					},
					{
						ComponentAvailableCondition: appsv1.ComponentAvailableCondition{
							None: &appsv1.ComponentAvailableConditionX{
								ActionCriteria: appsv1.ActionCriteria{
									Stderr: &appsv1.ActionOutputMatcher{
										Contains: pointer.String("FATAL"),
									},
								},
							},
						},
					},
				},
			}
			events := []proto.ProbeEvent{
				{
					Code:   0,
					Output: []byte("leader"),
				},
				{
					Code:   0,
					Output: []byte("follower"),
				},
				{
					Code:    -1,
					Message: "[xxxx] FATAL: detected data is conrputed at offset 0x1234",
				},
			}
			available, _, err := h.evaluateCondition(cond, 1, events)
			Expect(err).Should(Succeed())
			Expect(available).Should(BeFalse())
		})
	})
})
