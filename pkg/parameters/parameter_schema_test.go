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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

var _ = Describe("parameter schema status helpers", func() {
	It("detects terminal phases", func() {
		Expect(ParametersDefinitionTerminalPhases(parametersv1alpha1.ParametersDefinitionStatus{
			ObservedGeneration: 2,
			Phase:              parametersv1alpha1.PDAvailablePhase,
		}, 2)).To(BeTrue())
		Expect(ParametersDefinitionTerminalPhases(parametersv1alpha1.ParametersDefinitionStatus{
			ObservedGeneration: 1,
			Phase:              parametersv1alpha1.PDAvailablePhase,
		}, 2)).To(BeFalse())

		Expect(IsParameterFinished(parametersv1alpha1.CFinishedPhase)).To(BeTrue())
		Expect(IsParameterFinished(parametersv1alpha1.CMergeFailedPhase)).To(BeTrue())
		Expect(IsParameterFinished(parametersv1alpha1.CRunningPhase)).To(BeFalse())
		Expect(IsFailedPhase(parametersv1alpha1.CFailedAndPausePhase)).To(BeTrue())
		Expect(IsFailedPhase(parametersv1alpha1.CFinishedPhase)).To(BeFalse())
	})

	It("finds item status and specs by name", func() {
		status := &parametersv1alpha1.ComponentParameterStatus{
			ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{
				{Name: "mysql"},
				{Name: "proxy"},
			},
		}
		spec := &parametersv1alpha1.ComponentParameterSpec{
			ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
				{Name: "mysql"},
				{Name: "proxy"},
			},
		}

		Expect(GetItemStatus(status, "proxy")).To(Equal(&status.ConfigurationItemStatus[1]))
		Expect(GetItemStatus(status, "missing")).To(BeNil())
		Expect(GetConfigTemplateItem(spec, "mysql")).To(Equal(&spec.ConfigItemDetails[0]))
		Expect(GetConfigTemplateItem(spec, "missing")).To(BeNil())
	})
})
