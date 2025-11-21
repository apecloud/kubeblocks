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

package lifecycle

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("sharding lifecycle", func() {
	var (
		namespace       string
		clusterName     string
		compName        string
		shardingName    string
		shardingActions *appsv1.ShardingLifecycleActions
		pods            []*corev1.Pod
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
	}

	BeforeEach(func() {
		cleanEnv()

		namespace = "default"
		clusterName = "test-cluster"
		compName = "kbagent"
		shardingName = "test-sharding"
		shardingActions = &appsv1.ShardingLifecycleActions{
			PostProvision: &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"/bin/bash", "-c", "echo -n shard-post-provision"},
				},
				TimeoutSeconds: 5,
				RetryPolicy: &appsv1.RetryPolicy{
					MaxRetries:    5,
					RetryInterval: 10,
				},
			},
			PreTerminate: &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"/bin/bash", "-c", "echo -n shard-pre-terminate"},
				},
				TimeoutSeconds: 10,
				RetryPolicy: &appsv1.RetryPolicy{
					MaxRetries:    3,
					RetryInterval: 5,
				},
			},
			ShardAdd: &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"/bin/bash", "-c", "echo -n shard-add"},
				},
				TimeoutSeconds: 15,
				RetryPolicy: &appsv1.RetryPolicy{
					MaxRetries:    3,
					RetryInterval: 15,
				},
			},
			ShardRemove: &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"/bin/bash", "-c", "echo -n shard-remove"},
				},
				TimeoutSeconds: 20,
				RetryPolicy: &appsv1.RetryPolicy{
					MaxRetries:    2,
					RetryInterval: 20,
				},
			},
		}
		pods = []*corev1.Pod{{}}
	})

	AfterEach(func() {
		cleanEnv()

		kbacli.UnsetMockClient()
	})

	Context("new sharding lifecycle", func() {
		It("nil pod", func() {
			_, err := NewShardingLifecycle("", "", "", "", nil, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("either pod or pods must be provided to call lifecycle actions"))
		})

		It("pod", func() {
			pod := pods[0]
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, pod)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			agent := shardingLifecycle.(*shardingAgent)
			Expect(agent.namespace).Should(Equal(namespace))
			Expect(agent.shardingLifecycleActions).Should(Equal(shardingActions))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
			Expect(agent.clusterName).Should(Equal(clusterName))
			Expect(agent.compName).Should(Equal(compName))
			Expect(agent.shardingName).Should(Equal(shardingName))
		})

		It("pods", func() {
			pod := pods[0]
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			agent := shardingLifecycle.(*shardingAgent)
			Expect(agent.namespace).Should(Equal(namespace))
			Expect(agent.clusterName).Should(Equal(clusterName))
			Expect(agent.compName).Should(Equal(compName))
			Expect(agent.shardingName).Should(Equal(shardingName))
			Expect(agent.shardingLifecycleActions).Should(Equal(shardingActions))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
		})

		It("template vars", func() {
			templateVars := map[string]string{
				"SHARD_NAME":   shardingName,
				"CLUSTER_NAME": clusterName,
			}
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, templateVars, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			agent := shardingLifecycle.(*shardingAgent)
			Expect(agent.templateVars).Should(Equal(templateVars))
		})
	})

	Context("call sharding actions", func() {
		It("ShardRemove - not defined", func() {
			shardingActions.ShardRemove = nil
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			err = shardingLifecycle.ShardRemove(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionNotDefined)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("shardRemove"))
		})

		It("PostProvision - action request", func() {
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			action := shardingActions.PostProvision
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("shardPostProvision"))
					Expect(req.Parameters).Should(BeEmpty())
					Expect(req.NonBlocking).ShouldNot(BeNil())
					Expect(*req.NonBlocking).Should(BeTrue())
					Expect(req.TimeoutSeconds).ShouldNot(BeNil())
					Expect(*req.TimeoutSeconds).Should(Equal(action.TimeoutSeconds))
					Expect(req.RetryPolicy).ShouldNot(BeNil())
					Expect(req.RetryPolicy.MaxRetries).Should(Equal(action.RetryPolicy.MaxRetries))
					Expect(req.RetryPolicy.RetryInterval).Should(Equal(action.RetryPolicy.RetryInterval))
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			opts := &Options{
				NonBlocking:    &[]bool{true}[0],
				TimeoutSeconds: &action.TimeoutSeconds,
				RetryPolicy:    action.RetryPolicy,
			}
			err = shardingLifecycle.PostProvision(ctx, k8sClient, opts)
			Expect(err).Should(BeNil())
		})

		It("PreTerminate - succeed", func() {
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Output: []byte("pre-terminate-success"),
					}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.PreTerminate(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("fail - error code", func() {
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			unknownErr := fmt.Errorf("%s", "unknown error")
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrFailed),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, unknownErr
				}).MaxTimes(1)
			})

			// Test PostProvision
			err = shardingLifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())

			// Test ShardAdd with unknown error
			err = shardingLifecycle.ShardAdd(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, unknownErr)).Should(BeTrue())
		})

		It("fail - error msg", func() {
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error:   proto.Error2Type(proto.ErrFailed),
						Message: "shard command not found",
					}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.ShardRemove(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("shard command not found"))
		})

		It("template vars", func() {
			key := "SHARD_TEMPLATE_VAR"
			val := "shard-template-value"

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, map[string]string{key: val}, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("shardPostProvision"))
					Expect(req.Parameters).ShouldNot(BeNil())
					Expect(req.Parameters[key]).Should(Equal(val))
					return proto.ActionResponse{
						Output: []byte(req.Parameters[key]),
					}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - ComponentReady", func() {
			componentReady := appsv1.ComponentReadyPreConditionType
			shardingActions.PostProvision.PreCondition = &componentReady

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      constant.GenerateClusterComponentName(clusterName, compName),
							Labels: map[string]string{
								constant.AppInstanceLabelKey:       clusterName,
								constant.KBAppShardingNameLabelKey: shardingName,
								constant.AppManagedByLabelKey:      "kubeblocks",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningComponentPhase,
						},
					},
				},
			}

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.PostProvision(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - ComponentReady fail", func() {
			componentReady := appsv1.ComponentReadyPreConditionType
			shardingActions.PostProvision.PreCondition = &componentReady

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      constant.GenerateClusterComponentName(clusterName, compName),
							Labels: map[string]string{
								constant.AppInstanceLabelKey:       clusterName,
								constant.KBAppShardingNameLabelKey: shardingName,
								constant.AppManagedByLabelKey:      "kubeblocks",
							},
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.FailedComponentPhase,
						},
					},
				},
			}

			err = shardingLifecycle.PostProvision(ctx, reader, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("component is not ready"))
		})

		It("precondition - Immediately", func() {
			immediately := appsv1.ImmediatelyPreConditionType
			shardingActions.ShardAdd.PreCondition = &immediately

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Output: []byte("shard-add-success"),
					}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.ShardAdd(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - ClusterReady", func() {
			clusterReady := appsv1.ClusterReadyPreConditionType
			shardingActions.ShardRemove.PreCondition = &clusterReady

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      clusterName,
						},
						Status: appsv1.ClusterStatus{
							Phase: appsv1.RunningClusterPhase,
						},
					},
				},
			}

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = shardingLifecycle.ShardRemove(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - ClusterReady fail", func() {
			clusterReady := appsv1.ClusterReadyPreConditionType
			shardingActions.ShardRemove.PreCondition = &clusterReady

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      clusterName,
						},
						Status: appsv1.ClusterStatus{
							Phase: appsv1.FailedClusterPhase,
						},
					},
				},
			}

			err = shardingLifecycle.ShardRemove(ctx, reader, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("cluster is not ready"))
		})

		It("precondition - unknown type", func() {
			unknownType := appsv1.PreConditionType("UnknownType")
			shardingActions.PostProvision.PreCondition = &unknownType

			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			err = shardingLifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("unknown precondition type"))
		})

		It("multiple actions success", func() {
			shardingLifecycle, err := NewShardingLifecycle(namespace, clusterName, compName, shardingName, shardingActions, nil, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(shardingLifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					switch req.Action {
					case "shardPostProvision":
						return proto.ActionResponse{Output: []byte("post-provision-success")}, nil
					case "shardPreTerminate":
						return proto.ActionResponse{Output: []byte("pre-terminate-success")}, nil
					case "shardAdd":
						return proto.ActionResponse{Output: []byte("shard-add-success")}, nil
					case "shardRemove":
						return proto.ActionResponse{Output: []byte("shard-remove-success")}, nil
					default:
						return proto.ActionResponse{}, fmt.Errorf("unknown action: %s", req.Action)
					}
				}).AnyTimes()
			})

			// Test all actions
			err = shardingLifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())

			err = shardingLifecycle.PreTerminate(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())

			err = shardingLifecycle.ShardAdd(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())

			err = shardingLifecycle.ShardRemove(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})
	})
})
