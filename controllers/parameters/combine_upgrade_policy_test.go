/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure CombineSyncPolicy", func() {

	var (
		k8sMockClient *testutil.K8sClientMockHelper
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("combine reconfigure policy test", func() {
		It("Should success without error", func() {
			By("check normal policy name")
			testPolicyExecs := &combineUpgradePolicy{
				policyExecutors: []reconfigurePolicy{&testPolicy{}},
			}

			Expect(upgradePolicyMap[appsv1alpha1.DynamicReloadAndRestartPolicy]).ShouldNot(BeNil())

			mockParam := newMockReconfigureParams("simplePolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			Expect(testPolicyExecs.GetPolicyName()).Should(BeEquivalentTo(appsv1alpha1.DynamicReloadAndRestartPolicy))
			status, err := testPolicyExecs.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
		})

		It("Should success without error", func() {
			By("check failed policy name")
			testPolicyExecs := &combineUpgradePolicy{
				policyExecutors: []reconfigurePolicy{&testErrorPolicy{}},
			}

			mockParam := newMockReconfigureParams("simplePolicy", k8sMockClient.Client(),
				withMockInstanceSet(2, nil),
				withConfigSpec("for_test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2))

			Expect(testPolicyExecs.GetPolicyName()).Should(BeEquivalentTo(appsv1alpha1.DynamicReloadAndRestartPolicy))
			status, err := testPolicyExecs.Upgrade(mockParam)
			Expect(err).ShouldNot(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))
		})
	})
})

type testPolicy struct {
}

type testErrorPolicy struct {
}

func (t testErrorPolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	return makeReturnedStatus(ESFailedAndRetry), fmt.Errorf("testErrorPolicy failed")
}

func (t testErrorPolicy) GetPolicyName() string {
	return "testErrorPolicy"
}

func (t testPolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	return makeReturnedStatus(ESNone), nil
}

func (t testPolicy) GetPolicyName() string {
	return "testPolicy"
}
