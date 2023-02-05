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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Reconfigure simplePolicy", func() {

	var (
		// mockClient *mock_client.MockClient
		// ctrl       *gomock.Controller

		k8sMockClient *testutil.K8sClientMockHelper

		simplePolicy = upgradePolicyMap[dbaasv1alpha1.NormalPolicy]
	)

	BeforeEach(func() {
		// ctrl, mockClient = testutil.SetupK8sMock()
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		// ctrl.Finish()
		k8sMockClient.Finish()
	})

	Context("simple reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(simplePolicy.GetPolicyName()).Should(BeEquivalentTo("simple"))

			mockParam := newMockReconfigureParams("simplePolicy", k8sMockClient.Client(),
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			// mock client update caller
			updateErr := cfgcore.MakeError("update failed!")
			k8sMockClient.MockUpdateMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithTimes(1)))
			k8sMockClient.MockListMethod(testutil.WithListReturned(
				testutil.WithConstructListReturnedResult(
					fromPodObjectList(newMockPodsWithStatefulSet(&mockParam.ComponentUnits[0], 2))),
				testutil.WithTimes(0),
			))

			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status.Status).Should(BeEquivalentTo(ESAndRetryFailed))
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})
	})

	Context("simple reconfigure policy test without not support component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("simplePolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Stateless, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))
			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not support component type"))
			Expect(status.Status).Should(BeEquivalentTo(ESNotSupport))
		})
	})

	Context("simple reconfigure policy test without not configmap volume", func() {
		It("Should failed", func() {
			// mock not tpl
			mockParam := newMockReconfigureParams("simplePolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("not_tpl_name", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))
			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to find config meta"))
			Expect(status.Status).Should(BeEquivalentTo(ESFailed))
		})
	})
})
