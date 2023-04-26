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
			component := BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.versionCtx empty initContains and containers")
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.Containers = nil
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.InitContainers = nil
			component = BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component = BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("new init container in clusterVersion not in clusterDefinition")
			component = BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))
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
