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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var _ = Describe("Reconfigure syncPolicy test", func() {
	Context("sync reconfigure policy", func() {
		const (
			cfgName = "for-test"
		)

		var (
			rctx   reconfigureContext
			policy = &syncPolicy{}
		)

		BeforeEach(func() {
			configHash := "test-config-hash"
			rctx = reconfigureContext{
				RequestCtx: intctrlutil.RequestCtx{
					Ctx: context.Background(),
					Log: log.FromContext(context.Background()),
				},
				Client: nil,
				ConfigTemplate: appsv1.ComponentFileTemplate{
					Name: cfgName,
				},
				ConfigHash: &configHash,
				Cluster: &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
				},
				ClusterComponent: &appsv1.ClusterComponentSpec{
					Name:     "test-component",
					Replicas: 3,
					Configs: []appsv1.ClusterComponentConfig{
						{
							Name: ptr.To(cfgName),
						},
					},
				},
				SynthesizedComponent: &component.SynthesizedComponent{
					Name: "test-component",
				},
				its: &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-instanceset",
						Namespace: "default",
					},
				},
				ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
					Name: cfgName,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.RedisCfg,
					},
				},
				ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
					MergeReloadAndRestart:           ptr.To(false),
					ReloadStaticParamsBeforeRestart: ptr.To(true),
				},
				Patch: &core.ConfigPatchInfo{
					IsModify: true,
					UpdateConfig: map[string][]byte{
						cfgName: []byte(`{"a":"c b e f"}`),
					},
				},
			}
		})

		It("update cluster spec", func() {
			By("update cluster spec")
			status, err := policy.Upgrade(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			Expect(*rctx.ClusterComponent.Configs[0].ConfigHash).Should(Equal(*rctx.getTargetConfigHash()))
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
					PodName: "pod-0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
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
					PodName: "pod-0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
				{
					PodName: "pod-1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
				{
					PodName: "pod-2",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
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
