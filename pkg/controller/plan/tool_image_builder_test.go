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

package plan

import (
	configmanager "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	component2 "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("ToolsImageBuilderTest", func() {

	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"
	const mysqlCompDefName = "replicasets"
	const configSpecName = "test-config-spec"
	const kbToolsImage = "apecloud/kubeblocks-tools:latest"

	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var ClusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component2.SynthesizedComponent

	allFieldsClusterDefObj := func(needCreate bool) *appsv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefObj := apps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(apps.StatefulMySQLComponent, mysqlCompDefName).
			AddConfigTemplate(configSpecName, configSpecName, configSpecName, testCtx.DefaultNamespace, apps.ConfVolumeName).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
		}
		return clusterDefObj
	}

	allFieldsClusterVersionObj := func(needCreate bool) *appsv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionObj := apps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(mysqlCompDefName).
			AddContainerShort("mysql", apps.ApeCloudMySQLImage).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
		}
		return clusterVersionObj
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *appsv1alpha1.ClusterDefinition,
		clusterVersionObj *appsv1alpha1.ClusterVersion,
		needCreate bool,
	) (*appsv1alpha1.Cluster, *appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj requires default ClusterDefinition and ClusterVersion objects
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}
		pvcSpec := apps.NewPVCSpec("1Gi")
		clusterObj := apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompDefName).SetReplicas(1).
			AddVolumeClaimTemplate(apps.DataVolumeName, pvcSpec).
			AddService(apps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(apps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			GetObject()
		key := client.ObjectKeyFromObject(clusterObj)
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}
		return clusterObj, clusterDefObj, clusterVersionObj, key
	}

	newAllFieldsComponent := func(clusterDef *appsv1alpha1.ClusterDefinition, clusterVersion *appsv1alpha1.ClusterVersion) *component2.SynthesizedComponent {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDef, clusterVersion, false)
		By("assign every available fields")
		component, err := component2.BuildComponent(
			intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: logger,
			},
			nil,
			cluster,
			clusterDef,
			&clusterDef.Spec.ComponentDefs[0],
			&cluster.Spec.ComponentSpecs[0],
			&clusterVersion.Spec.ComponentVersions[0])
		Expect(err).Should(Succeed())
		Expect(component).ShouldNot(BeNil())
		return component
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		clusterObj, ClusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(ClusterDefObj, clusterVersionObj)
		viper.SetDefault(constant.KBToolsImage, kbToolsImage)
	})

	AfterEach(func() {
	})

	Context("ToolsImageBuilderTest", func() {
		It("TestScriptSpec", func() {
			sts, err := builder.BuildSts(intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: logger,
			}, clusterObj, clusterComponent, "for_test_env")
			Expect(err).Should(Succeed())

			cfgManagerParams := &configmanager.CfgManagerBuildParams{
				ManagerName:   constant.ConfigSidecarName,
				CharacterType: clusterComponent.CharacterType,
				ComponentName: clusterComponent.Name,
				SecreteName:   component2.GenerateConnCredential(clusterObj.Name),
				EnvConfigName: component2.GenerateComponentEnvName(clusterObj.Name, clusterComponent.Name),
				Image:         viper.GetString(constant.KBToolsImage),
				Volumes:       make([]corev1.VolumeMount, 0),
				Cluster:       clusterObj,
				ConfigSpecsBuildParams: []configmanager.ConfigSpecMeta{{
					ConfigSpecInfo: configmanager.ConfigSpecInfo{
						ConfigSpec:      clusterComponent.ConfigTemplates[0],
						ReloadType:      appsv1alpha1.TPLScriptType,
						FormatterConfig: appsv1alpha1.FormatterConfig{},
					},
					ToolsImageSpec: &appsv1alpha1.ToolsImageSpec{
						MountPoint: "/opt/images",
						ToolConfigs: []appsv1alpha1.ToolConfig{
							{
								Name:    "test",
								Image:   "test_images",
								Command: []string{"/bin/true"},
							},
							{
								Name:    "test2",
								Image:   "",
								Command: []string{"/bin/true"},
							},
							{
								Name:    "test3",
								Image:   "$(KUBEBLOCKS_TOOLS_IMAGE)",
								Command: []string{"/bin/true"},
							},
						},
					},
				}},
				ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
			}
			cfgManagerParams.ConfigSpecsBuildParams[0].ConfigSpec.VolumeName = "data"
			cfgManagerParams.ConfigSpecsBuildParams[0].ConfigSpec.LazyRenderedConfigSpec = &appsv1alpha1.LazyRenderedTemplateSpec{
				Namespace:   testCtx.DefaultNamespace,
				TemplateRef: "secondary_template",
				Policy:      appsv1alpha1.NoneMergePolicy,
			}
			Expect(buildConfigToolsContainer(cfgManagerParams, &sts.Spec.Template.Spec, clusterComponent)).Should(Succeed())
			Expect(4).Should(BeEquivalentTo(len(cfgManagerParams.ToolsContainers)))
			Expect("test_images").Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[0].Image))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[1].Image))
			Expect(kbToolsImage).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[2].Image))
			Expect(kbToolsImage).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[3].Image))
			Expect(initSecRenderedToolContainerName).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[3].Name))
		})
	})

})
