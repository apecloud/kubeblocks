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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

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
						Command: []string{"echo", "post-provision"},
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
							Command: []string{"echo", "role-probe"},
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
			&corev1.Pod{},
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

		It("succeed and stdout", func() {
			lifecycle, err := New(synthesizedComp, nil, pods...)
			Expect(err).Should(BeNil())
			Expect(lifecycle).ShouldNot(BeNil())

			mockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.CallAction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
					return proto.ActionResponse{
						Output: []byte("post-provision"),
					}, nil
				}).AnyTimes()
			})

			err = lifecycle.PostProvision(ctx, k8sClient, nil)
			Expect(err).Should(BeNil())
			// TODO: rsp
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

		It("fail - stdout & stderr", func() {
		})

		It("parameters", func() {
		})

		It("template vars", func() {
		})

		It("precondition", func() {
		})

		It("pod selector", func() {
		})

		It("non-blocking", func() {
		})

		It("timeout", func() {
		})

		// TODO: back-off to other pods
	})
})
