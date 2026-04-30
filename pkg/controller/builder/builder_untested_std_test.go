/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// --- InstanceBuilder ---

func TestInstanceBuilder(t *testing.T) {
	b := NewInstanceBuilder("ns", "inst-0")
	require.NotNil(t, b)
	obj := b.GetObject()
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "inst-0", obj.Name)
}

func TestInstanceBuilder_SetPodTemplate(t *testing.T) {
	tmpl := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "nginx"}}},
	}
	obj := NewInstanceBuilder("ns", "inst").SetPodTemplate(tmpl).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.Containers, 1)
}

func TestInstanceBuilder_SetContainers(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetContainers([]corev1.Container{{Name: "c1"}, {Name: "c2"}}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.Containers, 2)
}

func TestInstanceBuilder_SetInitContainers(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetInitContainers([]corev1.Container{{Name: "init"}}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.InitContainers, 1)
}

func TestInstanceBuilder_SetNodeName(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetNodeName(types.NodeName("node-1")).GetObject()
	assert.Equal(t, "node-1", obj.Spec.Template.Spec.NodeName)
}

func TestInstanceBuilder_SetHostnameSubdomain(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetHostname("myhost").SetSubdomain("mysub").GetObject()
	assert.Equal(t, "myhost", obj.Spec.Template.Spec.Hostname)
	assert.Equal(t, "mysub", obj.Spec.Template.Spec.Subdomain)
}

func TestInstanceBuilder_AddInitContainer(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		AddInitContainer(corev1.Container{Name: "init1"}).
		AddInitContainer(corev1.Container{Name: "init2"}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.InitContainers, 2)
}

func TestInstanceBuilder_AddContainer(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		AddContainer(corev1.Container{Name: "c1"}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.Containers, 1)
}

func TestInstanceBuilder_AddVolumes(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		AddVolumes(corev1.Volume{Name: "v1"}, corev1.Volume{Name: "v2"}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.Volumes, 2)
}

func TestInstanceBuilder_SetRestartPolicy(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetRestartPolicy(corev1.RestartPolicyAlways).GetObject()
	assert.Equal(t, corev1.RestartPolicyAlways, obj.Spec.Template.Spec.RestartPolicy)
}

func TestInstanceBuilder_SetSecurityContext(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetSecurityContext(corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}).GetObject()
	require.NotNil(t, obj.Spec.Template.Spec.SecurityContext)
	assert.Equal(t, int64(1000), *obj.Spec.Template.Spec.SecurityContext.RunAsUser)
}

func TestInstanceBuilder_AddTolerations(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		AddTolerations(corev1.Toleration{Key: "k1"}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.Tolerations, 1)
}

func TestInstanceBuilder_AddServiceAccount(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").AddServiceAccount("sa1").GetObject()
	assert.Equal(t, "sa1", obj.Spec.Template.Spec.ServiceAccountName)
}

func TestInstanceBuilder_SetNodeSelector(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetNodeSelector(map[string]string{"zone": "us-east"}).GetObject()
	assert.Equal(t, "us-east", obj.Spec.Template.Spec.NodeSelector["zone"])
}

func TestInstanceBuilder_SetAffinity(t *testing.T) {
	aff := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}}
	obj := NewInstanceBuilder("ns", "inst").SetAffinity(aff).GetObject()
	assert.NotNil(t, obj.Spec.Template.Spec.Affinity)
}

func TestInstanceBuilder_SetTopologySpreadConstraints(t *testing.T) {
	tsc := []corev1.TopologySpreadConstraint{{MaxSkew: 1, TopologyKey: "zone"}}
	obj := NewInstanceBuilder("ns", "inst").SetTopologySpreadConstraints(tsc).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.TopologySpreadConstraints, 1)
}

func TestInstanceBuilder_SetActiveDeadlineSeconds(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetActiveDeadlineSeconds(ptr.To(int64(60))).GetObject()
	assert.Equal(t, int64(60), *obj.Spec.Template.Spec.ActiveDeadlineSeconds)
}

func TestInstanceBuilder_SetImagePullSecrets(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetImagePullSecrets([]corev1.LocalObjectReference{{Name: "secret1"}}).GetObject()
	assert.Len(t, obj.Spec.Template.Spec.ImagePullSecrets, 1)
}

