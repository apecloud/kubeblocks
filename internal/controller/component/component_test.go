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
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"

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
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.versionCtx empty initContains and containers")
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.Containers = nil
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.InitContainers = nil
			component, err = BuildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component, err = BuildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.Containers)).Should(Equal(3))

			By("new init container in clusterVersion not in clusterDefinition")
			component, err = BuildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[1])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))
		})

		It("should auto fill first component if it's empty", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			clusterTplStr := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name:
spec:
  componentSpecs:
    - name: mysql # user-defined
      componentDefRef: mysql # ref clusterdefinition componentDefs.name
      monitor: false
      replicas: 1
      serviceAccountName: kb-release-name-apecloud-mysql-cluster
      enabledLogs:     ["slow","error"]
      volumeClaimTemplates:
        - name: data # ref clusterdefinition components.containers.volumeMounts.name
          spec:
            storageClassName:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
    - name: etcd
      componentDefRef: etcd # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtctld
      componentDefRef: vtctld # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtconsensus
      componentDefRef: vtconsensus # ref clusterdefinition componentDefs.name
      replicas: 1
    - name: vtgate
      componentDefRef: proxy # ref clusterdefinition componentDefs.name
      replicas: 1
`
			clusterTpl := appsv1alpha1.ClusterTemplate{}
			Expect(yaml.Unmarshal([]byte(clusterTplStr), &clusterTpl)).Should(Succeed())
			By("fill simplified fields")
			r := int32(3)
			cluster.Spec.Replicas = &r
			cluster.Spec.Resources.CPU = resource.MustParse("1000m")
			cluster.Spec.Resources.Memory = resource.MustParse("2Gi")
			cluster.Spec.Storage.Size = resource.MustParse("20Gi")
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			component, err := buildComponent(
				reqCtx,
				nil,
				cluster,
				&clusterTpl,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.Replicas).Should(Equal(*cluster.Spec.Replicas))
			Expect(component.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).Should(Equal(cluster.Spec.Storage.Size))
			component, err = buildComponent(
				reqCtx,
				nil,
				cluster,
				&clusterTpl,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[1],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component.Name).Should(Equal("vtgate"))
			Expect(component.ComponentDef).Should(Equal("proxy"))
		})

		It("build affinity correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			By("fill affinity")
			cluster.Spec.AvailabilityPolicy = appsv1alpha1.AvailabilityPolicyZone
			cluster.Spec.Tenancy = appsv1alpha1.DedicatedNode
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			component, err := buildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.PodSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey).Should(Equal("topology.kubernetes.io/zone"))
			Expect(component.PodSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).Should(Equal("kubernetes.io/hostname"))
		})

		It("build monitor correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			By("enable monitor config in clusterdefinition")
			clusterDef.Spec.ComponentDefs[0].Monitor = &appsv1alpha1.MonitorConfig{
				BuiltIn: true,
			}
			By("fill monitor")
			interval := intstr.Parse("0")
			cluster.Spec.Monitor.MonitoringInterval = &interval
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			component, err := buildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.Monitor.Enable).Should(Equal(false))
			By("set monitor interval to 10s")
			interval2 := intstr.Parse("10s")
			cluster.Spec.Monitor.MonitoringInterval = &interval2
			By("call build")
			component, err = buildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.Monitor.Enable).Should(Equal(true))
		})

		It("build network correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			By("setup cloud provider")
			viper.Set(constant.CfgKeyServerInfo, "v1.26.5-gke.1200")
			By("fill network")
			cluster.Spec.Network = &appsv1alpha1.ClusterNetwork{
				HostNetworkAccessible: true,
				PubliclyAccessible:    false,
			}
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			component, err := buildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.Services[1].Name).Should(Equal("vpc"))
			Expect(component.Services[1].Annotations["networking.gke.io/load-balancer-type"]).Should(Equal("Internal"))
			Expect(component.Services[1].Spec.Type).Should(BeEquivalentTo("LoadBalancer"))
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
								Name: constant.KBConnCredentialPlaceHolder,
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
								Name: constant.KBConnCredentialPlaceHolder,
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

		It("should not fill component if none of simplified api is present", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			component, err := buildComponent(
				reqCtx,
				nil,
				cluster,
				nil,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).Should(BeNil())
		})
	})
})
