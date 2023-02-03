/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metautil "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/internal/configuration/proto/mocks"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	var (
		mockClient        *mock_client.MockClient
		ctrl              *gomock.Controller
		mockParam         reconfigureParams
		reconfigureClient *mock_proto.MockReconfigureClient

		defaultReplica = 3
		rollingPolicy  = upgradePolicyMap[dbaasv1alpha1.RollingPolicy]
	)

	updateLabelPatch := func(pods []corev1.Pod, patch *corev1.Pod) {
		patchKey := client.ObjectKeyFromObject(patch)
		for i := range pods {
			orgPod := &pods[i]
			if client.ObjectKeyFromObject(orgPod) == patchKey {
				orgPod.Labels = patch.Labels
				break
			}
		}
	}

	createReconfigureParam := func(compType dbaasv1alpha1.ComponentType, replicas int) reconfigureParams {
		return newMockReconfigureParams("rollingPolicy", mockClient,
			withMockStatefulSet(replicas, nil),
			withConfigTpl("for_test", map[string]string{
				"key": "value",
			}),
			withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
				return reconfigureClient, nil
			}),
			withClusterComponent(replicas),
			withCDComponent(compType, []dbaasv1alpha1.ConfigTemplate{{
				Name:       "for_test",
				VolumeName: "test_volume",
			}}))
	}

	BeforeEach(func() {
		ctrl, mockClient = testutil.SetupK8sMock()
		reconfigureClient = mock_proto.NewMockReconfigureClient(ctrl)
		mockParam = createReconfigureParam(dbaasv1alpha1.Consensus, defaultReplica)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		ctrl.Finish()
	})

	Context("consensus rolling reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(rollingPolicy.GetPolicyName()).Should(BeEquivalentTo("rolling"))

			mockLeaderLabel := func(pod *corev1.Pod, i int) {
				if pod.Labels == nil {
					pod.Labels = make(map[string]string, 1)
				}
				if i == 1 {
					pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = "leader"
				} else {
					pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = "follower"
				}
			}

			acc := 0
			mockPods := [][]corev1.Pod{
				newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2),
				newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 5,
					mockLeaderLabel),
				newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3,
					withReadyPod(0, 0),
					withAvailablePod(0, 3),
					mockLeaderLabel),
			}
			mockClient.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					Expect(testutil.SetListReturnedObjects(list, fromPods(mockPods[acc]))).Should(Succeed())
					if acc < len(mockPods)-1 {
						acc++
					}
					return nil
				}).
				AnyTimes()

			mockClient.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					pod, _ := obj.(*corev1.Pod)
					// mock patch
					updateLabelPatch(mockPods[acc], pod)
					return nil
				}).AnyTimes()

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).
				Return(&cfgproto.StopContainerResponse{}, nil).
				AnyTimes()

			// mock wait the number of pods to target replicas
			status, err := rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// mock wait the number of pods to ready status
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// upgrade pod-0
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][0].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][2].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// upgrade pod-2
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][2].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).ShouldNot(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// upgrade pod-1
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(mockPods[acc][1].Labels[mockParam.getConfigKey()]).Should(BeEquivalentTo(mockParam.getTargetVersionHash()))

			// finish check, not upgrade
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("statefulSet rolling reconfigure policy test", func() {
		It("Should success without error", func() {

			// for mock sts
			var pods []corev1.Pod
			{
				mockParam.Component.ComponentType = dbaasv1alpha1.Stateful
				mockParam.Component.PDBSpec = &policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: func() *metautil.IntOrString { v := metautil.FromString("100%"); return &v }(),
				}
				pods = newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], defaultReplica)
			}

			mockClient.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					Expect(testutil.SetListReturnedObjects(list, fromPods(pods))).Should(Succeed())
					return nil
				}).
				MinTimes(3)

			mockClient.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					pod, _ := obj.(*corev1.Pod)
					updateLabelPatch(pods, pod)
					return nil
				}).
				Times(defaultReplica)

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).
				Return(&cfgproto.StopContainerResponse{}, nil).
				Times(defaultReplica)

			// mock wait the number of pods to target replicas
			status, err := rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// finish check, not finished
			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))

			// mock async update state
			go func() {
				f := withAvailablePod(0, len(pods))
				for i := range pods {
					f(&pods[i], i)
				}
			}()

			// finish check, not finished
			Eventually(func() bool {
				status, err = rollingPolicy.Upgrade(mockParam)
				Expect(err).Should(Succeed())
				Expect(status.Status).Should(BeElementOf(ESNone, ESRetry))
				return status.Status == ESNone
			}, time.Second*20, time.Second*1).Should(BeTrue())

			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("rolling reconfigure policy test without not support component", func() {
		It("Should failed", func() {
			// not support type
			_ = mockParam
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			status, err := rollingPolicy.Upgrade(createReconfigureParam(dbaasv1alpha1.Stateless, defaultReplica))
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not support component type"))
			Expect(status.Status).Should(BeEquivalentTo(ESNotSupport))
		})
	})
})