func TestInstanceBuilder_SetSelector(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}}
	obj := NewInstanceBuilder("ns", "inst").SetSelector(sel).GetObject()
	assert.NotNil(t, obj.Spec.Selector)
}

func TestInstanceBuilder_SetSelectorMatchLabels(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetSelectorMatchLabels(map[string]string{"app": "test"}).GetObject()
	assert.Equal(t, "test", obj.Spec.Selector.MatchLabels["app"])
}

func TestInstanceBuilder_SetSelectorMatchLabels_ExistingSelector(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetSelector(&metav1.LabelSelector{}).
		SetSelectorMatchLabels(map[string]string{"app": "test"}).GetObject()
	assert.Equal(t, "test", obj.Spec.Selector.MatchLabels["app"])
}

func TestInstanceBuilder_SetMinReadySeconds(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetMinReadySeconds(30).GetObject()
	assert.Equal(t, int32(30), obj.Spec.MinReadySeconds)
}

func TestInstanceBuilder_AddVolumeClaimTemplate(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	obj := NewInstanceBuilder("ns", "inst").AddVolumeClaimTemplate(pvc).GetObject()
	assert.Len(t, obj.Spec.VolumeClaimTemplates, 1)
}

func TestInstanceBuilder_SetPVCRetentionPolicy(t *testing.T) {
	policy := &workloads.PersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
	}
	obj := NewInstanceBuilder("ns", "inst").SetPVCRetentionPolicy(policy).GetObject()
	assert.NotNil(t, obj.Spec.PersistentVolumeClaimRetentionPolicy)
}

func TestInstanceBuilder_SetInstanceSetName(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetInstanceSetName("my-its").GetObject()
	assert.Equal(t, "my-its", obj.Spec.InstanceSetName)
}

func TestInstanceBuilder_SetInstanceTemplateName(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetInstanceTemplateName("tmpl-1").GetObject()
	assert.Equal(t, "tmpl-1", obj.Spec.InstanceTemplateName)
}

func TestInstanceBuilder_SetInstanceUpdateStrategyType(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		strategy := &workloads.InstanceUpdateStrategy{Type: "RollingUpdate"}
		obj := NewInstanceBuilder("ns", "inst").SetInstanceUpdateStrategyType(strategy).GetObject()
		require.NotNil(t, obj.Spec.InstanceUpdateStrategyType)
	})
	t.Run("nil", func(t *testing.T) {
		obj := NewInstanceBuilder("ns", "inst").SetInstanceUpdateStrategyType(nil).GetObject()
		assert.Nil(t, obj.Spec.InstanceUpdateStrategyType)
	})
}

func TestInstanceBuilder_SetPolicies(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").
		SetPodUpdatePolicy("InPlaceOnly").
		SetPodUpgradePolicy("PreferInPlace").GetObject()
	assert.Equal(t, workloads.PodUpdatePolicyType("InPlaceOnly"), obj.Spec.PodUpdatePolicy)
	assert.Equal(t, workloads.PodUpdatePolicyType("PreferInPlace"), obj.Spec.PodUpgradePolicy)
}

func TestInstanceBuilder_SetRoles(t *testing.T) {
	roles := []workloads.ReplicaRole{{Name: "leader"}}
	obj := NewInstanceBuilder("ns", "inst").SetRoles(roles).GetObject()
	assert.Len(t, obj.Spec.Roles, 1)
}

func TestInstanceBuilder_SetLifecycleActions(t *testing.T) {
	actions := &workloads.LifecycleActions{}
	obj := NewInstanceBuilder("ns", "inst").SetLifecycleActions(actions).GetObject()
	assert.NotNil(t, obj.Spec.LifecycleActions)
}

func TestInstanceBuilder_SetConfigs(t *testing.T) {
	configs := []workloads.ConfigTemplate{{Name: "cfg1"}}
	obj := NewInstanceBuilder("ns", "inst").SetConfigs(configs).GetObject()
	assert.Len(t, obj.Spec.Configs, 1)
}

func TestInstanceBuilder_SetInstanceAssistantObjects(t *testing.T) {
	objs := []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{}}}
	obj := NewInstanceBuilder("ns", "inst").SetInstanceAssistantObjects(objs).GetObject()
	assert.Len(t, obj.Spec.InstanceAssistantObjects, 1)
}

func TestInstanceBuilder_SetFinalizers(t *testing.T) {
	obj := NewInstanceBuilder("ns", "inst").SetFinalizers().GetObject()
	assert.Nil(t, obj.Finalizers)
}

