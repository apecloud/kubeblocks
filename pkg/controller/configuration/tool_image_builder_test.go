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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("ToolsImageBuilderTest", func() {

	const kbToolsImage = "apecloud/kubeblocks-tools:latest"

	var noneCommand = []string{"/bin/true"}
	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var ClusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		clusterObj, ClusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(ClusterDefObj, clusterVersionObj)
		viper.SetDefault(constant.KBToolsImage, kbToolsImage)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("ToolsImageBuilderTest", func() {
		It("TestScriptSpec", func() {
			sts, err := factory.BuildSts(intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: logger,
			}, clusterObj, clusterComponent, "for_test_env")
			Expect(err).Should(Succeed())

			cfgManagerParams := &cfgcm.CfgManagerBuildParams{
				ManagerName:   constant.ConfigSidecarName,
				CharacterType: clusterComponent.CharacterType,
				ComponentName: clusterComponent.Name,
				SecreteName:   component.GenerateConnCredential(clusterObj.Name),
				EnvConfigName: component.GenerateComponentEnvName(clusterObj.Name, clusterComponent.Name),
				Image:         viper.GetString(constant.KBToolsImage),
				Volumes:       make([]corev1.VolumeMount, 0),
				Cluster:       clusterObj,
				ConfigSpecsBuildParams: []cfgcm.ConfigSpecMeta{{
					ConfigSpecInfo: cfgcm.ConfigSpecInfo{
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
								Command: noneCommand,
							},
							{
								Name:    "test2",
								Image:   "",
								Command: noneCommand,
							},
							{
								Name:    "test3",
								Image:   "$(KUBEBLOCKS_TOOLS_IMAGE)",
								Command: noneCommand,
							},
						},
					},
				}},
				ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
			}
			cfgManagerParams.ConfigSpecsBuildParams[0].ConfigSpec.VolumeName = "data"
			cfgManagerParams.ConfigSpecsBuildParams[0].ConfigSpec.LegacyRenderedConfigSpec = &appsv1alpha1.LegacyRenderedTemplateSpec{
				ConfigTemplateExtension: appsv1alpha1.ConfigTemplateExtension{
					Namespace:   testCtx.DefaultNamespace,
					TemplateRef: "secondary_template",
					Policy:      appsv1alpha1.NoneMergePolicy,
				},
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
