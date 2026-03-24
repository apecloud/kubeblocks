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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("resource Fetcher", func() {
	var compDefObj *appsv1.ComponentDefinition
	var paramDef1, paramDef2 *configv1alpha1.ParametersDefinition
	var pcr *configv1alpha1.ParamConfigRenderer

	BeforeEach(func() {
		paramDef1 = testparameters.NewParametersDefinitionFactory("param_def1").
			SetConfigFile("file1").
			Schema(`
#Parameter: {
  param1: string
  param2: string
  param3: string
  param4: string
}`).GetObject()
		paramDef2 = testparameters.NewParametersDefinitionFactory("param_def2").
			SetConfigFile("file2").
			Schema(`
#Parameter: {
  param11: string
  param12: string
  param13: string
  param14: string
}`).GetObject()

		pcr = testparameters.NewParamConfigRendererFactory("test").
			SetConfigDescription("file1", configTemplateName, configv1alpha1.FileFormatConfig{Format: configv1alpha1.Ini}).
			SetConfigDescription("file2", configTemplateName, configv1alpha1.FileFormatConfig{Format: configv1alpha1.Ini}).
			GetObject()

		compDefObj = allFieldsCompDefObj(false)
		// clusterObj, _, _ = newAllFieldsClusterObj(compDefObj, false)
	})

	Context("ClassifyParamsFromConfigTemplate", func() {
		It("Should succeed with no error with one paramDef", func() {
			parameters := configv1alpha1.ComponentParameters{
				"param1": pointer.String("value1"),
				"param2": pointer.String("value2"),
			}

			paramItems, err := ClassifyParamsFromConfigTemplate(parameters, compDefObj, []*configv1alpha1.ParametersDefinition{paramDef1}, map[string]*corev1.ConfigMap{
				configTemplateName: {
					Data: map[string]string{
						"file1": "",
					}}}, pcr)
			Expect(err).NotTo(HaveOccurred())
			Expect(paramItems).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file1"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param1"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param2"))
		})

		It("Should succeed with no error with multi paramDef", func() {

			parameters := configv1alpha1.ComponentParameters{
				"param1":  cfgutil.ToPointer("value1"),
				"param12": cfgutil.ToPointer("value2"),
			}

			paramItems, err := ClassifyParamsFromConfigTemplate(parameters, compDefObj, []*configv1alpha1.ParametersDefinition{paramDef1, paramDef2}, map[string]*corev1.ConfigMap{
				configTemplateName: {
					Data: map[string]string{
						"file1": "",
						"file2": "",
					}}}, pcr)
			Expect(err).NotTo(HaveOccurred())
			Expect(paramItems).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveLen(2))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file1"))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file2"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param1"))
			Expect(paramItems[0].ConfigFileParams["file2"].Parameters).Should(HaveKey("param12"))
		})
	})

})