// --- ClusterRoleBuilder ---

func TestClusterRoleBuilder(t *testing.T) {
	obj := NewClusterRoleBuilder("my-role").GetObject()
	assert.Equal(t, "my-role", obj.Name)
}

func TestClusterRoleBuilder_AddPolicyRules(t *testing.T) {
	rules := []rbacv1.PolicyRule{
		{Verbs: []string{"get", "list"}, Resources: []string{"pods"}, APIGroups: []string{""}},
	}
	obj := NewClusterRoleBuilder("my-role").AddPolicyRules(rules).GetObject()
	assert.Len(t, obj.Rules, 1)
}

// --- ComponentParameterBuilder ---

func TestComponentParameterBuilder(t *testing.T) {
	obj := NewComponentParameterBuilder("ns", "param").GetObject()
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "param", obj.Name)
}

func TestComponentParameterBuilder_SetFields(t *testing.T) {
	obj := NewComponentParameterBuilder("ns", "param").
		SetClusterName("cluster1").
		SetCompName("comp1").
		SetInitial(&parametersv1alpha1.ParameterInputs{}).GetObject()
	assert.Equal(t, "cluster1", obj.Spec.ClusterName)
	assert.Equal(t, "comp1", obj.Spec.ComponentName)
	assert.NotNil(t, obj.Spec.Initial)
}

// --- ComponentBuilder ---

func TestComponentBuilder(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "compdef-1").GetObject()
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "comp", obj.Name)
	assert.Equal(t, "compdef-1", obj.Spec.CompDef)
}

func TestComponentBuilder_SetTerminationPolicy(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetTerminationPolicy(appsv1.Delete).GetObject()
	assert.Equal(t, appsv1.Delete, obj.Spec.TerminationPolicy)
}

func TestComponentBuilder_SetServiceVersion(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetServiceVersion("8.0").GetObject()
	assert.Equal(t, "8.0", obj.Spec.ServiceVersion)
}

func TestComponentBuilder_SetLabelsAnnotations(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetLabels(map[string]string{"l": "v"}).
		SetAnnotations(map[string]string{"a": "v"}).GetObject()
	assert.Equal(t, "v", obj.Spec.Labels["l"])
	assert.Equal(t, "v", obj.Spec.Annotations["a"])
}

func TestComponentBuilder_SetEnv(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetEnv([]corev1.EnvVar{{Name: "FOO", Value: "bar"}}).GetObject()
	assert.Len(t, obj.Spec.Env, 1)
}

func TestComponentBuilder_SetSchedulingPolicy(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetSchedulingPolicy(&appsv1.SchedulingPolicy{NodeName: "node1"}).GetObject()
	assert.Equal(t, "node1", obj.Spec.SchedulingPolicy.NodeName)
}

func TestComponentBuilder_SetReplicas(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetReplicas(3).GetObject()
	assert.Equal(t, int32(3), obj.Spec.Replicas)
}

func TestComponentBuilder_SetConfigs(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetConfigs([]appsv1.ClusterComponentConfig{{Name: ptr.To("cfg1")}}).GetObject()
	assert.Len(t, obj.Spec.Configs, 1)
}

func TestComponentBuilder_SetServiceAccountName(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetServiceAccountName("sa1").GetObject()
	assert.Equal(t, "sa1", obj.Spec.ServiceAccountName)
}

func TestComponentBuilder_SetParallelPodManagementConcurrency(t *testing.T) {
	c := intstr.FromInt32(5)
	obj := NewComponentBuilder("ns", "comp", "cd").SetParallelPodManagementConcurrency(&c).GetObject()
	assert.Equal(t, int32(5), obj.Spec.ParallelPodManagementConcurrency.IntVal)
}

func TestComponentBuilder_SetPodUpdatePolicy(t *testing.T) {
	policy := appsv1.PodUpdatePolicyType("InPlaceOnly")
	obj := NewComponentBuilder("ns", "comp", "cd").SetPodUpdatePolicy(&policy).GetObject()
	require.NotNil(t, obj.Spec.PodUpdatePolicy)
}

func TestComponentBuilder_SetPodUpgradePolicy(t *testing.T) {
	policy := appsv1.PodUpdatePolicyType("PreferInPlace")
	obj := NewComponentBuilder("ns", "comp", "cd").SetPodUpgradePolicy(&policy).GetObject()
	require.NotNil(t, obj.Spec.PodUpgradePolicy)
}

