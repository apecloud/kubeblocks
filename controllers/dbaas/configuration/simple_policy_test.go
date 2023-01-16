/*
Copyright ApeCloud Inc.

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

	"github.com/golang/mock/gomock"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	mock_client "github.com/apecloud/kubeblocks/controllers/dbaas/configuration/mocks"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var _ = Describe("Reconfigure simplePolicy", func() {

	var (
		mockClient *mock_client.MockClient
		ctrl       *gomock.Controller

		simplePolicy = upgradePolicyMap[dbaasv1alpha1.NormalPolicy]
	)

	setup := func() (*gomock.Controller, *mock_client.MockClient) {
		ctrl := gomock.NewController(GinkgoT())
		client := mock_client.NewMockClient(ctrl)
		return ctrl, client
	}

	BeforeEach(func() {
		ctrl, mockClient = setup()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		ctrl.Finish()
	})

	Context("simple reconfigure policy test", func() {
		It("Should success without error", func() {
			Expect(simplePolicy.GetPolicyName()).Should(BeEquivalentTo("simple"))

			// mock client update caller
			updateErr := cfgcore.MakeError("update failed!")
			mockClient.EXPECT().Update(gomock.Any(), gomock.Any()).
				Return(updateErr).
				Times(1)
			mockClient.EXPECT().Update(gomock.Any(), gomock.Any()).
				Return(nil).
				Times(1)

			mockParam := newMockReconfigureParams("simplePolicy", mockClient,
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))
			status, err := simplePolicy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status).Should(BeEquivalentTo(ESAndRetryFailed))
			status, err = simplePolicy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status).Should(BeEquivalentTo(ESNone))
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
			Expect(status).Should(BeEquivalentTo(ESNotSupport))
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
			Expect(err.Error()).Should(ContainSubstring("failed to found config meta"))
			Expect(status).Should(BeEquivalentTo(ESFailed))
		})
	})
})
