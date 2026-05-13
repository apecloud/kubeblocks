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

package factory

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("builder", func() {
	const compDefName = "test-compdef"
	const clusterName = "test-cluster"
	const mysqlCompName = "mysql"

	allFieldsCompDefObj := func(needCreate bool) *appsv1.ComponentDefinition {
		By("By assure an componentDefinition obj")
		compDebObj := testapps.NewComponentDefinitionFactory(compDefName).
			SetDefaultSpec().
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, compDebObj)).Should(Succeed())
		}
		return compDebObj
	}

	newAllFieldsClusterObj := func(compDefObj *appsv1.ComponentDefinition, create bool) (*appsv1.Cluster, *appsv1.ComponentDefinition, types.NamespacedName) {
		// setup Cluster obj requires default ComponentDefinition object
		if compDefObj == nil {
			compDefObj = allFieldsCompDefObj(create)
		}
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			AddComponent(mysqlCompName, compDefObj.GetName()).
			SetReplicas(1).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddComponentService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddComponentService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			GetObject()
		key := client.ObjectKeyFromObject(clusterObj)
		if create {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}
		return clusterObj, compDefObj, key
	}

	newAllFieldsSynthesizedComponent := func(compDef *appsv1.ComponentDefinition, cluster *appsv1.Cluster) *component.SynthesizedComponent {
		By("assign every available fields")
		comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
		Expect(err).Should(Succeed())
		synthesizeComp, err := component.BuildSynthesizedComponent(testCtx.Ctx, testCtx.Cli, compDef, comp)
		Expect(err).Should(Succeed())
		Expect(synthesizeComp).ShouldNot(BeNil())
		// to resolve and inject env vars
		synthesizeComp.Annotations = cluster.Annotations
		_, envVars, err := component.ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizeComp, nil)
		Expect(err).Should(Succeed())
		component.InjectEnvVars(synthesizeComp, envVars, nil)
		return synthesizeComp
	}

	newClusterObjs := func(compDefObj *appsv1.ComponentDefinition) (*appsv1.ComponentDefinition, *appsv1.Cluster, *component.SynthesizedComponent) {
		cluster, compDef, _ := newAllFieldsClusterObj(compDefObj, false)
		synthesizedComponent := newAllFieldsSynthesizedComponent(compDef.DeepCopy(), cluster)
		return compDef, cluster, synthesizedComponent
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds InstanceSet correctly", func() {
			compDef, cluster, synthesizedComponent := newClusterObjs(nil)

			its, err := BuildInstanceSet(synthesizedComponent, nil)
			Expect(err).Should(BeNil())
			Expect(its).ShouldNot(BeNil())

			By("set replicas = 0")
			newComponent := *synthesizedComponent
			newComponent.Replicas = 0
			its, err = BuildInstanceSet(&newComponent, nil)
			Expect(err).Should(BeNil())
			Expect(its).ShouldNot(BeNil())
			Expect(*its.Spec.Replicas).Should(Equal(int32(0)))

			By("set replicas = 2")
			cluster.Spec.ComponentSpecs[0].Replicas = 2
			synthesizedComp := newAllFieldsSynthesizedComponent(compDef, cluster)
			its, err = BuildInstanceSet(synthesizedComp, nil)
			Expect(err).Should(BeNil())
			Expect(its).ShouldNot(BeNil())
			Expect(*its.Spec.Replicas).Should(BeEquivalentTo(2))

			// test roles
			Expect(its.Spec.Roles).Should(HaveLen(len(compDef.Spec.Roles)))

			// test update strategy
			Expect(its.Spec.InstanceUpdateStrategy).Should(BeNil())
			Expect(its.Spec.MemberUpdateStrategy).ShouldNot(BeNil())
			Expect(*its.Spec.MemberUpdateStrategy).Should(BeEquivalentTo(workloads.BestEffortParallelUpdateStrategy))
		})

		It("builds ConfigMap with template correctly", func() {
			config := map[string]string{}
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			tplCfg := appsv1.ComponentFileTemplate{
				Name:     "test-config-tpl",
				Template: "test-config-tpl",
			}
			configmap := BuildConfigMapWithTemplate(cluster, synthesizedComponent, config,
				"test-cm", tplCfg)
			Expect(configmap).ShouldNot(BeNil())
		})

		It("builds config manager sidecar container correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			sidecarRenderedParam := &cfgcm.CfgManagerBuildParams{
				ManagerName:   "cfgmgr",
				ComponentName: synthesizedComponent.Name,
				Image:         constant.KBToolsImage,
				Args:          []string{},
				Envs:          []corev1.EnvVar{},
				Volumes:       []corev1.VolumeMount{},
				Cluster:       cluster,
			}
			configmap, err := BuildCfgManagerContainer(sidecarRenderedParam)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
			Expect(configmap.SecurityContext).Should(BeNil())
		})

		It("builds config manager sidecar container correctly", func() {
			_, cluster, _ := newClusterObjs(nil)
			sidecarRenderedParam := &cfgcm.CfgManagerBuildParams{
				ManagerName:           "cfgmgr",
				Image:                 constant.KBToolsImage,
				ShareProcessNamespace: true,
				Args:                  []string{},
				Envs:                  []corev1.EnvVar{},
				Volumes:               []corev1.VolumeMount{},
				Cluster:               cluster,
			}
			configmap, err := BuildCfgManagerContainer(sidecarRenderedParam)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
			Expect(configmap.SecurityContext).ShouldNot(BeNil())
			Expect(configmap.SecurityContext.RunAsUser).ShouldNot(BeNil())
			Expect(*configmap.SecurityContext.RunAsUser).Should(BeEquivalentTo(int64(0)))
		})

		It("builds cfg manager tools  correctly", func() {
			_, cluster, _ := newClusterObjs(nil)
			cfgManagerParams := &cfgcm.CfgManagerBuildParams{
				ManagerName: constant.ConfigSidecarName,
				Image:       viper.GetString(constant.KBToolsImage),
				Cluster:     cluster,
			}
			toolContainers := []parametersv1alpha1.ToolConfig{
				{Name: "test-tool", Image: "test-image", Command: []string{"sh"}},
			}

			obj, err := BuildCfgManagerToolsContainer(cfgManagerParams, toolContainers, map[string]cfgcm.ConfigSpecMeta{})
			Expect(err).Should(BeNil())
			Expect(obj).ShouldNot(BeEmpty())
		})

		It("builds serviceaccount correctly", func() {
			_, cluster, synthesizedComp := newClusterObjs(nil)
			expectName := fmt.Sprintf("kb-%s", cluster.Name)
			sa := BuildServiceAccount(synthesizedComp, expectName)
			Expect(sa).ShouldNot(BeNil())
			Expect(sa.Name).Should(Equal(expectName))
		})

		It("builds rolebinding correctly", func() {
			_, cluster, synthesizedComp := newClusterObjs(nil)
			expectName := fmt.Sprintf("kb-%s", cluster.Name)
			rb := BuildRoleBinding(synthesizedComp, expectName, &rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     constant.RBACRoleName,
			}, expectName)
			Expect(rb).ShouldNot(BeNil())
			Expect(rb.Name).Should(Equal(expectName))
		})
	})

	Context("cluster default resources", func() {
		AfterEach(func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, "")
		})

		newInstanceSet := func() *workloads.InstanceSet {
			return &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "main"},
								{Name: "sidecar"},
							},
							InitContainers: []corev1.Container{
								{Name: "init"},
							},
						},
					},
				},
			}
		}

		It("does not inject zero resources into init and sidecar containers when not configured", func() {
			its := newInstanceSet()

			Expect(setDefaultResourceLimits(its)).Should(Succeed())

			Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
			Expect(its.Spec.Template.Spec.Containers[1].Resources.Requests).Should(BeNil())
			Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(BeNil())
			Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Requests).Should(BeNil())
			Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(BeNil())
		})

		It("keeps zero resource limit behavior when zero is true", func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{},"limits":{}}`)
			its := newInstanceSet()

			Expect(setDefaultResourceLimits(its)).Should(Succeed())

			Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
			Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
			Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		})

		It("applies configured resources to init and sidecar containers", func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
			its := newInstanceSet()

			Expect(setDefaultResourceLimits(its)).Should(Succeed())

			main := its.Spec.Template.Spec.Containers[0]
			sidecar := its.Spec.Template.Spec.Containers[1]
			initContainer := its.Spec.Template.Spec.InitContainers[0]
			Expect(main.Resources.Requests).Should(BeNil())
			Expect(main.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
			Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
			Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
			Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
			Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
		})

		It("lets configured resource names override zero by resource name", func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m"},"limits":{}}`)
			its := newInstanceSet()

			Expect(setDefaultResourceLimits(its)).Should(Succeed())

			sidecar := its.Spec.Template.Spec.Containers[1]
			initContainer := its.Spec.Template.Spec.InitContainers[0]
			Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(sidecar.Resources.Requests).ShouldNot(HaveKey(corev1.ResourceMemory))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
			Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
			Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		})

		It("does not override sidecar resource values already set by definitions", func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
			its := newInstanceSet()
			its.Spec.Template.Spec.Containers[1].Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("250m"),
			}

			Expect(setDefaultResourceLimits(its)).Should(Succeed())

			sidecar := its.Spec.Template.Spec.Containers[1]
			Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
			Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
			Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
		})

		It("returns an error when cluster default resources are invalid", func() {
			viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":`)
			its := newInstanceSet()

			Expect(setDefaultResourceLimits(its)).ShouldNot(Succeed())
		})
	})
})
