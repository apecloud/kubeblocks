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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type mockReader struct {
	cli  client.Reader
	objs []client.Object
}

func (r *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	for _, o := range r.objs {
		// ignore the GVK check
		if client.ObjectKeyFromObject(o) == key {
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(o).Elem())
			return nil
		}
	}
	return r.cli.Get(ctx, key, obj, opts...)
}

func (r *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}
	objects := reflect.MakeSlice(items.Type(), 0, 0)

	listOpts := &client.ListOptions{}
	for _, opt := range opts {
		opt.ApplyToList(listOpts)
	}
	for i, o := range r.objs {
		// ignore the GVK check
		if listOpts.LabelSelector != nil {
			if listOpts.LabelSelector.Matches(labels.Set(o.GetLabels())) {
				objects = reflect.Append(objects, reflect.ValueOf(r.objs[i]).Elem())
			}
		}
	}
	if objects.Len() != 0 {
		items.Set(objects)
		return nil
	}
	return r.cli.List(ctx, list, opts...)
}

var mockKBAgentClient = func(mock func(*kbacli.MockClientMockRecorder)) {
	cli := kbacli.NewMockClient(gomock.NewController(GinkgoT()))
	if mock != nil {
		mock(cli.EXPECT())
	}
	kbacli.SetMockClient(cli, nil)
}

