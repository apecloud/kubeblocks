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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/pkg/configuration/proto/mocks"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var operatorSyncPolicy = &syncPolicy{}

var _ = Describe("Reconfigure OperatorSyncPolicy", func() {

	var (
		k8sMockClient     *testutil.K8sClientMockHelper
		reconfigureClient *mock_proto.MockReconfigureClient
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		reconfigureClient = mock_proto.NewMockReconfigureClient(k8sMockClient.Controller())
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("sync reconfigure policy test", func() {
		It("Should success without error", func() {
			By("check policy name")
			Expect(operatorSyncPolicy.GetPolicyName()).Should(BeEquivalentTo("operatorSyncUpdate"))

			By("prepare reconfigure policy params")
			mockParam := newMockReconfigureParams("operatorSyncPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigSpec("for_test", map[string]string{"a": "c b e f"}),
				withConfigConstraintSpec(&appsv1alpha1.FormatterConfig{Format: appsv1alpha1.RedisCfg}),
				withConfigPatch(map[string]string{
					"a": "c b e f",
				}),
				withClusterComponent(3),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					},
				}}))

			By("mock client get pod caller")
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3,
						withReadyPod(0, 1))),
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3,
						withReadyPod(0, 3))),
				}),
				testutil.WithAnyTimes()))

			By("mock client patch caller")
			// mock client update caller
			k8sMockClient.MockPatchMethod(testutil.WithSucceed(testutil.WithMinTimes(3)))

			By("mock remote online update caller")
			reconfigureClient.EXPECT().OnlineUpgradeParams(gomock.Any(), gomock.Any()).Return(
				&cfgproto.OnlineUpgradeParamsResponse{}, nil).
				MinTimes(3)

			status, err := operatorSyncPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(1))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))

			status, err = operatorSyncPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(3))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
		})
	})

	Context("sync reconfigure policy with selector test", func() {
		It("Should success without error", func() {
			By("check policy name")
			Expect(operatorSyncPolicy.GetPolicyName()).Should(BeEquivalentTo("operatorSyncUpdate"))

			By("prepare reconfigure policy params")
			mockParam := newMockReconfigureParams("operatorSyncPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigSpec("for_test", map[string]string{"a": "c b e f"}),
				withConfigConstraintSpec(&appsv1alpha1.FormatterConfig{Format: appsv1alpha1.RedisCfg}),
				withConfigPatch(map[string]string{
					"a": "c b e f",
				}),
				withClusterComponent(3),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					},
				}}))

			// add selector
			mockParam.ConfigConstraint.Selector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"primary": "true",
				},
			}

			By("mock client get pod caller")
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3,
						withReadyPod(0, 1), func(pod *corev1.Pod, index int) {
							if index == 0 {
								if pod.Labels == nil {
									pod.Labels = make(map[string]string)
								}
								pod.Labels["primary"] = "true"
							}
						}))),
				testutil.WithAnyTimes()))

			By("mock client patch caller")
			// mock client update caller
			k8sMockClient.MockPatchMethod(testutil.WithSucceed(testutil.WithTimes(1)))

			By("mock remote online update caller")
			reconfigureClient.EXPECT().OnlineUpgradeParams(gomock.Any(), gomock.Any()).Return(
				&cfgproto.OnlineUpgradeParamsResponse{}, nil).
				Times(1)

			status, err := operatorSyncPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(1))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(1))
		})
	})

})
