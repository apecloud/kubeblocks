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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/internal/configuration/proto/mocks"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var parallelPolicy = parallelUpgradePolicy{}

var _ = Describe("Reconfigure ParallelPolicy", func() {

	Context("parallel reconfigure policy test", func() {
		It("Should success without error", func() {

			Expect(parallelPolicy.GetPolicyName()).Should(BeEquivalentTo("parallel"))

			ctrl, k8sClient := testutil.SetupK8sMock()
			reconfigureClient := mock_proto.NewMockReconfigureClient(ctrl)
			defer ctrl.Finish()

			// mock client update caller
			k8sClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil).
				Times(3)
			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, nil).
				Times(3)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sClient,
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigTpl("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			setPods1 := newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3)

			k8sClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
					Expect(testutil.SetListReturnedObjects(list, fromPodObjectList(setPods1))).Should(Succeed())
				}).
				Return(nil)

			status, err := parallelPolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("parallel reconfigure policy test with List pods failed", func() {
		It("Should failed", func() {
			ctrl, k8sClient := testutil.SetupK8sMock()
			reconfigureClient := mock_proto.NewMockReconfigureClient(ctrl)
			defer ctrl.Finish()

			mockParam := newMockReconfigureParams("parallelPolicy", k8sClient,
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigTpl("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			// first failed
			getPodsError := cfgcore.MakeError("for grpc failed.")
			k8sClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(getPodsError)

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(getPodsError))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))
		})
	})

	Context("parallel reconfigure policy test with stop container failed", func() {
		It("Should failed", func() {
			ctrl, k8sClient := testutil.SetupK8sMock()
			reconfigureClient := mock_proto.NewMockReconfigureClient(ctrl)
			defer ctrl.Finish()

			stopError := cfgcore.MakeError("failed to stop!")
			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, stopError).
				Times(1)

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{
					ErrMessage: "failed to stop container.",
				}, nil).
				Times(1)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sClient,
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigTpl("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			setPods1 := newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 3)

			k8sClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
					Expect(testutil.SetListReturnedObjects(list, fromPodObjectList(setPods1))).Should(Succeed())
				}).
				Return(nil).
				Times(2)

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(stopError))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))

			status, err = parallelPolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to stop container"))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))
		})
	})

	Context("parallel reconfigure policy test with patch failed", func() {
		It("Should failed", func() {
			ctrl, k8sClient := testutil.SetupK8sMock()
			reconfigureClient := mock_proto.NewMockReconfigureClient(ctrl)
			defer ctrl.Finish()

			// mock client update caller
			patchError := cfgcore.MakeError("update failed!")
			k8sClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(patchError).
				Times(1)

			reconfigureClient.EXPECT().StopContainer(gomock.Any(), gomock.Any()).Return(
				&cfgproto.StopContainerResponse{}, nil).
				Times(1)

			mockParam := newMockReconfigureParams("parallelPolicy", k8sClient,
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return reconfigureClient, nil
				}),
				withMockStatefulSet(3, nil),
				withConfigTpl("for_test", map[string]string{
					"a": "b",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			setPods := newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 5)

			k8sClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
					Expect(testutil.SetListReturnedObjects(list, fromPodObjectList(setPods))).Should(Succeed())
				}).
				Return(nil)

			status, err := parallelPolicy.Upgrade(mockParam)
			// first failed
			Expect(err).Should(BeEquivalentTo(patchError))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))
		})
	})

	Context("parallel reconfigure policy test without not support component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("parallelPolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Stateless, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))
			status, err := parallelPolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not support component type"))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))
		})
	})
})