func TestComponentBuilder_SetInstanceUpdateStrategy(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetInstanceUpdateStrategy(&appsv1.InstanceUpdateStrategy{}).GetObject()
	assert.NotNil(t, obj.Spec.InstanceUpdateStrategy)
}

func TestComponentBuilder_SetResources(t *testing.T) {
	res := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
	}
	obj := NewComponentBuilder("ns", "comp", "cd").SetResources(res).GetObject()
	assert.NotNil(t, obj.Spec.Resources.Requests)
}

func TestComponentBuilder_SetDisableExporter(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetDisableExporter(ptr.To(true)).GetObject()
	assert.True(t, *obj.Spec.DisableExporter)
}

func TestComponentBuilder_SetTLSConfig(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		obj := NewComponentBuilder("ns", "comp", "cd").SetTLSConfig(true, &appsv1.Issuer{Name: "letsencrypt"}).GetObject()
		require.NotNil(t, obj.Spec.TLSConfig)
		assert.True(t, obj.Spec.TLSConfig.Enable)
	})
	t.Run("disabled", func(t *testing.T) {
		obj := NewComponentBuilder("ns", "comp", "cd").SetTLSConfig(false, nil).GetObject()
		assert.Nil(t, obj.Spec.TLSConfig)
	})
}

func TestComponentBuilder_SetVolumeClaimTemplates(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetVolumeClaimTemplates([]appsv1.PersistentVolumeClaimTemplate{
			{Name: "data"},
		}).GetObject()
	assert.Len(t, obj.Spec.VolumeClaimTemplates, 1)
}

func TestComponentBuilder_SetPVCRetentionPolicy(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetPVCRetentionPolicy(&appsv1.PersistentVolumeClaimRetentionPolicy{}).GetObject()
	assert.NotNil(t, obj.Spec.PersistentVolumeClaimRetentionPolicy)
}

func TestComponentBuilder_SetVolumes(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetVolumes([]corev1.Volume{{Name: "v1"}}).GetObject()
	assert.Len(t, obj.Spec.Volumes, 1)
}

func TestComponentBuilder_SetNetwork(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetNetwork(&appsv1.ComponentNetwork{}).GetObject()
	assert.NotNil(t, obj.Spec.Network)
}

func TestComponentBuilder_SetServices(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetServices([]appsv1.ClusterComponentService{
			{Name: "svc1", ServiceType: corev1.ServiceTypeClusterIP},
		}).GetObject()
	assert.Len(t, obj.Spec.Services, 1)
}

func TestComponentBuilder_SetSystemAccounts(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetSystemAccounts([]appsv1.ComponentSystemAccount{{Name: "admin"}}).GetObject()
	assert.Len(t, obj.Spec.SystemAccounts, 1)
}

func TestComponentBuilder_SetServiceRefs(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetServiceRefs([]appsv1.ServiceRef{{Name: "ref1"}}).GetObject()
	assert.Len(t, obj.Spec.ServiceRefs, 1)
}

func TestComponentBuilder_SetInstances(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetInstances([]appsv1.InstanceTemplate{{Name: "tmpl1"}}).GetObject()
	assert.Len(t, obj.Spec.Instances, 1)
}

func TestComponentBuilder_SetOrdinals(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetOrdinals(appsv1.Ordinals{Ranges: []appsv1.Range{{Start: 0, End: 2}}}).GetObject()
	assert.Len(t, obj.Spec.Ordinals.Ranges, 1)
}

func TestComponentBuilder_SetFlatInstanceOrdinal(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetFlatInstanceOrdinal(true).GetObject()
	assert.True(t, obj.Spec.FlatInstanceOrdinal)
}

func TestComponentBuilder_SetOfflineInstances(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetOfflineInstances([]string{"inst-1"}).GetObject()
	assert.Len(t, obj.Spec.OfflineInstances, 1)
}

func TestComponentBuilder_SetRuntimeClassName(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		obj := NewComponentBuilder("ns", "comp", "cd").SetRuntimeClassName(ptr.To("kata")).GetObject()
		assert.Equal(t, "kata", *obj.Spec.RuntimeClassName)
	})
	t.Run("nil", func(t *testing.T) {
		obj := NewComponentBuilder("ns", "comp", "cd").SetRuntimeClassName(nil).GetObject()
		assert.Nil(t, obj.Spec.RuntimeClassName)
	})
}

