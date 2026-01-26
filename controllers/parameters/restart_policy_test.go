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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var _ = Describe("Reconfigure restartPolicy test", func() {
	const (
		cfgName = "test"
	)

	var (
		policy = &restartPolicy{}
	)

	Context("restart policy", func() {
		It("should success without error", func() {
			configHash := "test-hash"
			mockParam := reconfigureContext{
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
					Replicas: 2,
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
			}

			// first upgrade, no pod is ready
			mockParam.its.Status.InstanceStatus = []workloads.InstanceStatus{}
			status, err := policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(0)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// only one pod ready
			mockParam.its.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "pod1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       mockParam.ConfigTemplate.Name,
							ConfigHash: mockParam.getTargetConfigHash(),
						},
					},
				},
			}
			status, err = policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESRetry))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(1)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))

			// succeed update pod
			mockParam.its.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "pod1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       mockParam.ConfigTemplate.Name,
							ConfigHash: mockParam.getTargetConfigHash(),
						},
					},
				},
				{
					PodName: "pod2",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       mockParam.ConfigTemplate.Name,
							ConfigHash: mockParam.getTargetConfigHash(),
						},
					},
				},
			}
			status, err = policy.Upgrade(mockParam)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(ESNone))
			Expect(status.SucceedCount).Should(BeEquivalentTo(int32(2)))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(int32(2)))
		})
	})
})
