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
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

	Context("handle event", func() {
		var (
			compDef             *appsv1.ComponentDefinition
			comp                *appsv1.Component
			availableTimeWindow = int32(10)
		)

		BeforeEach(func() {
			compDef = &appsv1.ComponentDefinition{
				Spec: appsv1.ComponentDefinitionSpec{
					Available: &appsv1.ComponentAvailable{
						WithProbe: &appsv1.ComponentAvailableWithProbe{
							TimeWindowSeconds: &availableTimeWindow,
							// has the leader, and majority replicas have roles
							Condition: &appsv1.ComponentAvailableCondition{
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
							},
						},
					},
					LifecycleActions: &appsv1.ComponentLifecycleActions{
						AvailableProbe: &appsv1.Probe{
							Action: appsv1.Action{
								Exec: &appsv1.ExecAction{
									Command: []string{"echo", "available"},
								},
							},
						},
					},
				},
			}
			comp = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-comp",
				},
				Spec: appsv1.ComponentSpec{
					Replicas: 3,
				},
				Status: appsv1.ComponentStatus{},
			}
		})

		reqCtx := func() intctrlutil.RequestCtx {
			return intctrlutil.RequestCtx{
				Ctx:      ctx,
				Log:      logger,
				Recorder: recorder,
			}
		}()

		annotationWithProbeEvents := func(events []probeEvent) {
			message, ok := json.Marshal(&events)
			Expect(ok).Should(BeNil())
			comp.Annotations = map[string]string{
				availableProbeEventKey: string(message),
			}
		}

		It("not probe event", func() {
			h := &AvailableEventHandler{}

			event := &corev1.Event{
				InvolvedObject: corev1.ObjectReference{
					FieldPath: proto.ProbeEventFieldPath,
				},
				Reason:              "roleProbe",
				ReportingController: proto.ProbeEventReportingController,
			}
			err := h.Handle(k8sClient, reqCtx, reqCtx.Recorder, event)
			Expect(err).Should(Succeed())
		})

		It("ok", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-1",
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte(""), // has no role
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())
		})

		It("has no event", func() {
			h := &AvailableEventHandler{}

			event := probeEvent{}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeFalse())
		})

		It("duplicate events", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-1",
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
				{
					PodName:   "test-cluster-comp-2",
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"), // duplicate event
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())
		})

		It("event expired", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-15 * time.Second), // expired
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-1",
					Timestamp: time.Now().Add(-15 * -time.Second), // expired
					Code:      0,
					Stdout:    []byte("follower"),
				},
				{
					PodName:   "test-cluster-comp-2",
					Timestamp: time.Now().Add(-15 * -time.Second), // expired
					Code:      0,
					Stdout:    []byte("follower"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeFalse())
		})

		It("has new event and keep", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-1",
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
				{
					PodName:   "test-cluster-comp-2",
					Timestamp: time.Now().Add(-15 * -time.Second), // expired
					Code:      0,
					Stdout:    []byte("follower"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"), // new event
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())
		})

		It("probe event in annotation", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
			})
			Expect(comp.Annotations).ShouldNot(BeNil())
			Expect(comp.Annotations[availableProbeEventKey]).ShouldNot(BeEmpty())
			message := comp.Annotations[availableProbeEventKey]

			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-3 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())

			Expect(comp.Annotations[availableProbeEventKey]).ShouldNot(Equal(message))
			events := make([]probeEvent, 0)
			Expect(json.Unmarshal([]byte(comp.Annotations[availableProbeEventKey]), &events)).Should(Succeed())
			Expect(events).Should(HaveLen(2))
			Expect(events[0].PodName).Should(Or(Equal("test-cluster-comp-0"), Equal("test-cluster-comp-2")))
			Expect(events[1].PodName).Should(Or(Equal("test-cluster-comp-0"), Equal("test-cluster-comp-2")))
		})

		It("strict all", func() {
			h := &AvailableEventHandler{}

			// all has leader or follower
			compDef.Spec.Available.WithProbe.Condition = &appsv1.ComponentAvailableCondition{
				All: &appsv1.ComponentAvailableConditionX{
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
					Strict: pointer.Bool(true),
				},
			}
			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-1",
				Timestamp: time.Now().Add(-3 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeFalse())

			// new event for last replica
			event = probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err = h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())
		})

		It("deleted replicas - available", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-1",
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
				{
					PodName:   "test-cluster-comp-3", // replica 3 is deleted
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte("follower"),
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeTrue())
		})

		It("deleted replicas - unavailable", func() {
			h := &AvailableEventHandler{}

			annotationWithProbeEvents([]probeEvent{
				{
					PodName:   "test-cluster-comp-0",
					Timestamp: time.Now().Add(-5 * time.Second),
					Code:      0,
					Stdout:    []byte("leader"),
				},
				{
					PodName:   "test-cluster-comp-3", // replica 3 is deleted
					Timestamp: time.Now().Add(-5 * -time.Second),
					Code:      0,
					Stdout:    []byte("follower"),
				},
			})
			event := probeEvent{
				PodName:   "test-cluster-comp-2",
				Timestamp: time.Now().Add(-1 * -time.Second),
				Code:      0,
				Stdout:    []byte(""), // has no role
			}
			available, _, err := h.handleEvent(event, comp, compDef)
			Expect(err).Should(Succeed())
			Expect(*available).Should(BeFalse())
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("ok"),
				},
				{
					Code:   0,
					Stdout: []byte("ok"),
				},
				{
					Code:   0,
					Stdout: []byte("ok"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("ok"),
				},
				{
					Code:   0,
					Stdout: []byte("ok"),
				},
				{
					Code:   -1,
					Stderr: []byte("command not found"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("leader"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   -1,
					Stderr: []byte("host is unreachable"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   -1,
					Stderr: []byte("operation is timed-out"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("leader"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   0,
					Stdout: []byte("learner"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
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
			events := []probeEvent{
				{
					Code:   0,
					Stdout: []byte("leader"),
				},
				{
					Code:   0,
					Stdout: []byte("follower"),
				},
				{
					Code:   -1,
					Stderr: []byte("[xxxx] FATAL: detected data is conrputed at offset 0x1234"),
				},
			}
			available, _ := h.evaluateCondition(cond, 1, events)
			Expect(available).Should(BeFalse())
		})
	})
})
