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
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/apiutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component Definition Convertor", func() {
	Context("convertors", func() {
		var (
			clusterCompDef *appsv1alpha1.ClusterComponentDefinition

			clusterName = "mysql-test"

			defaultHighWatermark = 90
			lowerHighWatermark   = 85
			dataVolumeName       = "data"
			logVolumeName        = "log"

			defaultVolumeMode = int32(0555)

			runAsUser    = int64(0)
			runAsNonRoot = false
		)

		commandExecutorEnvItem := &appsv1alpha1.CommandExecutorEnvItem{
			Image: testapps.ApeCloudMySQLImage,
			Env: []corev1.EnvVar{
				{
					Name: "user",
				},
			},
		}
		commandExecutorItem := &appsv1alpha1.CommandExecutorItem{
			Command: []string{"foo", "bar"},
			Args:    []string{"zoo", "boo"},
		}

		BeforeEach(func() {
			clusterCompDef = &appsv1alpha1.ClusterComponentDefinition{
				Name:          "mysql",
				Description:   "component definition convertor",
				CharacterType: "mysql",
				ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{
					{
						ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
							Name:        "mysql-config",
							TemplateRef: "mysql-config-template",
							VolumeName:  "mysql-config",
							DefaultMode: &defaultVolumeMode,
						},
						ConfigConstraintRef: "mysql-config-constraints",
					},
				},
				ScriptSpecs: []appsv1alpha1.ComponentTemplateSpec{
					{
						Name:        "mysql-scripts",
						TemplateRef: "mysql-scripts",
						VolumeName:  "scripts",
						DefaultMode: &defaultVolumeMode,
					},
				},
				LogConfigs: []appsv1alpha1.LogConfig{
					{
						Name:            "error",
						FilePathPattern: "/data/mysql/log/mysqld-error.log",
					},
					{
						Name:            "slow",
						FilePathPattern: "/data/mysql/log/mysqld-slowquery.log",
					},
					{
						Name:            "general",
						FilePathPattern: "/data/mysql/log/mysqld.log",
					},
				},
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
				Service: &appsv1alpha1.ServiceSpec{
					Ports: []appsv1alpha1.ServicePort{
						{
							Name: "mysql",
							Port: 3306,
							TargetPort: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "mysql",
							},
						},
						{
							Name: "paxos",
							Port: 13306,
							TargetPort: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: "paxos",
							},
						},
					},
				},
				HorizontalScalePolicy: &appsv1alpha1.HorizontalScalePolicy{},
				SystemAccounts: &appsv1alpha1.SystemAccountSpec{
					CmdExecutorConfig: &appsv1alpha1.CmdExecutorConfig{
						CommandExecutorEnvItem: appsv1alpha1.CommandExecutorEnvItem{
							Image: "image",
							Env: []corev1.EnvVar{
								{
									Name:  "user",
									Value: "user",
								},
								{
									Name:  "password",
									Value: "password",
								},
							},
						},
						CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
							Command: []string{"mysql"},
							Args:    []string{"create user"},
						},
					},
					PasswordConfig: appsv1alpha1.PasswordConfig{
						Length:     16,
						NumDigits:  8,
						NumSymbols: 8,
						LetterCase: appsv1alpha1.MixedCases,
					},
					Accounts: []appsv1alpha1.SystemAccountConfig{
						{
							Name: appsv1alpha1.AdminAccount,
							ProvisionPolicy: appsv1alpha1.ProvisionPolicy{
								Type:  appsv1alpha1.CreateByStmt,
								Scope: appsv1alpha1.AnyPods,
								Statements: &appsv1alpha1.ProvisionStatements{
									CreationStatement: "creation-statement",
								},
							},
						},
						{
							Name: appsv1alpha1.ReplicatorAccount,
							ProvisionPolicy: appsv1alpha1.ProvisionPolicy{
								Type: appsv1alpha1.ReferToExisting,
								SecretRef: &appsv1alpha1.ProvisionSecretRef{
									Name:      "refer-to-existing",
									Namespace: "default",
								},
							},
						},
					},
				},
				VolumeTypes: []appsv1alpha1.VolumeTypeSpec{
					{
						Name: dataVolumeName,
						Type: appsv1alpha1.VolumeTypeData,
					},
					{
						Name: logVolumeName,
						Type: appsv1alpha1.VolumeTypeLog,
					},
				},
				CustomLabelSpecs: []appsv1alpha1.CustomLabelSpec{
					{
						Key:   "scope",
						Value: "scope",
						Resources: []appsv1alpha1.GVKResource{
							{
								GVK: "v1/pod",
								Selector: map[string]string{
									"managed-by": "kubeblocks",
								},
							},
						},
					},
				},
				VolumeProtectionSpec: &appsv1alpha1.VolumeProtectionSpec{
					HighWatermark: defaultHighWatermark,
					Volumes: []appsv1alpha1.ProtectedVolume{
						{
							Name:          logVolumeName,
							HighWatermark: &lowerHighWatermark,
						},
					},
				},
				ComponentDefRef:        []appsv1alpha1.ComponentDefRef{},
				ServiceRefDeclarations: []appsv1alpha1.ServiceRefDeclaration{},
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
			Expect(res).Should(Equal(clusterCompDef.Description))
		})

		It("service kind", func() {
			convertor := &compDefServiceKindConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(Equal(clusterCompDef.CharacterType))
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

			It("w/o comp version", func() {
				convertor := &compDefRuntimeConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeEquivalentTo(*clusterCompDef.PodSpec))
			})

			It("w/ comp version", func() {
				clusterCompVer := &appsv1alpha1.ClusterComponentVersion{
					VersionsCtx: appsv1alpha1.VersionsContext{
						InitContainers: []corev1.Container{
							{
								Name:  "init",
								Image: "init",
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "mysql",
								Image: "image",
							},
						},
					},
				}

				convertor := &compDefRuntimeConvertor{}
				res, err := convertor.convert(clusterCompDef, clusterCompVer)
				Expect(err).Should(Succeed())

				expectedPodSpec := clusterCompDef.PodSpec
				Expect(expectedPodSpec.Containers[0].Image).Should(BeEmpty())
				Expect(expectedPodSpec.InitContainers).Should(HaveLen(0))
				expectedPodSpec.Containers[0].Image = clusterCompVer.VersionsCtx.Containers[0].Image
				expectedPodSpec.InitContainers = clusterCompVer.VersionsCtx.InitContainers
				Expect(res).Should(BeEquivalentTo(*expectedPodSpec))
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
			It("w/o volume types", func() {
				clusterCompDefCopy := clusterCompDef.DeepCopy()
				clusterCompDefCopy.VolumeTypes = nil

				convertor := &compDefVolumesConvertor{}
				res, err := convertor.convert(clusterCompDefCopy)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})

			It("w/o volume protection", func() {
				clusterCompDefCopy := clusterCompDef.DeepCopy()
				clusterCompDefCopy.VolumeProtectionSpec = nil

				convertor := &compDefVolumesConvertor{}
				res, err := convertor.convert(clusterCompDefCopy)
				Expect(err).Should(Succeed())

				expectedVolumes := make([]appsv1alpha1.ComponentVolume, 0)
				for _, vol := range clusterCompDef.VolumeTypes {
					expectedVolumes = append(expectedVolumes, appsv1alpha1.ComponentVolume{Name: vol.Name})
				}
				Expect(res).Should(BeEquivalentTo(expectedVolumes))
			})

			It("ok", func() {
				convertor := &compDefVolumesConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())

				expectedVolumes := make([]appsv1alpha1.ComponentVolume, 0)
				for _, vol := range clusterCompDef.VolumeTypes {
					highWatermark := 0
					if vol.Name == logVolumeName {
						highWatermark = lowerHighWatermark
					}
					expectedVolumes = append(expectedVolumes, appsv1alpha1.ComponentVolume{
						Name:          vol.Name,
						HighWatermark: highWatermark,
					})
				}
				Expect(res).Should(BeEquivalentTo(expectedVolumes))
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
			It("w/o service defined", func() {
				clusterCompDef.Service = nil

				convertor := &compDefServicesConvertor{}
				res, err := convertor.convert(clusterCompDef, clusterName)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})

			It("ok", func() {
				convertor := &compDefServicesConvertor{}
				res, err := convertor.convert(clusterCompDef, clusterName)
				Expect(err).Should(Succeed())

				services, ok := res.([]appsv1alpha1.ComponentService)
				Expect(ok).Should(BeTrue())
				Expect(services).Should(HaveLen(1))

				// service
				Expect(services[0].ServiceName).Should(BeEmpty())
				Expect(services[0].Spec.Ports).Should(HaveLen(len(clusterCompDef.Service.Ports)))
				for i := range services[0].Spec.Ports {
					Expect(services[0].Spec.Ports[i].Name).Should(Equal(clusterCompDef.Service.Ports[i].Name))
					Expect(services[0].Spec.Ports[i].Port).Should(Equal(clusterCompDef.Service.Ports[i].Port))
					Expect(services[0].Spec.Ports[i].TargetPort).Should(Equal(clusterCompDef.Service.Ports[i].TargetPort))
				}
				Expect(services[0].Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))
				Expect(services[0].Spec.ClusterIP).Should(BeEmpty())
				Expect(services[0].RoleSelector).Should(BeEmpty())
			})
		})

		Context("configs", func() {
			It("w/o comp version", func() {
				convertor := &compDefConfigsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeEquivalentTo(clusterCompDef.ConfigSpecs))
			})

			It("w/ comp version", func() {
				clusterCompVer := &appsv1alpha1.ClusterComponentVersion{
					ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{
						{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "agamotto-config",
								TemplateRef: "agamotto-config-template",
								VolumeName:  "agamotto-config",
								DefaultMode: &defaultVolumeMode,
							},
						},
					},
				}

				convertor := &compDefConfigsConvertor{}
				res, err := convertor.convert(clusterCompDef, clusterCompVer)
				Expect(err).Should(Succeed())

				expectedConfigs := make([]appsv1alpha1.ComponentConfigSpec, 0)
				expectedConfigs = append(expectedConfigs, clusterCompVer.ConfigSpecs...)
				expectedConfigs = append(expectedConfigs, clusterCompDef.ConfigSpecs...)
				Expect(res).Should(BeEquivalentTo(expectedConfigs))
			})
		})

		It("log configs", func() {
			convertor := &compDefLogConfigsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())

			logConfigs := res.([]appsv1alpha1.LogConfig)
			Expect(logConfigs).Should(BeEquivalentTo(clusterCompDef.LogConfigs))
		})

		It("scripts", func() {
			convertor := &compDefScriptsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())

			scripts := res.([]appsv1alpha1.ComponentTemplateSpec)
			Expect(scripts).Should(BeEquivalentTo(clusterCompDef.ScriptSpecs))
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

			labels := res.(map[string]string)
			expectedLabels := map[string]string{}
			for _, item := range clusterCompDef.CustomLabelSpecs {
				expectedLabels[item.Key] = item.Value
			}
			Expect(labels).Should(BeEquivalentTo(expectedLabels))
		})

		Context("system accounts", func() {
			It("w/o accounts", func() {
				clusterCompDef.SystemAccounts = nil

				convertor := &compDefSystemAccountsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())
				Expect(res).Should(BeNil())
			})

			It("w/ accounts", func() {
				convertor := &compDefSystemAccountsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())

				expectedAccounts := []appsv1alpha1.SystemAccount{
					{
						Name:                     string(clusterCompDef.SystemAccounts.Accounts[0].Name),
						PasswordGenerationPolicy: clusterCompDef.SystemAccounts.PasswordConfig,
						Statement:                clusterCompDef.SystemAccounts.Accounts[0].ProvisionPolicy.Statements.CreationStatement,
					},
					{
						Name:                     string(clusterCompDef.SystemAccounts.Accounts[1].Name),
						PasswordGenerationPolicy: clusterCompDef.SystemAccounts.PasswordConfig,
						SecretRef:                clusterCompDef.SystemAccounts.Accounts[1].ProvisionPolicy.SecretRef,
					},
				}
				Expect(res).Should(BeEquivalentTo(expectedAccounts))
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
			It("nil", func() {
				convertor := &compDefLifecycleActionsConvertor{}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())

				actions := res.(*appsv1alpha1.ComponentLifecycleActions)
				expectedActions := &appsv1alpha1.ComponentLifecycleActions{}
				Expect(*actions).Should(BeEquivalentTo(*expectedActions))
			})

			It("post provision", func() {
				convertor := &compDefLifecycleActionsConvertor{}
				clusterCompDef.PostStartSpec = &appsv1alpha1.PostStartAction{
					CmdExecutorConfig: appsv1alpha1.CmdExecutorConfig{
						CommandExecutorEnvItem: *commandExecutorEnvItem,
						CommandExecutorItem:    *commandExecutorItem,
					},
					ScriptSpecSelectors: []appsv1alpha1.ScriptSpecSelector{
						{
							Name: "post-start",
						},
					},
				}
				res, err := convertor.convert(clusterCompDef)
				Expect(err).Should(Succeed())

				actions := res.(*appsv1alpha1.ComponentLifecycleActions)
				Expect(actions.PostProvision).ShouldNot(BeNil())
				Expect(actions.PostProvision.CustomHandler).ShouldNot(BeNil())
				Expect(actions.PostProvision.CustomHandler.Image).Should(BeEquivalentTo(commandExecutorEnvItem.Image))
				Expect(actions.PostProvision.CustomHandler.Env).Should(BeEquivalentTo(commandExecutorEnvItem.Env))
				Expect(actions.PostProvision.CustomHandler.Exec.Command).Should(BeEquivalentTo(commandExecutorItem.Command))
				Expect(actions.PostProvision.CustomHandler.Exec.Args).Should(BeEquivalentTo(commandExecutorItem.Args))
				Expect(*actions.PostProvision.CustomHandler.PreCondition).Should(BeEquivalentTo(appsv1alpha1.ComponentReadyPreConditionType))
			})
		})

		It("service ref declarations", func() {
			convertor := &compDefServiceRefDeclarationsConvertor{}
			res, err := convertor.convert(clusterCompDef)
			Expect(err).Should(Succeed())
			Expect(res).Should(BeEquivalentTo(clusterCompDef.ServiceRefDeclarations))
		})
	})
})
