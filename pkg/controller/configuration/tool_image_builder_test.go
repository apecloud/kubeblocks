/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("ToolsImageBuilderTest", func() {
	const kbToolsImage = "apecloud/kubeblocks-tools:latest"

	var noneCommand = []string{"/bin/true"}
	var clusterObj *appsv1alpha1.Cluster
	var ClusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		clusterObj, ClusterDefObj, _ = newAllFieldsClusterObj(nil, false)
		clusterComponent = newAllFieldsSynthesizedComponent(ClusterDefObj, clusterObj)
		viper.SetDefault(constant.KBToolsImage, kbToolsImage)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("ToolsImageBuilderTest", func() {
		It("TestScriptSpec", func() {
			its, err := factory.BuildInstanceSet(clusterComponent, nil)
			Expect(err).Should(Succeed())

			cfgManagerParams := &cfgcm.CfgManagerBuildParams{
				ManagerName:   constant.ConfigSidecarName,
				ComponentName: clusterComponent.Name,
				Image:         viper.GetString(constant.KBToolsImage),
				Volumes:       make([]corev1.VolumeMount, 0),
				Cluster:       clusterObj,
				ConfigSpecsBuildParams: []cfgcm.ConfigSpecMeta{{
					ConfigSpecInfo: cfgcm.ConfigSpecInfo{
						ConfigSpec:      clusterComponent.ConfigTemplates[0],
						ReloadType:      appsv1beta1.TPLScriptType,
						FormatterConfig: appsv1beta1.FileFormatConfig{},
					},
					ToolsImageSpec: &appsv1beta1.ToolsSetup{
						MountPoint: "/opt/tools",
						ToolConfigs: []appsv1beta1.ToolConfig{
							{
								Name:    "test",
								Image:   "test_images",
								Command: noneCommand,
							},
							{
								Name:    "test2",
								Image:   "",
								Command: noneCommand,
								// AsContainerImage: cfgutil.ToPointer(true),
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
			Expect(buildReloadToolsContainer(cfgManagerParams, &its.Spec.Template.Spec)).Should(Succeed())
			Expect(4).Should(BeEquivalentTo(len(cfgManagerParams.ToolsContainers)))
			Expect("test_images").Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[0].Image))
			Expect(its.Spec.Template.Spec.Containers[0].Image).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[1].Image))
			Expect(kbToolsImage).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[2].Image))
			Expect(kbToolsImage).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[3].Image))
			Expect(initSecRenderedToolContainerName).Should(BeEquivalentTo(cfgManagerParams.ToolsContainers[3].Name))
		})
	})

})
