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

package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hashicorp/go-version"
)

var _ = Describe("version", func() {
	It("compare version", func() {
		v1, err := version.NewVersion("20.10.0")
		Expect(err).Should(Succeed())
		v2, err := version.NewVersion("20.10.5")
		Expect(err).Should(Succeed())
		v3, err := version.NewVersion("20.10.24")
		Expect(err).Should(Succeed())

		Expect(v1.LessThan(MinimumDockerVersion)).Should(BeTrue())
		Expect(v2.LessThan(MinimumDockerVersion)).Should(BeFalse())
		Expect(v3.LessThan(MinimumDockerVersion)).Should(BeFalse())
	})
})
