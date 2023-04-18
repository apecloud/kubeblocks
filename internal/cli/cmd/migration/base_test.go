/*
Copyright ApeCloud, Inc.

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

package migration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
)

var _ = Describe("base", func() {

	Context("Basic function validate", func() {

		It("CliStepChangeToStructure", func() {
			resultMap, resultKeyArr := CliStepChangeToStructure()
			Expect(len(resultMap)).Should(Equal(4))
			Expect(len(resultKeyArr)).Should(Equal(4))
		})

		It("BuildInitializationStepsOrder", func() {
			task := &v1alpha1.MigrationTask{
				Spec: v1alpha1.MigrationTaskSpec{
					Initialization: v1alpha1.InitializationConfig{
						Steps: []v1alpha1.StepEnum{
							v1alpha1.StepFullLoad,
							v1alpha1.StepStructPreFullLoad,
						},
					},
				},
			}
			template := &v1alpha1.MigrationTemplate{
				Spec: v1alpha1.MigrationTemplateSpec{
					Initialization: v1alpha1.InitializationModel{
						Steps: []v1alpha1.StepModel{
							{Step: v1alpha1.StepStructPreFullLoad},
							{Step: v1alpha1.StepFullLoad},
						},
					},
				},
			}
			arr := BuildInitializationStepsOrder(task, template)
			Expect(len(arr)).Should(Equal(2))
			Expect(arr[0]).Should(Equal(v1alpha1.StepStructPreFullLoad.CliString()))
			Expect(arr[1]).Should(Equal(v1alpha1.StepFullLoad.CliString()))
		})
	})

})
