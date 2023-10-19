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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/pkg/configuration/proto/mocks"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var parallelPolicy = parallelUpgradePolicy{}

var _ = Describe("Reconfigure ParallelPolicy", func() {

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

	Context("parallel reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(parallelPolicy.GetPolicyName()).Should(BeEquivalentTo("parallel"))

			// mock client update caller
			k8sMockClient.MockPatchMethod(testutil.WithSucceed(testutil.WithTimes(3)))

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, nil).
				Times(3)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withClusterComponent(3),
				withConfigSpec("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					},
				}}))

			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(fromPodObjectList(
					newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3),
				))))

			status, err := parallelPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("parallel reconfigure policy test with List pods failed", func() {
		It("Should failed", func() {
			mockParam := newMockReconfigureParams("parallelPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withClusterComponent(3),
				withConfigSpec("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					},
				}}))

			// first failed
			getPodsError := cfgcore.MakeError("for grpc failed.")
			k8sMockClient.MockListMethod(testutil.WithFailed(getPodsError))

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(getPodsError))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})
	})

	Context("parallel reconfigure policy test with stop container failed", func() {
		It("Should failed", func() {
			stopError := cfgcore.MakeError("failed to stop!")
			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, stopError).
				Times(1)

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{
					ErrMessage: "failed to stop container.",
				}, nil).
				Times(1)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withClusterComponent(3),
				withConfigSpec("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))

			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3))), testutil.WithTimes(2),
			))

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(stopError))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))

			status, err = parallelPolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to stop container"))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})
	})

	Context("parallel reconfigure policy test with patch failed", func() {
		It("Should failed", func() {
			// mock client update caller
			patchError := cfgcore.MakeError("update failed!")
			k8sMockClient.MockPatchMethod(testutil.WithFailed(patchError, testutil.WithTimes(1)))

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, nil).
				Times(1)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sMockClient.Client(),
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withClusterComponent(3),
				withConfigSpec("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(appsv1alpha1.Consensus, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))

			setPods := newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 5)
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(fromPodObjectList(setPods)), testutil.WithAnyTimes(),
			))

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(patchError))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})
	})

	Context("parallel reconfigure policy test for not supported component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("parallelPolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2),
				withCDComponent(appsv1alpha1.Stateless, []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:       "for_test",
						VolumeName: "test_volume",
					}}}))
			status, err := parallelPolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not supported component workload type"))
			Expect(status.Status).Should(BeEquivalentTo(ESNotSupport))
		})
	})
})
