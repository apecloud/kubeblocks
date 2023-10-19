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

package preflight

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("load_spec_test", func() {
	It("LoadPreflightSpec test, and expect success", func() {
		yamlCheckFiles := []string{"../cli/testing/testdata/preflight.yaml", "../cli/testing/testdata/hostpreflight.yaml"}
		Eventually(func(g Gomega) {
			preflightSpec, hostPreflightSpec, preflightName, err := LoadPreflightSpec(yamlCheckFiles, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(len(preflightSpec.Spec.Analyzers)).Should(Equal(1))
			g.Expect(len(hostPreflightSpec.Spec.Analyzers)).Should(Equal(1))
			g.Expect(preflightName).NotTo(BeNil())
		}).Should(Succeed())
	})
})