func TestComponentBuilder_SetStop(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetStop(ptr.To(true)).GetObject()
	assert.True(t, *obj.Spec.Stop)
}

func TestComponentBuilder_SetSidecars(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").
		SetSidecars([]appsv1.Sidecar{{Name: "sidecar1"}}).GetObject()
	assert.Len(t, obj.Spec.Sidecars, 1)
}

func TestComponentBuilder_SetEnableInstanceAPI(t *testing.T) {
	obj := NewComponentBuilder("ns", "comp", "cd").SetEnableInstanceAPI(ptr.To(true)).GetObject()
	assert.True(t, *obj.Spec.EnableInstanceAPI)
}

// --- ComponentDefinitionBuilder ---

func TestComponentDefinitionBuilder(t *testing.T) {
	obj := NewComponentDefinitionBuilder("compdef").GetObject()
	assert.Equal(t, "compdef", obj.Name)
}

func TestComponentDefinitionBuilder_SetRuntime(t *testing.T) {
	t.Run("nil container", func(t *testing.T) {
		obj := NewComponentDefinitionBuilder("cd").SetRuntime(nil).GetObject()
		assert.Nil(t, obj.Spec.Runtime.Containers)
	})
	t.Run("new container", func(t *testing.T) {
		c := &corev1.Container{Name: "main", Image: "nginx"}
		obj := NewComponentDefinitionBuilder("cd").SetRuntime(c).GetObject()
		assert.Len(t, obj.Spec.Runtime.Containers, 1)
	})
	t.Run("update existing container", func(t *testing.T) {
		c1 := &corev1.Container{Name: "main", Image: "nginx:1.0"}
		c2 := &corev1.Container{Name: "main", Image: "nginx:2.0"}
		obj := NewComponentDefinitionBuilder("cd").SetRuntime(c1).SetRuntime(c2).GetObject()
		assert.Len(t, obj.Spec.Runtime.Containers, 1)
		assert.Equal(t, "nginx:2.0", obj.Spec.Runtime.Containers[0].Image)
	})
}

func TestComponentDefinitionBuilder_AddEnv(t *testing.T) {
	c := &corev1.Container{Name: "main"}
	obj := NewComponentDefinitionBuilder("cd").SetRuntime(c).
		AddEnv("main", corev1.EnvVar{Name: "FOO", Value: "bar"}).GetObject()
	assert.Len(t, obj.Spec.Runtime.Containers[0].Env, 1)
}

func TestComponentDefinitionBuilder_AddVolumeMounts(t *testing.T) {
	c := &corev1.Container{Name: "main", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}
	obj := NewComponentDefinitionBuilder("cd").SetRuntime(c).
		AddVolumeMounts("main", []corev1.VolumeMount{{Name: "log", MountPath: "/log"}}).GetObject()
	assert.Len(t, obj.Spec.Runtime.Containers[0].VolumeMounts, 2)
}

func TestComponentDefinitionBuilder_AddVar(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").
		AddVar(appsv1.EnvVar{Name: "V1"}).
		AddVar(appsv1.EnvVar{Name: "V2"}).GetObject()
	assert.Len(t, obj.Spec.Vars, 2)
}

func TestComponentDefinitionBuilder_AddVolume(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").
		AddVolume("data", true, 90).GetObject()
	assert.Len(t, obj.Spec.Volumes, 1)
	assert.Equal(t, "data", obj.Spec.Volumes[0].Name)
	assert.True(t, obj.Spec.Volumes[0].NeedSnapshot)
	assert.Equal(t, 90, obj.Spec.Volumes[0].HighWatermark)
}

func TestComponentDefinitionBuilder_AddService(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").
		AddService("svc1", "mysvc", 3306, "leader").GetObject()
	assert.Len(t, obj.Spec.Services, 1)
	assert.Equal(t, "svc1", obj.Spec.Services[0].Name)
	assert.Equal(t, int32(3306), obj.Spec.Services[0].Spec.Ports[0].Port)
}

func TestComponentDefinitionBuilder_AddServiceExt(t *testing.T) {
	spec := corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}}
	obj := NewComponentDefinitionBuilder("cd").
		AddServiceExt("svc1", "mysvc", spec, "leader").
		AddServiceExt("svc2", "mysvc2", spec, "").GetObject()
	assert.Len(t, obj.Spec.Services, 2)
}