var _ = Describe("lifecycle", func() {
	var (
		namespace        string
		clusterName      string
		compName         string
		lifecycleActions *appsv1.ComponentLifecycleActions
		pods             []*corev1.Pod
		customAction     *appsv1.Action
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
		retryIntervalSeconds := int64(10)
		lifecycleActions = &appsv1.ComponentLifecycleActions{
			PostProvision: &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"/bin/bash", "-c", "echo -n post-provision"},
				},
				TimeoutSeconds: 5,
				RetryPolicy: &appsv1.RetryPolicy{
					MaxRetries:           5,
					RetryInterval:        10,
					RetryIntervalSeconds: &retryIntervalSeconds,
				},
			},
			RoleProbe: &appsv1.Probe{
				Action: appsv1.Action{
					Exec: &appsv1.ExecAction{
						Command: []string{"/bin/bash", "-c", "echo -n role-probe"},
					},
					TimeoutSeconds: 5,
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       1,
				SuccessThreshold:    3,
				FailureThreshold:    3,
			},
		}
		customAction = &appsv1.Action{
			Exec: &appsv1.ExecAction{
				Command: []string{"/bin/bash", "-c", "echo -n custom-action"},
			},
			TimeoutSeconds: 5,
			RetryPolicy: &appsv1.RetryPolicy{
				MaxRetries:           5,
				RetryInterval:        10,
				RetryIntervalSeconds: &retryIntervalSeconds,
			},
		}
		pods = []*corev1.Pod{{}}
	})

	AfterEach(func() {
		cleanEnv()

		kbacli.UnsetMockClient()
	})

	Context("new", func() {
		It("nil pod", func() {
			_, err := New("", "", "", nil, nil, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("pods must be provided to call lifecycle actions"))
		})

		It("pod", func() {
			pod := pods[0]
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, pod, pods)
			Expect(err).Should(BeNil())

			Expect(lifecycle).ShouldNot(BeNil())
			agent := lifecycle.(*kbagent)
			Expect(agent.namespace).Should(Equal(namespace))
			Expect(agent.clusterName).Should(Equal(clusterName))
			Expect(agent.compName).Should(Equal(compName))
			Expect(agent.lifecycleActions).Should(Equal(lifecycleActions))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
		})

		It("pods", func() {
			pod := pods[0]
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())

			Expect(lifecycle).ShouldNot(BeNil())
			agent := lifecycle.(*kbagent)
			Expect(agent.namespace).Should(Equal(namespace))
			Expect(agent.clusterName).Should(Equal(clusterName))
			Expect(agent.compName).Should(Equal(compName))
			Expect(agent.lifecycleActions).Should(Equal(lifecycleActions))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
		})
	})

	DescribeTable("normalizes retry policy at the Apps API-to-kbagent boundary",
		func(policy *appsv1.RetryPolicy, expected time.Duration) {
			wirePolicy := BuildKBAgentRetryPolicy(policy)
			Expect(wirePolicy).ShouldNot(BeNil())
			Expect(wirePolicy.MaxRetries).Should(Equal(policy.MaxRetries))
			Expect(wirePolicy.RetryInterval).Should(Equal(expected))
		},
		Entry("preserves the legacy duration", &appsv1.RetryPolicy{
			MaxRetries: 3, RetryInterval: 1500 * time.Millisecond,
		}, 1500*time.Millisecond),
		Entry("converts explicit seconds", &appsv1.RetryPolicy{
			MaxRetries: 3, RetryIntervalSeconds: ptr.To[int64](1),
		}, time.Second),
		Entry("explicit seconds take precedence", &appsv1.RetryPolicy{
			MaxRetries: 3, RetryInterval: time.Nanosecond, RetryIntervalSeconds: ptr.To[int64](1),
		}, time.Second),
		Entry("explicit zero takes precedence", &appsv1.RetryPolicy{
			MaxRetries: 3, RetryInterval: time.Second, RetryIntervalSeconds: ptr.To[int64](0),
		}, time.Duration(0)),
		Entry("seconds overflow saturates", &appsv1.RetryPolicy{
			MaxRetries: 3, RetryIntervalSeconds: ptr.To[int64](1<<63 - 1),
		}, time.Duration(1<<63-1)),
	)

	It("keeps an absent retry policy absent at the kbagent boundary", func() {
		Expect(BuildKBAgentRetryPolicy(nil)).Should(BeNil())
	})

	Context("call action", func() {
		It("not defined", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PreTerminate(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionNotDefined)).Should(BeTrue())
		})

		It("action request", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			action := lifecycleActions.PostProvision
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("postProvision"))
					Expect(req.Parameters).Should(BeEmpty())
					Expect(req.NonBlocking).ShouldNot(BeNil())
					Expect(*req.NonBlocking).Should(BeTrue())
					Expect(req.TimeoutSeconds).ShouldNot(BeNil())
					Expect(*req.TimeoutSeconds).Should(Equal(action.TimeoutSeconds))
					Expect(req.RetryPolicy).ShouldNot(BeNil())
					Expect(req.RetryPolicy.MaxRetries).Should(Equal(action.RetryPolicy.MaxRetries))
					Expect(req.RetryPolicy.RetryInterval).Should(Equal(10 * time.Second))
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			opts := &Options{
				NonBlocking:    &[]bool{true}[0],
				TimeoutSeconds: &action.TimeoutSeconds,
				RetryPolicy:    action.RetryPolicy,
			}
			err = lifecycle.PostProvision(ctx, k8sClient, opts)
			Expect(err).Should(BeNil())
		})

		It("action request arguments", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			arguments := [][]string{{"maxmemory", "1gb"}, {"timeout", "30"}}
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("postProvision"))
					Expect(req.Arguments).Should(Equal(arguments))
					Expect(req.Parameters).Should(BeEmpty())
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, &Options{Arguments: arguments})
			Expect(err).Should(BeNil())
		})

		It("succeed", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("succeed and stdout", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Output: []byte("role-probe"),
					}, nil
				}).AnyTimes()
			})

			output, err1 := lifecycle.RoleProbe(ctx, k8sClient, nil)
			Expect(err1).Should(BeNil())
			Expect(output).Should(Equal([]byte("role-probe")))
		})

		It("fail - error code", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			unknownErr := fmt.Errorf("%s", "unknown error")
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrNotDefined),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrNotImplemented),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrBadRequest),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrInProgress),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrBusy),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrTimedOut),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrFailed),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error: proto.Error2Type(proto.ErrInternalError),
					}, nil
				}).MaxTimes(1)
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, unknownErr
				}).MaxTimes(1)
			})

			for _, expected := range []error{
				ErrActionNotDefined,
				ErrActionNotImplemented,
				ErrActionInternalError,
				ErrActionInProgress,
				ErrActionBusy,
				ErrActionTimedOut,
				ErrActionFailed,
				ErrActionInternalError,
				unknownErr,
			} {
				err = lifecycle.PostProvision(ctx, k8sClient, nil)
				Expect(err).ShouldNot(BeNil())
				Expect(errors.Is(err, expected)).Should(BeTrue())
			}
		})

		It("fail - error msg", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Error:   proto.Error2Type(proto.ErrFailed),
						Message: "command not found",
					}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("command not found"))
		})

		It("parameters", func() {
			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      constant.GenerateClusterComponentName(clusterName, compName),
							Labels: map[string]string{
								constant.AppInstanceLabelKey: clusterName,
							},
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      constant.GenerateClusterComponentName(clusterName, "another"),
							Labels: map[string]string{
								constant.AppInstanceLabelKey: clusterName,
							},
						},
					},
				},
			}

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("postProvision"))
					Expect(req.Parameters).Should(BeEmpty())
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("template vars", func() {
			key := "TEMPLATE_VAR1"
			val := "template-vars1"

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, map[string]string{key: val}, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("roleProbe"))
					Expect(req.Parameters).ShouldNot(BeNil())
					Expect(req.Parameters[key]).Should(Equal(val))
					return proto.ActionResponse{
						Output: []byte(req.Parameters[key]),
					}, nil
				}).AnyTimes()
			})

			output, err1 := lifecycle.RoleProbe(ctx, k8sClient, nil)
			Expect(err1).Should(BeNil())
			Expect(output).Should(Equal([]byte(val)))
		})

		It("precondition", func() {
			clusterReady := appsv1.ClusterReadyPreConditionType
			lifecycleActions.PostProvision.PreCondition = &clusterReady

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

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

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - fail", func() {
			clusterReady := appsv1.ClusterReadyPreConditionType
			lifecycleActions.PostProvision.PreCondition = &clusterReady

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

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

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("action precondition is not met"))
		})

		It("precondition - object selector", func() {
			componentReady := appsv1.ComponentReadyPreConditionType
			labels := map[string]string{
				"test": "test",
			}

			lifecycle, err := New(namespace, clusterName, compName, nil, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "comp-1",
							Labels:    labels,
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningComponentPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "comp-2",
							Labels:    labels,
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

			customAction.PreCondition = &componentReady
			err = lifecycle.UserDefined(ctx, reader, &Options{
				PreConditionObjectSelector: labels,
			}, "custom-action", customAction, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - object selector fail", func() {
			componentReady := appsv1.ComponentReadyPreConditionType
			labels := map[string]string{
				"test": "test",
			}

			lifecycle, err := New(namespace, clusterName, compName, nil, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "comp-1",
							Labels:    labels,
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.RunningComponentPhase,
						},
					},
					&appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: namespace,
							Name:      "comp-2",
							Labels:    labels,
						},
						Status: appsv1.ComponentStatus{
							Phase: appsv1.FailedComponentPhase,
						},
					},
				},
			}

			customAction.PreCondition = &componentReady
			err = lifecycle.UserDefined(ctx, reader, &Options{
				PreConditionObjectSelector: labels,
			}, "custom-action", customAction, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("action precondition is not met"))
		})

		It("pod selector - any", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.AnyReplica
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-1",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(Or(ContainSubstring("pod pod-0 has no ip"), ContainSubstring("pod pod-1 has no ip")))
		})

		It("pod selector - all", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.AllReplicas
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-1",
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).Times(2)
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("pod selector - all lets a later replica unblock an earlier primary", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.AllReplicas
			pods = []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "primary-0"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "replica-0"}},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			totalCallCount := 0
			roundCallCount := 0
			replicaAttached := false
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					totalCallCount++
					roundCallCount++
					// The stable selection order is primary first, replica second. The
					// primary cannot finish until the replica has executed its own action.
					if roundCallCount == 1 && !replicaAttached {
						return proto.ActionResponse{Error: proto.Error2Type(proto.ErrFailed)}, nil
					}
					if roundCallCount == 2 {
						replicaAttached = true
					}
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())
			Expect(roundCallCount).Should(Equal(2))
			Expect(replicaAttached).Should(BeTrue())

			roundCallCount = 0
			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
			Expect(roundCallCount).Should(Equal(2))
			Expect(totalCallCount).Should(Equal(4))
		})

		It("pod selector - all aggregates every pod error with pod identity", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.AllReplicas
			pods = []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-0"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-1"}},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			callCount := 0
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					callCount++
					if callCount == 1 {
						return proto.ActionResponse{Error: proto.Error2Type(proto.ErrInProgress)}, nil
					}
					return proto.ActionResponse{Error: proto.Error2Type(proto.ErrBusy)}, nil
				}).Times(2)
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionInProgress)).Should(BeTrue())
			Expect(errors.Is(err, ErrActionBusy)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("pod-0"))
			Expect(err.Error()).Should(ContainSubstring("pod-1"))
			Expect(callCount).Should(Equal(2))
		})

		It("pod selector - all ignores NotDefined only when every pod is NotDefined", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.AllReplicas
			pods = []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-0"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-1"}},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			callCount := 0
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					callCount++
					if callCount == 1 {
						return proto.ActionResponse{Error: proto.Error2Type(proto.ErrNotDefined)}, nil
					}
					return proto.ActionResponse{Error: proto.Error2Type(proto.ErrFailed)}, nil
				}).Times(2)
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionNotDefined)).Should(BeFalse())
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())
			Expect(IgnoreNotDefined(err)).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("pod-0"))
			Expect(err.Error()).Should(ContainSubstring("pod-1"))

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Return(proto.ActionResponse{Error: proto.Error2Type(proto.ErrNotDefined)}, nil).Times(2)
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(errors.Is(err, ErrActionNotDefined)).Should(BeTrue())
			Expect(IgnoreNotDefined(err)).Should(BeNil())
		})

		It("pod selector - role keeps first-error behavior", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.RoleSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "leader"
			pods = []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-0", Labels: map[string]string{constant.RoleLabelKey: "leader"}}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "pod-1", Labels: map[string]string{constant.RoleLabelKey: "leader"}}},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			callCount := 0
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					callCount++
					return proto.ActionResponse{Error: proto.Error2Type(proto.ErrFailed)}, nil
				}).Times(1)
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(errors.Is(err, ErrActionFailed)).Should(BeTrue())
			Expect(callCount).Should(Equal(1))
		})

		It("pod selector - role", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.RoleSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "leader"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
						Labels: map[string]string{
							constant.RoleLabelKey: "follower",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-1",
						Labels: map[string]string{
							constant.RoleLabelKey: "leader",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("pod pod-1 has no ip"))
		})

		It("pod selector - ordinal", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.OrdinalSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "1"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-1",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kbagent",
								Ports: []corev1.ContainerPort{
									{
										Name: "http",
									},
								},
							},
						},
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("pod pod-1 has no ip"))
		})

		It("pod selector - ordinal invalid matching key", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.OrdinalSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "invalid"

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("invalid ordinal matchingKey: invalid"))
		})

		It("pod selector - ordinal ambiguous", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.OrdinalSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "0"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-template-0",
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("ambiguous ordinal selector matchingKey 0 matches multiple pods: pod-0,pod-template-0"))
		})

		It("pod selector - has no matched", func() {
			lifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1.RoleSelector
			lifecycleActions.PostProvision.Exec.MatchingKey = "leader"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-0",
						Labels: map[string]string{
							constant.RoleLabelKey: "follower",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pod-1",
						Labels: map[string]string{
							constant.RoleLabelKey: "follower",
						},
					},
				},
			}

			lifecycle, err := New(namespace, clusterName, compName, lifecycleActions, nil, nil, pods)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("no available pod to execute action"))
		})

		It("non-blocking", func() {
			// TODO: impl
		})

		It("timeout", func() {
			// TODO: impl
		})
	})
})
