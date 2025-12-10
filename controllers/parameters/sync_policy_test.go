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

	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	apisappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var _ = Describe("Reconfigure syncPolicy test", func() {
	Context("sync reconfigure policy", func() {
		var (
			rctx   reconfigureContext
			policy = &syncPolicy{}
		)

		BeforeEach(func() {
			By("prepare reconfigure policy params")
			rctx = newMockReconfigureParams("operatorSyncPolicy", k8sClient,
				withConfigSpec("for_test", map[string]string{"a": "c b e f"}),
				withConfigDescription(&parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.RedisCfg}),
				withUpdatedParameters(&core.ConfigPatchInfo{
					IsModify: true,
					UpdateConfig: map[string][]byte{
						"for-test": []byte(`{"a":"c b e f"}`),
					},
				}),
				withParamDef(&parametersv1alpha1.ParametersDefinitionSpec{
					MergeReloadAndRestart:           pointer.Bool(false),
					ReloadStaticParamsBeforeRestart: pointer.Bool(true),
				}),
				withClusterComponentNConfigs(3, []apisappsv1.ClusterComponentConfig{
					{
						Name: ptr.To("for_test"),
					},
				}),
				withWorkload())
		})

		It("update cluster spec", func() {
			By("update cluster spec")
			status, err := policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			Expect(rctx.ClusterComponent.Configs[0].VersionHash).Should(Equal(rctx.getTargetVersionHash()))
			Expect(rctx.ClusterComponent.Configs[0].Variables).Should(HaveKeyWithValue("a", "c b e f"))
		})

		It("status replicas - partially updated", func() {
			By("update cluster spec")
			status, err := policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			By("mock the instance status")
			rctx.its.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        rctx.ConfigTemplate.Name,
							VersionHash: rctx.getTargetVersionHash(),
						},
					},
				},
			}

			By("status check")
			status, err = policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(1))
		})

		It("status replicas - all", func() {
			By("update cluster spec")
			status, err := policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			By("mock the instance status")
			rctx.its.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        rctx.ConfigTemplate.Name,
							VersionHash: rctx.getTargetVersionHash(),
						},
					},
				},
				{
					PodName: "1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        rctx.ConfigTemplate.Name,
							VersionHash: rctx.getTargetVersionHash(),
						},
					},
				},
				{
					PodName: "2",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:        rctx.ConfigTemplate.Name,
							VersionHash: rctx.getTargetVersionHash(),
						},
					},
				},
			}

			By("status check")
			status, err = policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(3))
		})
	})
})