func TestComponentDefinitionBuilder_SetPolicyRules(t *testing.T) {
	rules := []rbacv1.PolicyRule{{Verbs: []string{"get"}, Resources: []string{"pods"}, APIGroups: []string{""}}}
	obj := NewComponentDefinitionBuilder("cd").SetPolicyRules(rules).GetObject()
	assert.Len(t, obj.Spec.PolicyRules, 1)
}

func TestComponentDefinitionBuilder_SetLabels(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").SetLabels(map[string]string{"l": "v"}).GetObject()
	assert.Equal(t, "v", obj.Spec.Labels["l"])
}

func TestComponentDefinitionBuilder_SetReplicasLimit(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").SetReplicasLimit(1, 5).GetObject()
	require.NotNil(t, obj.Spec.ReplicasLimit)
	assert.Equal(t, int32(1), obj.Spec.ReplicasLimit.MinReplicas)
	assert.Equal(t, int32(5), obj.Spec.ReplicasLimit.MaxReplicas)
}

func TestComponentDefinitionBuilder_SetUpdateStrategy(t *testing.T) {
	s := appsv1.SerialStrategy
	obj := NewComponentDefinitionBuilder("cd").SetUpdateStrategy(&s).GetObject()
	require.NotNil(t, obj.Spec.UpdateStrategy)
	assert.Equal(t, appsv1.SerialStrategy, *obj.Spec.UpdateStrategy)
}

func TestComponentDefinitionBuilder_AddRole(t *testing.T) {
	obj := NewComponentDefinitionBuilder("cd").
		AddRole("leader", 1, true).
		AddRole("follower", 2, false).GetObject()
	assert.Len(t, obj.Spec.Roles, 2)
	assert.Equal(t, "leader", obj.Spec.Roles[0].Name)
}

// SetLifecycleAction uses reflect internally and is covered by existing ginkgo tests

// --- PodBuilder ---

func TestPodBuilder(t *testing.T) {
	obj := NewPodBuilder("ns", "pod-0").GetObject()
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "pod-0", obj.Name)
}

func TestPodBuilder_SetPodSpec(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Name: "c1"}}}
	obj := NewPodBuilder("ns", "pod").SetPodSpec(spec).GetObject()
	assert.Len(t, obj.Spec.Containers, 1)
}

func TestPodBuilder_SetInitContainers(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").
		SetInitContainers([]corev1.Container{{Name: "init"}}).GetObject()
	assert.Len(t, obj.Spec.InitContainers, 1)
}

func TestPodBuilder_SetNodeName(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").SetNodeName("node-1").GetObject()
	assert.Equal(t, "node-1", obj.Spec.NodeName)
}

func TestPodBuilder_SetHostnameSubdomain(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").SetHostname("h").SetSubdomain("s").GetObject()
	assert.Equal(t, "h", obj.Spec.Hostname)
	assert.Equal(t, "s", obj.Spec.Subdomain)
}

func TestPodBuilder_SetFinalizers(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").SetFinalizers().GetObject()
	assert.Nil(t, obj.Finalizers)
}

func TestPodBuilder_AddInitContainer(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").
		AddInitContainer(corev1.Container{Name: "i1"}).
		AddInitContainer(corev1.Container{Name: "i2"}).GetObject()
	assert.Len(t, obj.Spec.InitContainers, 2)
}

func TestPodBuilder_SetAffinity(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").SetAffinity(&corev1.Affinity{}).GetObject()
	assert.NotNil(t, obj.Spec.Affinity)
}

func TestPodBuilder_SetTopologySpreadConstraints(t *testing.T) {
	tsc := []corev1.TopologySpreadConstraint{{MaxSkew: 1, TopologyKey: "zone"}}
	obj := NewPodBuilder("ns", "pod").SetTopologySpreadConstraints(tsc).GetObject()
	assert.Len(t, obj.Spec.TopologySpreadConstraints, 1)
}

func TestPodBuilder_SetActiveDeadlineSeconds(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").SetActiveDeadlineSeconds(ptr.To(int64(30))).GetObject()
	assert.Equal(t, int64(30), *obj.Spec.ActiveDeadlineSeconds)
}

