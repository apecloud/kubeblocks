/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("resource Fetcher", func() {
	var compDefObj *appsv1.ComponentDefinition
	var paramDef1, paramDef2 *configv1alpha1.ParametersDefinition

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

		compDefObj = allFieldsCompDefObj(false)
		// clusterObj, _, _ = newAllFieldsClusterObj(compDefObj, false)
	})

	Context("ClassifyParamsFromConfigTemplate", func() {
		It("Should succeed with no error with one paramDef", func() {
			parameters := configv1alpha1.ComponentParameters{
				"param1": pointer.String("value1"),
				"param2": pointer.String("value2"),
			}

			paramItems := ClassifyParamsFromConfigTemplate(parameters, compDefObj, []*configv1alpha1.ParametersDefinition{paramDef1}, map[string]*corev1.ConfigMap{
				configTemplateName: {
					Data: map[string]string{
						"file1": "",
					}}})
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

			paramItems := ClassifyParamsFromConfigTemplate(parameters, compDefObj, []*configv1alpha1.ParametersDefinition{paramDef1, paramDef2}, map[string]*corev1.ConfigMap{
				configTemplateName: {
					Data: map[string]string{
						"file1": "",
						"file2": "",
					}}})
			Expect(paramItems).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveLen(2))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file1"))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file2"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param1"))
			Expect(paramItems[0].ConfigFileParams["file2"].Parameters).Should(HaveKey("param12"))
		})
	})

})
