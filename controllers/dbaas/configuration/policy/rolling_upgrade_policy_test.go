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

package policy

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	mock_client "github.com/apecloud/kubeblocks/controllers/dbaas/configuration/policy/mocks"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	mock_proto "github.com/apecloud/kubeblocks/internal/configuration/proto/mocks"
)

var rollingPolicy = RollingUpgradePolicy{}

var _ = Describe("Reconfigure RollingPolicy", func() {

	setup := func() (*gomock.Controller, *mock_client.MockClient) {
		ctrl := gomock.NewController(GinkgoT())
		client := mock_client.NewMockClient(ctrl)
		return ctrl, client
	}

	Context("rolling reconfigure policy test", func() {
		It("Should success without error", func() {

			Expect(rollingPolicy.GetPolicyName()).Should(BeEquivalentTo("rolling"))

			ctrl, k8sClient := setup()
			defer ctrl.Finish()

			mockParam := newMockReconfigureParams("rollingPolicy", k8sClient,
				withGRPCClient(func(addr string) (cfgproto.ReconfigureClient, error) {
					return mock_proto.NewMockReconfigureClient(ctrl), nil
				}),
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Consensus, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))

			Expect(&mockParam).ShouldNot(BeNil())
		})
	})

	Context("rolling reconfigure policy test without not support component", func() {
		It("Should failed", func() {
			// not support type
			mockParam := newMockReconfigureParams("rollingPolicy", nil,
				withMockStatefulSet(2, nil),
				withConfigTpl("for_test", map[string]string{
					"key": "value",
				}),
				withCDComponent(dbaasv1alpha1.Stateless, []dbaasv1alpha1.ConfigTemplate{{
					Name:       "for_test",
					VolumeName: "test_volume",
				}}))
			status, err := rollingPolicy.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not support component type"))
			Expect(status).Should(BeEquivalentTo(ESNotSupport))
		})
	})
})
