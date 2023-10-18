/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package configuration

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metautil "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/pkg/configuration/proto/mocks"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	var (
		k8sMockClient     *testutil.K8sClientMockHelper
		mockParam         reconfigureParams
		reconfigureClient *mock_proto.MockReconfigureClient

		defaultReplica = 3
		rollingPolicy  = upgradePolicyMap[appsv1alpha1.RollingPolicy]
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

	createReconfigureParam := func(compType appsv1alpha1.WorkloadType, replicas int) reconfigureParams {
		return newMockReconfigureParams("rollingPolicy", k8sMockClient.Client(),
			withMockStatefulSet(replicas, nil),
			withConfigSpec("for_test", map[string]string{
				"key": "value",
			}),
			withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
				return reconfigureClient, nil
			}),
			withClusterComponent(replicas),
			withCDComponent(compType, []appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}}))
	}

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		reconfigureClient = mock_proto.NewMockReconfigureClient(k8sMockClient.Controller())
		mockParam = createReconfigureParam(appsv1alpha1.Consensus, defaultReplica)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	Context("consensus rolling reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(rollingPolicy.GetPolicyName()).Should(BeEquivalentTo("rolling"))

			mockLeaderLabel := func(pod *corev1.Pod, i int) {
				if pod.Labels == nil {
					pod.Labels = make(map[string]string, 1)
				}
				if i == 1 {
					pod.Labels[constant.RoleLabelKey] = "leader"
				} else {
					pod.Labels[constant.RoleLabelKey] = "follower"
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

			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(mockPods[0]),
					fromPodObjectList(mockPods[1]),
					fromPodObjectList(mockPods[2]),
				}, func(sequence int, r []runtime.Object) { acc = sequence }), testutil.WithAnyTimes()))

			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				pod, _ := obj.(*corev1.Pod)
				// mock patch
				updateLabelPatch(mockPods[acc], pod)
				return nil
			}, testutil.WithAnyTimes()))

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
				mockParam.Component.WorkloadType = appsv1alpha1.Stateful
				mockParam.Component.StatefulSpec = &appsv1alpha1.StatefulSetSpec{
					LLUpdateStrategy: &apps.StatefulSetUpdateStrategy{
						RollingUpdate: &apps.RollingUpdateStatefulSetStrategy{
							MaxUnavailable: func() *metautil.IntOrString { v := metautil.FromString("100%"); return &v }(),
						},
					},
				}
				pods = newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], defaultReplica)
			}

			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(fromPodObjectList(pods)),
				testutil.WithMinTimes(3)))

			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				pod, _ := obj.(*corev1.Pod)
				updateLabelPatch(pods, pod)
				return nil
			}, testutil.WithTimes(defaultReplica)))

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
			}).Should(BeTrue())

			status, err = rollingPolicy.Upgrade(mockParam)
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("rolling reconfigure policy test for not supported component", func() {
		It("Should failed", func() {
			// not supported type
			_ = mockParam
			k8sMockClient.MockListMethod(testutil.WithSucceed(testutil.WithTimes(0)))

			status, err := rollingPolicy.Upgrade(createReconfigureParam(appsv1alpha1.Stateless, defaultReplica))
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not supported component workload type"))
			Expect(status.Status).Should(BeEquivalentTo(ESNotSupport))
		})
	})
})
