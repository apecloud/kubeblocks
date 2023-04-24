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