func TestPodBuilder_SetImagePullSecrets(t *testing.T) {
	obj := NewPodBuilder("ns", "pod").
		SetImagePullSecrets([]corev1.LocalObjectReference{{Name: "sec"}}).GetObject()
	assert.Len(t, obj.Spec.ImagePullSecrets, 1)
}

// --- EventBuilder ---

func TestEventBuilder(t *testing.T) {
	obj := NewEventBuilder("ns", "evt").GetObject()
	assert.Equal(t, "ns", obj.Namespace)
}

func TestEventBuilder_AllSetters(t *testing.T) {
	now := metav1.Now()
	microNow := metav1.NewMicroTime(time.Now())
	obj := NewEventBuilder("ns", "evt").
		SetFirstTimestamp(now).
		SetLastTimestamp(now).
		SetEventTime(microNow).
		SetReportingController("ctrl").
		SetReportingInstance("inst").
		SetAction("Create").GetObject()
	assert.Equal(t, now, obj.FirstTimestamp)
	assert.Equal(t, now, obj.LastTimestamp)
	assert.Equal(t, "ctrl", obj.ReportingController)
	assert.Equal(t, "inst", obj.ReportingInstance)
	assert.Equal(t, "Create", obj.Action)
}

// --- BackupBuilder ---

func TestBackupBuilder_SetParentBackupName(t *testing.T) {
	obj := NewBackupBuilder("ns", "bk").SetParentBackupName("parent1").GetObject()
	assert.Equal(t, "parent1", obj.Spec.ParentBackupName)
}

// --- InstanceSetBuilder uncovered methods ---

func TestInstanceSetBuilder_SetPVCRetentionPolicy(t *testing.T) {
	policy := &workloads.PersistentVolumeClaimRetentionPolicy{}
	obj := NewInstanceSetBuilder("ns", "its").SetPVCRetentionPolicy(policy).GetObject()
	assert.NotNil(t, obj.Spec.PersistentVolumeClaimRetentionPolicy)
}

func TestInstanceSetBuilder_SetLifecycleActions(t *testing.T) {
	t.Run("both non-nil", func(t *testing.T) {
		lca := &appsv1.ComponentLifecycleActions{
			Switchover: &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"switch"}}},
		}
		vars := map[string]string{"K": "V"}
		obj := NewInstanceSetBuilder("ns", "its").SetLifecycleActions(lca, vars).GetObject()
		require.NotNil(t, obj.Spec.LifecycleActions)
		assert.NotNil(t, obj.Spec.LifecycleActions.Switchover)
		assert.Equal(t, "V", obj.Spec.LifecycleActions.TemplateVars["K"])
	})
	t.Run("both nil", func(t *testing.T) {
		obj := NewInstanceSetBuilder("ns", "its").SetLifecycleActions(nil, nil).GetObject()
		assert.Nil(t, obj.Spec.LifecycleActions)
	})
}

func TestInstanceSetBuilder_SetOrdinals(t *testing.T) {
	obj := NewInstanceSetBuilder("ns", "its").
		SetOrdinals(workloads.Ordinals{Ranges: []appsv1.Range{{Start: 0, End: 2}}}).GetObject()
	assert.Len(t, obj.Spec.Ordinals.Ranges, 1)
}

func TestInstanceSetBuilder_SetFlatInstanceOrdinal(t *testing.T) {
	obj := NewInstanceSetBuilder("ns", "its").SetFlatInstanceOrdinal(true).GetObject()
	assert.True(t, obj.Spec.FlatInstanceOrdinal)
}

func TestInstanceSetBuilder_SetOfflineInstances(t *testing.T) {
	obj := NewInstanceSetBuilder("ns", "its").SetOfflineInstances([]string{"inst-1"}).GetObject()
	assert.Len(t, obj.Spec.OfflineInstances, 1)
}

func TestInstanceSetBuilder_SetDisableDefaultHeadlessService(t *testing.T) {
	obj := NewInstanceSetBuilder("ns", "its").SetDisableDefaultHeadlessService(true).GetObject()
	assert.True(t, obj.Spec.DisableDefaultHeadlessService)
}

func TestInstanceSetBuilder_SetEnableInstanceAPI(t *testing.T) {
	obj := NewInstanceSetBuilder("ns", "its").SetEnableInstanceAPI(ptr.To(true)).GetObject()
	assert.True(t, *obj.Spec.EnableInstanceAPI)
}

