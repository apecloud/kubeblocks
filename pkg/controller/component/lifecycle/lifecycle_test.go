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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
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
		synthesizedComp *component.SynthesizedComponent
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

		synthesizedComp = &component.SynthesizedComponent{
			Namespace:   "default",
			ClusterName: "test-cluster",
			Name:        "kbagent",
			PodSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test-kbagent",
					},
				},
			},
			LifecycleActions: &appsv1alpha1.ComponentLifecycleActions{
				PostProvision: &appsv1alpha1.Action{
					Exec: &appsv1alpha1.ExecAction{
						Command: []string{"/bin/bash", "-c", "echo -n post-provision"},
					},
					TimeoutSeconds: 5,
					RetryPolicy: &appsv1alpha1.RetryPolicy{
						MaxRetries:    5,
						RetryInterval: 10,
					},
				},
				RoleProbe: &appsv1alpha1.Probe{
					Action: appsv1alpha1.Action{
						Exec: &appsv1alpha1.ExecAction{
							Command: []string{"/bin/bash", "-c", "echo -n role-probe"},
						},
						TimeoutSeconds: 5,
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       1,
					SuccessThreshold:    3,
					FailureThreshold:    3,
				},
			},
		}

		pods = []*corev1.Pod{
			{},
		}
	})

	AfterEach(func() {
		cleanEnv()

		kbacli.UnsetMockClient()
	})

	Context("new", func() {
		It("nil pod", func() {
			_, err := New(nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("either pod or pods must be provided to call lifecycle actions"))
		})

		It("pod", func() {
			pod := pods[0]
			lifecycle, err := New(synthesizedComp, pod)
			Expect(err).Should(BeNil())

			Expect(lifecycle).ShouldNot(BeNil())
			agent := lifecycle.(*kbagent)
			Expect(agent.synthesizedComp).Should(Equal(synthesizedComp))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
		})

		It("pods", func() {
			pod := pods[0]
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())

			Expect(lifecycle).ShouldNot(BeNil())
			agent := lifecycle.(*kbagent)
			Expect(agent.synthesizedComp).Should(Equal(synthesizedComp))
			Expect(agent.pod).Should(Equal(pod))
			Expect(agent.pods).Should(HaveLen(1))
			Expect(agent.pods[0]).Should(Equal(pod))
		})
	})

	Context("call action", func() {
		It("not defined", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PreTerminate(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, ErrActionNotDefined)).Should(BeTrue())
		})

		It("action request", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			action := synthesizedComp.LifecycleActions.PostProvision
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("postProvision"))
					Expect(req.Parameters).ShouldNot(BeNil()) // legacy parameters for post-provision action
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
			err = lifecycle.PostProvision(ctx, k8sClient, opts)
			Expect(err).Should(BeNil())
		})

		It("succeed", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
		})

		It("succeed and stdout", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
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
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			unknownErr := fmt.Errorf("%s", "unknown error")
			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrNotDefined
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrNotImplemented
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrInProgress
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrBusy
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrTimeout
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrFailed
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, service.ErrInternalError
				}).MaxTimes(1)
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, unknownErr
				}).MaxTimes(1)
			})

			for _, expected := range []error{
				ErrActionNotDefined,
				ErrActionNotImplemented,
				ErrActionInProgress,
				ErrActionBusy,
				ErrActionTimeout,
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
			// TODO: pass error message from kb-agent
		})

		It("parameters", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1alpha1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: synthesizedComp.Namespace,
							Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name),
							Labels: map[string]string{
								constant.AppInstanceLabelKey: synthesizedComp.ClusterName,
							},
						},
					},
					&appsv1alpha1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: synthesizedComp.Namespace,
							Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, "another"),
							Labels: map[string]string{
								constant.AppInstanceLabelKey: synthesizedComp.ClusterName,
							},
						},
					},
				},
			}

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					Expect(req.Action).Should(Equal("postProvision"))
					Expect(req.Parameters).ShouldNot(BeNil()) // legacy parameters for post-provision action
					Expect(req.Parameters[hackedAllCompList]).Should(Equal(strings.Join([]string{synthesizedComp.Name, "another"}, ",")))
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("template vars", func() {
			key := "TEMPLATE_VAR1"
			val := "template-vars1"
			synthesizedComp.TemplateVars = map[string]any{key: val}

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
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
			clusterReady := appsv1alpha1.ClusterReadyPreConditionType
			synthesizedComp.LifecycleActions.PostProvision.PreCondition = &clusterReady

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: synthesizedComp.Namespace,
							Name:      synthesizedComp.ClusterName,
						},
						Status: appsv1alpha1.ClusterStatus{
							Phase: appsv1alpha1.RunningClusterPhase,
						},
					},
				},
			}

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).Should(BeNil())
		})

		It("precondition - fail", func() {
			clusterReady := appsv1alpha1.ClusterReadyPreConditionType
			synthesizedComp.LifecycleActions.PostProvision.PreCondition = &clusterReady

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			reader := &mockReader{
				cli: k8sClient,
				objs: []client.Object{
					&appsv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: synthesizedComp.Namespace,
							Name:      synthesizedComp.ClusterName,
						},
						Status: appsv1alpha1.ClusterStatus{
							Phase: appsv1alpha1.FailedClusterPhase,
						},
					},
				},
			}

			err = lifecycle.PostProvision(ctx, reader, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("precondition check error"))
		})

		It("pod selector - any", func() {
			synthesizedComp.LifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1alpha1.AnyReplica
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
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
						Namespace: synthesizedComp.Namespace,
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

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(Or(ContainSubstring("pod pod-0 has no ip"), ContainSubstring("pod pod-1 has no ip")))
		})

		It("pod selector - all", func() {
			// TODO: impl
		})

		It("pod selector - role", func() {
			synthesizedComp.LifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1alpha1.RoleSelector
			synthesizedComp.LifecycleActions.PostProvision.Exec.MatchingKey = "leader"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
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
						Namespace: synthesizedComp.Namespace,
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

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("pod pod-1 has no ip"))
		})

		It("pod selector - has no matched", func() {
			synthesizedComp.LifecycleActions.PostProvision.Exec.TargetPodSelector = appsv1alpha1.RoleSelector
			synthesizedComp.LifecycleActions.PostProvision.Exec.MatchingKey = "leader"
			pods = []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
						Name:      "pod-0",
						Labels: map[string]string{
							constant.RoleLabelKey: "follower",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
						Name:      "pod-1",
						Labels: map[string]string{
							constant.RoleLabelKey: "follower",
						},
					},
				},
			}

			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("no available pod to call action"))
		})

		It("non-blocking", func() {
			// TODO: impl
		})

		It("timeout", func() {
			// TODO: impl
		})
	})
})
