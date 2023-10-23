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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("ReplicatedStateMachine Webhook", func() {
	Context("spec validation", func() {
		const name = "test-replicated-state-machine"
		var rsm *ReplicatedStateMachine

		BeforeEach(func() {
			commonLabels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "ClusterDefName",
				constant.AppComponentLabelKey:   "CompDefName",
				constant.AppInstanceLabelKey:    "clusterName",
				constant.KBAppComponentLabelKey: "componentName",
			}
			replicas := int32(1)
			rsm = &ReplicatedStateMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: testCtx.DefaultNamespace,
				},
				Spec: ReplicatedStateMachineSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: commonLabels,
					},
					Service: &corev1.Service{},
					RoleProbe: &RoleProbe{
						CustomHandler: []Action{
							{
								Image:   "foo",
								Command: []string{"bar"},
							},
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: commonLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "foo",
									Image: "bar",
								},
							},
						},
					},
				},
			}
		})

		It("should return an error if no leader set", func() {
			rsm.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   false,
					AccessMode: ReadWriteMode,
				},
			}
			err := k8sClient.Create(ctx, rsm)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("leader is required"))
		})

		It("should return an error if servicePort not provided", func() {
			rsm.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   true,
					AccessMode: ReadWriteMode,
				},
			}
			err := k8sClient.Create(ctx, rsm)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("servicePort must provide"))
		})

		It("should succeed if spec is well defined", func() {
			rsm.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   true,
					AccessMode: ReadWriteMode,
				},
			}
			rsm.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:     "foo",
					Protocol: "tcp",
					Port:     12345,
				},
			}
			Expect(k8sClient.Create(ctx, rsm)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, rsm)).Should(Succeed())
		})
	})
})
