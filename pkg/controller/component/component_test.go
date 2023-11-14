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
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Component", func() {

	Context("has the BuildComponent function", func() {
		const (
			clusterDefName           = "test-clusterdef"
			clusterVersionName       = "test-clusterversion"
			clusterName              = "test-cluster"
			mysqlCompDefName         = "replicasets"
			mysqlCompName            = "mysql"
			proxyCompDefName         = "proxy"
			proxyCompName            = "proxy"
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
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				AddComponent(proxyCompName, proxyCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.versionCtx empty initContains and containers")
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.Containers = nil
			clusterVersion.Spec.ComponentVersions[0].VersionsCtx.InitContainers = nil
			component, err = BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component, err = BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.Containers) >= 3).Should(BeTrue())

			By("new init container in clusterVersion not in clusterDefinition")
			component, err = BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, &cluster.Spec.ComponentSpecs[1])
			Expect(err).Should(Succeed())
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))
		})

		It("should auto fill first component if it's empty", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}

			By("fill simplified fields")
			r := int32(3)
			cluster.Spec.Replicas = &r
			cluster.Spec.Resources.CPU = resource.MustParse("1000m")
			cluster.Spec.Resources.Memory = resource.MustParse("2Gi")
			cluster.Spec.Storage.Size = resource.MustParse("20Gi")

			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil

			By("build first component from simplified fields")
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.Replicas).Should(Equal(*cluster.Spec.Replicas))
			Expect(synthesizeComp.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).Should(Equal(cluster.Spec.Storage.Size))

			// By("build second component will be nil")
			// synthesizeComp, err = BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			// Expect(err).Should(Succeed())
			// Expect(synthesizeComp).Should(BeNil())
		})

		It("build affinity correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			By("fill affinity")
			cluster.Spec.AvailabilityPolicy = appsv1alpha1.AvailabilityPolicyZone
			cluster.Spec.Tenancy = appsv1alpha1.DedicatedNode
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.PodSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey).Should(Equal("topology.kubernetes.io/zone"))
			Expect(synthesizeComp.PodSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).Should(Equal("kubernetes.io/hostname"))
		})

		It("build monitor correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
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
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.Monitor.Enable).Should(Equal(false))
			By("set monitor interval to 10s")
			interval2 := intstr.Parse("10s")
			cluster.Spec.Monitor.MonitoringInterval = &interval2
			By("call build")
			synthesizeComp, err = BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.Monitor.Enable).Should(Equal(true))
		})

		It("build network correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
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
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.Services[1].Name).Should(Equal("vpc"))
			Expect(synthesizeComp.Services[1].Annotations["networking.gke.io/load-balancer-type"]).Should(Equal("Internal"))
			Expect(synthesizeComp.Services[1].Spec.Type).Should(BeEquivalentTo("LoadBalancer"))
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
				Expect(env.ValueFrom.SecretKeyRef.Name).Should(Equal(constant.GenerateDefaultConnCredential(cluster.Name)))
			}
		})

		It("should not fill component if none of simplified api is present", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			By("clear cluster's component spec")
			cluster.Spec.ComponentSpecs = nil
			By("call build")
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, nil)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).Should(BeNil())
		})

		PIt("build serviceReference correctly", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			const (
				name    = "nginx"
				ns      = "default"
				kind    = "mock-kind"
				version = "mock-version"
			)
			By("generate serviceReference")
			serviceDescriptor := &appsv1alpha1.ServiceDescriptor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
				},
				Spec: appsv1alpha1.ServiceDescriptorSpec{
					ServiceKind:    kind,
					ServiceVersion: version,
				},
			}
			serviceReferenceMap := map[string]*appsv1alpha1.ServiceDescriptor{
				testapps.NginxImage: serviceDescriptor,
			}
			By("call build")
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli, clusterDef, clusterVersion, cluster, &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.ServiceReferences).ShouldNot(BeNil())
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Name).Should(Equal(name))
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Spec.ServiceKind).Should(Equal(kind))
			Expect(synthesizeComp.ServiceReferences[testapps.NginxImage].Spec.ServiceVersion).Should(Equal(version))
			Expect(serviceReferenceMap).Should(BeEmpty()) // for test failed
		})
	})
})

func TestGetConfigSpecByName(t *testing.T) {
	type args struct {
		component  *SynthesizedComponent
		configSpec string
	}
	tests := []struct {
		name string
		args args
		want *appsv1alpha1.ComponentConfigSpec
	}{{
		name: "test",
		args: args{
			component:  &SynthesizedComponent{},
			configSpec: "for_test",
		},
		want: nil,
	}, {
		name: "test",
		args: args{
			component: &SynthesizedComponent{
				ConfigTemplates: []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name: "test",
					}}},
			},
			configSpec: "for-test",
		},
		want: nil,
	}, {
		name: "test",
		args: args{
			component: &SynthesizedComponent{
				ConfigTemplates: []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name: "for-test",
					}}},
			},
			configSpec: "for-test",
		},
		want: &appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name: "for-test",
			}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetConfigSpecByName(tt.args.component, tt.args.configSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConfigSpecByName() = %v, want %v", got, tt.want)
			}
		})
	}
}
