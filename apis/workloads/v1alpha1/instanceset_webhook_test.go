/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("InstanceSet Webhook", func() {
	Context("spec validation", func() {
		const name = "test-instance-set"
		var its *InstanceSet

		BeforeEach(func() {
			commonLabels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "ClusterDefName",
				constant.AppComponentLabelKey:   "CompDefName",
				constant.AppInstanceLabelKey:    "clusterName",
				constant.KBAppComponentLabelKey: "componentName",
			}
			replicas := int32(1)
			its = &InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: testCtx.DefaultNamespace,
				},
				Spec: InstanceSetSpec{
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
								Args:    []string{"baz"},
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
			its.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   false,
					AccessMode: ReadWriteMode,
				},
			}
			err := k8sClient.Create(ctx, its)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("leader is required"))
		})

		It("should return an error if servicePort not provided", func() {
			its.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   true,
					AccessMode: ReadWriteMode,
				},
			}
			err := k8sClient.Create(ctx, its)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("servicePort must provide"))
		})

		It("should succeed if spec is well defined", func() {
			its.Spec.Roles = []ReplicaRole{
				{
					Name:       "leader",
					IsLeader:   true,
					AccessMode: ReadWriteMode,
				},
			}
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:     "foo",
					Protocol: "tcp",
					Port:     12345,
				},
			}
			Expect(k8sClient.Create(ctx, its)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, its)).Should(Succeed())
		})
	})
})
