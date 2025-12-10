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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("Reconfigure restartPolicy test", func() {
	var (
		policy = &restartPolicy{}
	)

	Context("restart policy", func() {
		It("should success without error", func() {
			mockParam := newMockReconfigureParams("restartPolicy", k8sClient,
				withConfigSpec("test", map[string]string{
					"key": "value",
				}),
				withClusterComponent(2),
				withWorkload())

			// first upgrade, not pod is ready
			mockParam.its.Status.InstanceStatus = []workloads.InstanceStatus{}
			status, err := policy.Upgrade(mockParam)
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
							Name:        mockParam.ConfigTemplate.Name,
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
							Name:        mockParam.ConfigTemplate.Name,
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
