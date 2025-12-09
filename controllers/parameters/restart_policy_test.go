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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure restartPolicy", func() {
	var (
		k8sMockClient *testutil.K8sClientMockHelper
		policy        = upgradePolicyMap[parametersv1alpha1.RestartPolicy]
	)

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("restart policy test", func() {
		It("should success without error", func() {
			mockParam := newMockReconfigureParams("restartPolicy", k8sMockClient.Client(),
				withConfigSpec("test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2),
				withWorkload())

			updateErr := fmt.Errorf("mock error")
			k8sMockClient.MockUpdateMethod(
				testutil.WithFailed(updateErr, testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithAnyTimes()))

			status, err := policy.Upgrade(mockParam)
			Expect(err).Should(BeEquivalentTo(updateErr))
			Expect(status.Status).Should(BeEquivalentTo(ESFailedAndRetry))

			// first upgrade, not pod is ready
			mockParam.its.Status.InstanceStatus = []workloads.InstanceStatus{}
			status, err = policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(0)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// only one pod ready
			mockParam.its.Status.InstanceStatus = append(mockParam.its.Status.InstanceStatus,
				workloads.InstanceStatus{
					PodName: "pod1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        mockParam.generateConfigIdentifier(),
							VersionHash: mockParam.getTargetVersionHash(),
						},
					},
				})
			status, err = policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(1)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// succeed update pod
			mockParam.its.Status.InstanceStatus = append(mockParam.its.Status.InstanceStatus,
				workloads.InstanceStatus{
					PodName: "pod2",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        mockParam.generateConfigIdentifier(),
							VersionHash: mockParam.getTargetVersionHash(),
						},
					},
				})
			status, err = policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(2)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))
		})
	})
})
