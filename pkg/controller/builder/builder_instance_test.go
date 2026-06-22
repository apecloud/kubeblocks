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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("instance builder", func() {
	It("should set instance spec fields", func() {
		activeDeadlineSeconds := int64(60)
		selectorLabels := map[string]string{"app": "mysql"}
		affinity := &corev1.Affinity{}
		topologySpreadConstraints := []corev1.TopologySpreadConstraint{{TopologyKey: "zone"}}
		securityContext := corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}
		retentionPolicy := &workloads.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
		}
		instanceUpdateStrategy := &workloads.InstanceUpdateStrategy{
			Type: appsv1.OnDeleteStrategyType,
		}
		lifecycleActions := &workloads.LifecycleActions{}
		assistantObjects := []workloads.InstanceAssistantObject{
			{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "conf"}}},
		}

		obj := NewInstanceBuilder("default", "mysql-0").
			AddFinalizers([]string{"cleanup"}).
			SetFinalizers().
			SetPodTemplate(corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"template": "base"},
				},
			}).
			SetContainers([]corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}).
			SetInitContainers([]corev1.Container{{Name: "init"}}).
			SetNodeName(types.NodeName("node-1")).
			SetHostname("mysql-0").
			SetSubdomain("mysql-headless").
			AddInitContainer(corev1.Container{Name: "prepare"}).
			AddContainer(corev1.Container{Name: "metrics", Image: "exporter:1.0"}).
			AddVolumes(corev1.Volume{Name: "data"}, corev1.Volume{Name: "scripts"}).
			SetRestartPolicy(corev1.RestartPolicyAlways).
			SetSecurityContext(securityContext).
			AddTolerations(corev1.Toleration{Key: "dedicated", Value: "db"}).
			AddServiceAccount("mysql-sa").
			SetNodeSelector(map[string]string{"disk": "ssd"}).
			SetAffinity(affinity).
			SetTopologySpreadConstraints(topologySpreadConstraints).
			SetActiveDeadlineSeconds(&activeDeadlineSeconds).
			SetImagePullSecrets([]corev1.LocalObjectReference{{Name: "registry"}}).
			SetSelectorMatchLabels(selectorLabels).
			SetMinReadySeconds(10).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}).
			SetPVCRetentionPolicy(retentionPolicy).
			SetInstanceSetName("mysql").
			SetInstanceTemplateName("az-a").
			SetInstanceUpdateStrategyType(instanceUpdateStrategy).
			SetPodUpdatePolicy(appsv1.PreferInPlacePodUpdatePolicyType).
			SetPodUpgradePolicy(appsv1.ReCreatePodUpdatePolicyType).
			SetRoles([]workloads.ReplicaRole{{Name: "leader", UpdatePriority: 10}}).
			SetLifecycleActions(lifecycleActions).
			SetConfigs([]workloads.ConfigTemplate{{Name: "mysql-conf"}}).
			SetInstanceAssistantObjects(assistantObjects).
			GetObject()

		selectorLabels["app"] = "changed"

		Expect(obj.Namespace).Should(Equal("default"))
		Expect(obj.Name).Should(Equal("mysql-0"))
		Expect(obj.Finalizers).Should(BeNil())
		Expect(obj.Spec.Template.Labels).Should(Equal(map[string]string{"template": "base"}))
		Expect(obj.Spec.Template.Spec.Containers).Should(Equal([]corev1.Container{
			{Name: "mysql", Image: "mysql:8.0"},
			{Name: "metrics", Image: "exporter:1.0"},
		}))
		Expect(obj.Spec.Template.Spec.InitContainers).Should(Equal([]corev1.Container{{Name: "init"}, {Name: "prepare"}}))
		Expect(obj.Spec.Template.Spec.NodeName).Should(Equal("node-1"))
		Expect(obj.Spec.Template.Spec.Hostname).Should(Equal("mysql-0"))
		Expect(obj.Spec.Template.Spec.Subdomain).Should(Equal("mysql-headless"))
		Expect(obj.Spec.Template.Spec.Volumes).Should(Equal([]corev1.Volume{{Name: "data"}, {Name: "scripts"}}))
		Expect(obj.Spec.Template.Spec.RestartPolicy).Should(Equal(corev1.RestartPolicyAlways))
		Expect(obj.Spec.Template.Spec.SecurityContext).Should(Equal(&securityContext))
		Expect(obj.Spec.Template.Spec.Tolerations).Should(Equal([]corev1.Toleration{{Key: "dedicated", Value: "db"}}))
		Expect(obj.Spec.Template.Spec.ServiceAccountName).Should(Equal("mysql-sa"))
		Expect(obj.Spec.Template.Spec.NodeSelector).Should(Equal(map[string]string{"disk": "ssd"}))
		Expect(obj.Spec.Template.Spec.Affinity).Should(Equal(affinity))
		Expect(obj.Spec.Template.Spec.TopologySpreadConstraints).Should(Equal(topologySpreadConstraints))
		Expect(obj.Spec.Template.Spec.ActiveDeadlineSeconds).Should(Equal(&activeDeadlineSeconds))
		Expect(obj.Spec.Template.Spec.ImagePullSecrets).Should(Equal([]corev1.LocalObjectReference{{Name: "registry"}}))
		Expect(obj.Spec.Selector.MatchLabels).Should(Equal(map[string]string{"app": "mysql"}))
		Expect(obj.Spec.MinReadySeconds).Should(Equal(int32(10)))
		Expect(obj.Spec.VolumeClaimTemplates).Should(HaveLen(1))
		Expect(obj.Spec.VolumeClaimTemplates[0].Name).Should(Equal("data"))
		Expect(obj.Spec.PersistentVolumeClaimRetentionPolicy).Should(Equal(retentionPolicy))
		Expect(obj.Spec.InstanceSetName).Should(Equal("mysql"))
		Expect(obj.Spec.InstanceTemplateName).Should(Equal("az-a"))
		Expect(obj.Spec.InstanceUpdateStrategyType).Should(Equal(&instanceUpdateStrategy.Type))
		Expect(obj.Spec.PodUpdatePolicy).Should(Equal(appsv1.PreferInPlacePodUpdatePolicyType))
		Expect(obj.Spec.PodUpgradePolicy).Should(Equal(appsv1.ReCreatePodUpdatePolicyType))
		Expect(obj.Spec.Roles).Should(Equal([]workloads.ReplicaRole{{Name: "leader", UpdatePriority: 10}}))
		Expect(obj.Spec.LifecycleActions).Should(Equal(lifecycleActions))
		Expect(obj.Spec.Configs).Should(Equal([]workloads.ConfigTemplate{{Name: "mysql-conf"}}))
		Expect(obj.Spec.InstanceAssistantObjects).Should(Equal(assistantObjects))
	})

	It("should leave update strategy unset when strategy is nil", func() {
		obj := NewInstanceBuilder("default", "mysql-0").
			SetInstanceUpdateStrategyType(nil).
			GetObject()

		Expect(obj.Spec.InstanceUpdateStrategyType).Should(BeNil())
	})
})
