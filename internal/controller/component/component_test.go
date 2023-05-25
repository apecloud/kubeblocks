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

package component

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var tlog = ctrl.Log.WithName("component_testing")

var _ = Describe("component module", func() {

	Context("has the BuildComponent function", func() {
		const (
			clusterDefName           = "test-clusterdef"
			clusterVersionName       = "test-clusterversion"
			clusterName              = "test-cluster"
			mysqlCompDefName         = "replicasets"
			mysqlCompName            = "mysql"
			proxyCompDefName         = "proxy"
			mysqlSecretUserEnvName   = "MYSQL_ROOT_USER"
			mysqlSecretPasswdEnvName = "MYSQL_ROOT_PASSWORD"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddComponentDef(testapps.StatelessNginxComponent, proxyCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponentVersion(proxyCompDefName).
				AddInitContainerShort("nginx-init", testapps.NginxImage).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component, err := BuildComponent(
				reqCtx,
				*cluster,
				nil,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.versionCtx empty initContains and containers")
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.Containers = nil
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.InitContainers = nil
			component, err = BuildComponent(
				reqCtx,
				*cluster,
				nil,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component, err = BuildComponent(
				reqCtx,
				*cluster,
				nil,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("new init container in clusterVersion not in clusterDefinition")
			component, err = BuildComponent(
				reqCtx,
				*cluster,
				nil,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))
		})

		It("classDefRef has higher precedence than resources", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}

			By("component without class")
			resources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    testapps.Class2c4g.CPU,
					corev1.ResourceMemory: testapps.Class2c4g.Memory,
				},
			}
			cluster.Spec.ComponentSpecs[0].Resources = resources
			component, err := BuildComponent(
				reqCtx,
				*cluster,
				nil,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(reflect.DeepEqual(component.PodSpec.Containers[0].Resources, resources)).Should(BeTrue())

			By("component with class")
			classes := map[string]map[string]*appsv1alpha1.ComponentClassInstance{
				cluster.Spec.ComponentSpecs[0].ComponentDefRef: {
					testapps.Class1c1gName: &appsv1alpha1.ComponentClassInstance{
						ComponentClass: testapps.Class1c1g,
					},
				},
			}
			cluster.Spec.ComponentSpecs[0].ClassDefRef = &appsv1alpha1.ClassDefRef{Class: testapps.Class1c1gName}
			component, err = BuildComponent(
				reqCtx,
				*cluster,
				classes,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			container := component.PodSpec.Containers[0]
			Expect(err).Should(Succeed())
			Expect(container.Resources.Requests.Cpu().Equal(testapps.Class1c1g.CPU)).Should(BeTrue())
			Expect(container.Resources.Limits.Cpu().Equal(testapps.Class1c1g.CPU)).Should(BeTrue())
			Expect(container.Resources.Requests.Memory().Equal(testapps.Class1c1g.Memory)).Should(BeTrue())
			Expect(container.Resources.Limits.Memory().Equal(testapps.Class1c1g.Memory)).Should(BeTrue())
		})

		It("Test replace secretRef env placeholder token", func() {
			By("mock connect credential and do replace placeholder token")
			credentialMap := GetEnvReplacementMapForConnCredential(cluster.Name)
			mockEnvs := []corev1.EnvVar{
				{
					Name: mysqlSecretUserEnvName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "username",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constant.ConnCredentialPlaceHolder,
							},
						},
					},
				},
				{
					Name: mysqlSecretPasswdEnvName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "password",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constant.ConnCredentialPlaceHolder,
							},
						},
					},
				},
			}
			mockEnvs = ReplaceSecretEnvVars(credentialMap, mockEnvs)
			Expect(len(mockEnvs)).Should(Equal(2))
			for _, env := range mockEnvs {
				Expect(env.ValueFrom).ShouldNot(BeNil())
				Expect(env.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
				Expect(env.ValueFrom.SecretKeyRef.Name).Should(Equal(fmt.Sprintf("%s-conn-credential", cluster.Name)))
			}
		})
	})
})
