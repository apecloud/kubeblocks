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

var _ = Describe("parameter metadata helpers", func() {
	It("finds config descriptions by name and template", func() {
		configs := []parametersv1alpha1.ComponentConfigDescription{
			{Name: "mysql.cnf", TemplateName: "mysql-template"},
			{Name: "proxy.cnf", TemplateName: "proxy-template"},
			{Name: "mysql-extra.cnf", TemplateName: "mysql-template"},
		}

		Expect(GetComponentConfigDescription(configs, "mysql.cnf")).To(Equal(&configs[0]))
		Expect(GetComponentConfigDescription(configs, "missing.cnf")).To(BeNil())
		Expect(GetComponentConfigDescriptions(configs, "mysql-template")).To(Equal([]parametersv1alpha1.ComponentConfigDescription{
			configs[0],
			configs[2],
		}))
		Expect(GetComponentConfigDescriptions(configs, "missing-template")).To(BeEmpty())
		Expect(HasValidParameterTemplate(configs)).To(BeTrue())
		Expect(HasValidParameterTemplate(nil)).To(BeFalse())
	})
})
