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

package reconfigure

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var _ = ginkgo.Describe("Reconfigure syncPolicy test", func() {
	ginkgo.Context("sync reconfigure policy", func() {
		var (
			rctx Context
		)

		ginkgo.BeforeEach(func() {
			configHash := "test-config-hash"
			rctx = Context{
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
				ITS: &workloads.InstanceSet{
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

		ginkgo.It("update cluster spec", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			Expect(*rctx.ClusterComponent.Configs[0].ConfigHash).Should(Equal(*rctx.getTargetConfigHash()))
			Expect(rctx.ClusterComponent.Configs[0].Variables).Should(HaveKeyWithValue("a", "c b e f"))
		})

		ginkgo.It("status replicas - partially updated", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			ginkgo.By("mock the instance status")
			rctx.ITS.Status.InstanceStatus = []workloads.InstanceStatus{
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

			ginkgo.By("status check")
			status, err = syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(1))
		})

		ginkgo.It("status replicas - all", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			ginkgo.By("mock the instance status")
			rctx.ITS.Status.InstanceStatus = []workloads.InstanceStatus{
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

			ginkgo.By("status check")
			status, err = syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusNone))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(3))
		})
	})
})
