/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/apiutil"
)

var _ = Describe("Component Definition Convertor", func() {
	Context("convertors", func() {
		var (
			clusterCompDef *appsv1alpha1.ClusterComponentDefinition

			clusterName = "mysql-test"

			dataVolumeName = "data"
			logVolumeName  = "log"

			runAsUser    = int64(0)
			runAsNonRoot = false
		)

		BeforeEach(func() {
			clusterCompDef = &appsv1alpha1.ClusterComponentDefinition{
				Name: "mysql",
				PodSpec: &corev1.PodSpec{
					Volumes: []corev1.Volume{},
					Containers: []corev1.Container{
						{
							Name:    "mysql",
							Command: []string{"/entrypoint.sh"},
							Env: []corev1.EnvVar{
								{
									Name:  "port",
									Value: "3306",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      dataVolumeName,
									MountPath: "/data/mysql",
								},
								{
									Name:      logVolumeName,
									MountPath: "/data/log",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "mysql",
									ContainerPort: 3306,
								},
								{
									Name:          "paxos",
									ContainerPort: 13306,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:    &runAsUser,
								RunAsNonRoot: &runAsNonRoot,
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/pre-stop.sh"},
									},
								},
							},
						},
					},
				},
			}
		})

		It("provider", func() {
			convertor := &compDefProviderConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeEmpty())
		})

		It("description", func() {
			convertor := &compDefDescriptionConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeEmpty())
		})

		It("service kind", func() {
			convertor := &compDefServiceKindConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeEmpty())
		})

		It("service version", func() {
			convertor := &compDefServiceVersionConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeEmpty())
		})

		Context("runtime", func() {
			It("w/o pod spec", func() {
				clusterCompDefCopy := clusterCompDef.DeepCopy()
				clusterCompDefCopy.PodSpec = nil

				convertor := &compDefRuntimeConvertor{}
				res, err := convertor.convert(clusterCompDefCopy)
				Expect(err).Should(HaveOccurred())
				Expect(res).Should(BeNil())
			})
		})

		Context("vars", func() {
			It("host network ports", func() {
				clusterCompDef.PodSpec.HostNetwork = true
				// default ports are 3306 and 13306
				clusterCompDef.PodSpec.Containers[0].Ports[0].ContainerPort = 36

				convertor := &compDefVarsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).ShouldNot(BeNil())

				vars, ok := res.([]appsv1alpha1.EnvVar)
				Expect(ok).Should(BeTrue())
				Expect(vars).Should(HaveLen(1))

				container := clusterCompDef.PodSpec.Containers[0]
				expectedVar := appsv1alpha1.EnvVar{
					Name: apiutil.HostNetworkDynamicPortVarName(container.Name, container.Ports[0].Name),
					ValueFrom: &appsv1alpha1.VarSource{
						HostNetworkVarRef: &appsv1alpha1.HostNetworkVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Optional: func() *bool { optional := false; return &optional }(),
							},
							HostNetworkVars: appsv1alpha1.HostNetworkVars{
								Container: &appsv1alpha1.ContainerVars{
									Name: container.Name,
									Port: &appsv1alpha1.NamedVar{
										Name:   container.Ports[0].Name,
										Option: &appsv1alpha1.VarRequired,
									},
								},
							},
						},
					},
				}
				Expect(vars[0]).Should(BeEquivalentTo(expectedVar))
			})
		})

		Context("volumes", func() {
			It("ok", func() {
				convertor := &compDefVolumesConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})
		})

		Context("host network", func() {
			It("w/o pod spec", func() {
				clusterCompDef.PodSpec = nil

				convertor := &compDefHostNetworkConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})

			It("host network disabled", func() {
				clusterCompDef.PodSpec.HostNetwork = false

				convertor := &compDefHostNetworkConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})

			It("empty container ports", func() {
				clusterCompDef.PodSpec.HostNetwork = true
				for i := range clusterCompDef.PodSpec.Containers {
					clusterCompDef.PodSpec.Containers[i].Ports = nil
				}

				convertor := &compDefHostNetworkConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).ShouldNot(BeNil())

				hostNetwork, ok := res.(*appsv1alpha1.HostNetwork)
				Expect(ok).Should(BeTrue())
				Expect(hostNetwork.ContainerPorts).Should(HaveLen(0))
			})

			It("no dynamic ports", func() {
				clusterCompDef.PodSpec.HostNetwork = true
				// default ports are 3306 and 13306

				convertor := &compDefHostNetworkConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).ShouldNot(BeNil())

				hostNetwork, ok := res.(*appsv1alpha1.HostNetwork)
				Expect(ok).Should(BeTrue())
				Expect(hostNetwork.ContainerPorts).Should(HaveLen(0))
			})

			It("part dynamic ports", func() {
				clusterCompDef.PodSpec.HostNetwork = true
				// default ports are 3306 and 13306
				container := clusterCompDef.PodSpec.Containers[0]
				clusterCompDef.PodSpec.Containers[0].Ports[0].ContainerPort = 36

				convertor := &compDefHostNetworkConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).ShouldNot(BeNil())

				hostNetwork, ok := res.(*appsv1alpha1.HostNetwork)
				Expect(ok).Should(BeTrue())
				Expect(hostNetwork.ContainerPorts).Should(HaveLen(1))
				Expect(hostNetwork.ContainerPorts[0].Container).Should(Equal(container.Name))
				Expect(hostNetwork.ContainerPorts[0].Ports).Should(HaveLen(1))
				Expect(hostNetwork.ContainerPorts[0].Ports[0]).Should(Equal(container.Ports[0].Name))
			})
		})

		Context("services", func() {
			It("ok", func() {
				convertor := &compDefServicesConvertor{}
				res, err := convertor.convert(clusterCompDef, clusterName)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})
		})

		It("configs", func() {
			convertor := &compDefConfigsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})

		It("scripts", func() {
			convertor := &compDefScriptsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})

		It("log configs", func() {
			convertor := &compDefLogConfigsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})

		It("policy rules", func() {
			convertor := &compDefPolicyRulesConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})

		It("labels", func() {
			convertor := &compDefLabelsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})

		Context("system accounts", func() {
			It("ok", func() {
				convertor := &compDefSystemAccountsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})
		})

		Context("update strategy", func() {
			It("ok", func() {
				convertor := &compDefUpdateStrategyConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})
		})

		Context("roles", func() {
			It("ok", func() {
				convertor := &compDefRolesConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})
		})

		Context("lifecycle actions", func() {
			It("ok", func() {
				convertor := &compDefLifecycleActionsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())

				actions := res.(*appsv1alpha1.ComponentLifecycleActions)
				expectedActions := &appsv1alpha1.ComponentLifecycleActions{}
				Expect(*actions).Should(BeEquivalentTo(*expectedActions))
			})
		})

		It("service ref declarations", func() {
			convertor := &compDefServiceRefDeclarationsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeNil())
		})
	})
})
