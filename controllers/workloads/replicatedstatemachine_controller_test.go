/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package workloads

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ReplicatedStateMachine Controller", func() {
	Context("reconciliation", func() {
		It("should reconcile well", func() {
			name := "test-stateful-replica-set"
			port := int32(12345)
			service := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "foo",
							Protocol: corev1.ProtocolTCP,
							Port:     port,
						},
					},
				},
			}
			commonLabels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "ClusterDefName",
				constant.AppComponentLabelKey:   "CompDefName",
				constant.AppInstanceLabelKey:    "clusterName",
				constant.KBAppComponentLabelKey: "componentName",
			}
			pod := builder.NewPodBuilder(testCtx.DefaultNamespace, "foo").
				AddLabelsInMap(commonLabels).
				AddContainer(corev1.Container{
					Name:  "foo",
					Image: "bar",
					Ports: []corev1.ContainerPort{
						{
							Name:          "foo",
							Protocol:      corev1.ProtocolTCP,
							ContainerPort: port,
						},
					},
				}).GetObject()
			template := corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			action := workloads.Action{
				Image:   "foo",
				Command: []string{"bar"},
			}
			rsm := builder.NewReplicatedStateMachineBuilder(testCtx.DefaultNamespace, name).
				AddMatchLabelsInMap(commonLabels).
				SetService(service).
				SetTemplate(template).
				AddCustomHandler(action).
				GetObject()
			Expect(k8sClient.Create(ctx, rsm)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(rsm),
				func(g Gomega, set *workloads.ReplicatedStateMachine) {
					g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}),
			).Should(Succeed())
			Expect(k8sClient.Delete(ctx, rsm)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(rsm), &workloads.ReplicatedStateMachine{}, false)).
				Should(Succeed())
		})
	})
})
