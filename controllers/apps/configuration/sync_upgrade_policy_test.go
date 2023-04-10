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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/mock/gomock"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/internal/configuration/proto/mocks"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
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

})