func TestInstanceSetBuilder_SetInstanceAssistantObjects(t *testing.T) {
	objs := []corev1.ObjectReference{{Name: "obj1"}}
	obj := NewInstanceSetBuilder("ns", "its").SetInstanceAssistantObjects(objs).GetObject()
	assert.Len(t, obj.Spec.InstanceAssistantObjects, 1)
}

// --- ServiceBuilder uncovered methods ---

func TestServiceBuilder_NewServiceBuilder(t *testing.T) {
	obj := NewServiceBuilder("ns", "svc").GetObject()
	assert.Equal(t, "ns", obj.Namespace)
	assert.Equal(t, "svc", obj.Name)
}

func TestServiceBuilder_SetType_Empty(t *testing.T) {
	obj := NewServiceBuilder("ns", "svc").SetType("").GetObject()
	assert.Empty(t, string(obj.Spec.Type))
}

func TestServiceBuilder_SetSpec(t *testing.T) {
	spec := &corev1.ServiceSpec{ClusterIP: "10.0.0.1"}
	obj := NewServiceBuilder("ns", "svc").SetSpec(spec).GetObject()
	assert.Equal(t, "10.0.0.1", obj.Spec.ClusterIP)
}

func TestServiceBuilder_SetSpec_Nil(t *testing.T) {
	obj := NewServiceBuilder("ns", "svc").SetSpec(nil).GetObject()
	assert.Empty(t, obj.Spec.ClusterIP)
}

func TestServiceBuilder_Optimize4ExternalTraffic(t *testing.T) {
	t.Run("loadbalancer without policy", func(t *testing.T) {
		obj := NewServiceBuilder("ns", "svc").
			SetSpec(&corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}).
			Optimize4ExternalTraffic().GetObject()
		assert.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, obj.Spec.ExternalTrafficPolicy)
	})
	t.Run("clusterip no-op", func(t *testing.T) {
		obj := NewServiceBuilder("ns", "svc").
			SetType(corev1.ServiceTypeClusterIP).
			Optimize4ExternalTraffic().GetObject()
		assert.Empty(t, string(obj.Spec.ExternalTrafficPolicy))
	})
}

// --- ServiceAccountBuilder ---

func TestServiceAccountBuilder_SetImagePullSecrets(t *testing.T) {
	obj := NewServiceAccountBuilder("ns", "sa").
		SetImagePullSecrets([]corev1.LocalObjectReference{{Name: "sec"}}).GetObject()
	assert.Len(t, obj.ImagePullSecrets, 1)
}

// --- ServiceDescriptorBuilder ---

func TestServiceDescriptorBuilder_SetAuthUsername(t *testing.T) {
	obj := NewServiceDescriptorBuilder("ns", "sd").
		SetAuthUsername(appsv1.CredentialVar{Value: "admin"}).GetObject()
	require.NotNil(t, obj.Spec.Auth)
	require.NotNil(t, obj.Spec.Auth.Username)
	assert.Equal(t, "admin", obj.Spec.Auth.Username.Value)
}

func TestServiceDescriptorBuilder_SetAuthPassword(t *testing.T) {
	obj := NewServiceDescriptorBuilder("ns", "sd").
		SetAuthPassword(appsv1.CredentialVar{Value: "secret"}).GetObject()
	require.NotNil(t, obj.Spec.Auth)
	require.NotNil(t, obj.Spec.Auth.Password)
	assert.Equal(t, "secret", obj.Spec.Auth.Password.Value)
}

// --- BaseBuilder ---

func TestBaseBuilder_SetName(t *testing.T) {
	obj := NewConfigMapBuilder("ns", "cm").SetName("newname").GetObject()
	assert.Equal(t, "newname", obj.Name)
}

func TestBaseBuilder_AddLabelsInMap_Empty(t *testing.T) {
	obj := NewConfigMapBuilder("ns", "cm").AddLabelsInMap(nil).GetObject()
	assert.Nil(t, obj.Labels)
}

func TestBaseBuilder_AddAnnotationsInMap_Empty(t *testing.T) {
	obj := NewConfigMapBuilder("ns", "cm").AddAnnotationsInMap(nil).GetObject()
	assert.Nil(t, obj.Annotations)
}

// --- ClusterBuilder ---

func TestClusterBuilder_SetResourceVersion(t *testing.T) {
	obj := NewClusterBuilder("ns", "cl").SetResourceVersion("123").GetObject()
	assert.Equal(t, "123", obj.ResourceVersion)
}
