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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	mockproto "github.com/apecloud/kubeblocks/pkg/configuration/proto/mocks"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var operatorSyncPolicy = &syncPolicy{}

var _ = Describe("Reconfigure OperatorSyncPolicy", func() {

	var (
		k8sMockClient     *testutil.K8sClientMockHelper
		reconfigureClient *mockproto.MockReconfigureClient
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		reconfigureClient = mockproto.NewMockReconfigureClient(k8sMockClient.Controller())
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("sync reconfigure policy test", func() {
		It("Should success without error", func() {
			By("check policy name")
			Expect(operatorSyncPolicy.GetPolicyName()).Should(BeEquivalentTo("syncReload"))

			By("prepare reconfigure policy params")
			mockParam := newMockReconfigureParams("operatorSyncPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockInstanceSet(3, nil),
				withConfigSpec("for_test", map[string]string{"a": "c b e f"}),
				withConfigDescription(&parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.RedisCfg}),
				withUpdatedParameters(map[string]string{
					"a": "c b e f",
				}),
				withClusterComponent(3))

			By("mock client get pod caller")
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListSequenceResult([][]runtime.Object{
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 3,
						withReadyPod(0, 1))),
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 3,
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
			Expect(operatorSyncPolicy.GetPolicyName()).Should(BeEquivalentTo("syncReload"))

			By("prepare reconfigure policy params")
			mockParam := newMockReconfigureParams("operatorSyncPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockInstanceSet(3, nil),
				withConfigSpec("for_test", map[string]string{"a": "c b e f"}),
				withConfigDescription(&parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.RedisCfg}),
				withUpdatedParameters(map[string]string{
					"a": "c b e f",
				}),
				withClusterComponent(3))

			// add selector
			mockParam.ParametersDef = &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					TargetPodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"primary": "true",
						},
					},
				},
			}

			By("mock client get pod caller")
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(
					fromPodObjectList(newMockPodsWithInstanceSet(&mockParam.InstanceSetUnits[0], 3,
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
