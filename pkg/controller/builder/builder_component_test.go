/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("component builder", func() {
	It("should set component spec fields", func() {
		podUpdatePolicy := appsv1.PreferInPlacePodUpdatePolicyType
		podUpgradePolicy := appsv1.ReCreatePodUpdatePolicyType
		instanceUpdateStrategy := &appsv1.InstanceUpdateStrategy{
			Type: appsv1.OnDeleteStrategyType,
		}
		concurrency := intstr.FromString("50%")
		issuer := &appsv1.Issuer{Name: appsv1.IssuerKubeBlocks}
		disableExporter := true
		stop := true
		enableInstanceAPI := true
		runtimeClassName := "kata"
		resources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
			},
		}
		retentionPolicy := &appsv1.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
		}
		schedulingPolicy := &appsv1.SchedulingPolicy{
			SchedulerName: "custom-scheduler",
			NodeSelector:  map[string]string{"disk": "ssd"},
		}
		network := &appsv1.ComponentNetwork{
			HostNetwork: true,
		}
		configName := "config"

		obj := NewComponentBuilder("default", "mysql", "mysql-def").
			SetTerminationPolicy(appsv1.Delete).
			SetServiceVersion("8.0.30").
			SetLabels(map[string]string{"app": "mysql"}).
			SetAnnotations(map[string]string{"owner": "test"}).
			SetEnv([]corev1.EnvVar{{Name: "MYSQL_ROOT_HOST", Value: "%"}}).
			SetSchedulingPolicy(schedulingPolicy).
			SetReplicas(3).
			SetConfigs([]appsv1.ClusterComponentConfig{{Name: &configName}}).
			SetServiceAccountName("mysql-sa").
			SetParallelPodManagementConcurrency(&concurrency).
			SetPodUpdatePolicy(&podUpdatePolicy).
			SetPodUpgradePolicy(&podUpgradePolicy).
			SetInstanceUpdateStrategy(instanceUpdateStrategy).
			SetResources(resources).
			SetDisableExporter(&disableExporter).
			SetTLSConfig(true, issuer).
			SetVolumeClaimTemplates([]appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}).
			SetPVCRetentionPolicy(retentionPolicy).
			SetVolumes([]corev1.Volume{{Name: "scripts"}}).
			SetNetwork(network).
			SetServices([]appsv1.ClusterComponentService{{
				Name:        "primary",
				ServiceType: corev1.ServiceTypeClusterIP,
				Annotations: map[string]string{
					"service": "primary",
				},
				PodService: ptr.To(true),
			}}).
			SetSystemAccounts([]appsv1.ComponentSystemAccount{{Name: "root"}}).
			SetServiceRefs([]appsv1.ServiceRef{{Name: "redis"}}).
			SetInstances([]appsv1.InstanceTemplate{{Name: "az-a"}}).
			SetOrdinals(appsv1.Ordinals{Ranges: []appsv1.Range{{Start: 1, End: 3}}}).
			SetFlatInstanceOrdinal(true).
			SetOfflineInstances([]string{"mysql-1"}).
			SetRuntimeClassName(&runtimeClassName).
			SetStop(&stop).
			SetSidecars([]appsv1.Sidecar{{Name: "metrics", Owner: "mysql", SidecarDef: "metrics-def"}}).
			SetEnableInstanceAPI(&enableInstanceAPI).
			GetObject()

		Expect(obj.Namespace).Should(Equal("default"))
		Expect(obj.Name).Should(Equal("mysql"))
		Expect(obj.Spec.CompDef).Should(Equal("mysql-def"))
		Expect(obj.Spec.TerminationPolicy).Should(Equal(appsv1.Delete))
		Expect(obj.Spec.ServiceVersion).Should(Equal("8.0.30"))
		Expect(obj.Spec.Labels).Should(Equal(map[string]string{"app": "mysql"}))
		Expect(obj.Spec.Annotations).Should(Equal(map[string]string{"owner": "test"}))
		Expect(obj.Spec.Env).Should(Equal([]corev1.EnvVar{{Name: "MYSQL_ROOT_HOST", Value: "%"}}))
		Expect(obj.Spec.SchedulingPolicy).Should(Equal(schedulingPolicy))
		Expect(obj.Spec.Replicas).Should(Equal(int32(3)))
		Expect(obj.Spec.Configs).Should(Equal([]appsv1.ClusterComponentConfig{{Name: &configName}}))
		Expect(obj.Spec.ServiceAccountName).Should(Equal("mysql-sa"))
		Expect(obj.Spec.ParallelPodManagementConcurrency).Should(Equal(&concurrency))
		Expect(obj.Spec.PodUpdatePolicy).Should(Equal(&podUpdatePolicy))
		Expect(obj.Spec.PodUpgradePolicy).Should(Equal(&podUpgradePolicy))
		Expect(obj.Spec.InstanceUpdateStrategy).Should(Equal(instanceUpdateStrategy))
		Expect(obj.Spec.Resources).Should(Equal(resources))
		Expect(obj.Spec.DisableExporter).Should(Equal(&disableExporter))
		Expect(obj.Spec.TLSConfig).Should(Equal(&appsv1.TLSConfig{Enable: true, Issuer: issuer}))
		Expect(obj.Spec.VolumeClaimTemplates).Should(Equal([]appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}))
		Expect(obj.Spec.PersistentVolumeClaimRetentionPolicy).Should(Equal(retentionPolicy))
		Expect(obj.Spec.Volumes).Should(Equal([]corev1.Volume{{Name: "scripts"}}))
		Expect(obj.Spec.Network).Should(Equal(network))
		Expect(obj.Spec.Services).Should(Equal([]appsv1.ComponentService{{
			Service: appsv1.Service{
				Name: "primary",
				Annotations: map[string]string{
					"service": "primary",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
			},
			PodService: ptr.To(true),
		}}))
		Expect(obj.Spec.SystemAccounts).Should(Equal([]appsv1.ComponentSystemAccount{{Name: "root"}}))
		Expect(obj.Spec.ServiceRefs).Should(Equal([]appsv1.ServiceRef{{Name: "redis"}}))
		Expect(obj.Spec.Instances).Should(Equal([]appsv1.InstanceTemplate{{Name: "az-a"}}))
		Expect(obj.Spec.Ordinals).Should(Equal(appsv1.Ordinals{Ranges: []appsv1.Range{{Start: 1, End: 3}}}))
		Expect(obj.Spec.FlatInstanceOrdinal).Should(BeTrue())
		Expect(obj.Spec.OfflineInstances).Should(Equal([]string{"mysql-1"}))
		Expect(obj.Spec.RuntimeClassName).Should(Equal(&runtimeClassName))
		Expect(obj.Spec.Stop).Should(Equal(&stop))
		Expect(obj.Spec.Sidecars).Should(Equal([]appsv1.Sidecar{{Name: "metrics", Owner: "mysql", SidecarDef: "metrics-def"}}))
		Expect(obj.Spec.EnableInstanceAPI).Should(Equal(&enableInstanceAPI))
	})

	It("should leave TLS config and runtime class unset when disabled", func() {
		obj := NewComponentBuilder("default", "mysql", "mysql-def").
			SetTLSConfig(false, &appsv1.Issuer{Name: appsv1.IssuerKubeBlocks}).
			SetRuntimeClassName(nil).
			GetObject()

		Expect(obj.Spec.TLSConfig).Should(BeNil())
		Expect(obj.Spec.RuntimeClassName).Should(BeNil())
	})
})
