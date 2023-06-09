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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("StatefulReplicaSet Controller", func() {
	Context("reconciliation", func() {
		It("should reconcile well", func() {
			name := "test-stateful-replica-set"
			port := int32(12345)
			service := corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "foo",
						Protocol: corev1.ProtocolTCP,
						Port:     port,
					},
				},
			}
			pod := builder.NewPodBuilder(testCtx.DefaultNamespace, "foo").
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
			csSet := builder.NewStatefulReplicaSetBuilder(testCtx.DefaultNamespace, name).
				SetService(service).
				SetTemplate(template).
				AddObservationAction(action).
				GetObject()
			Expect(k8sClient.Create(ctx, csSet)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(csSet),
				func(g Gomega, set *workloads.StatefulReplicaSet) {
					g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}),
			).Should(Succeed())
			Expect(k8sClient.Delete(ctx, csSet)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(csSet), &workloads.StatefulReplicaSet{}, false)).
				Should(Succeed())
		})
	})
})
