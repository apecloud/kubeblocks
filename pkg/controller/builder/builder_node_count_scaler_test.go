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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("node_count_scaler builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)
		clusterName := "target-cluster-name"
		componentNames := []string{"comp-1", "comp-2"}

		ncs := NewNodeCountScalerBuilder(ns, name).
			SetTargetClusterName(clusterName).
			SetTargetComponentNames(componentNames).
			GetObject()

		Expect(ncs.Name).Should(Equal(name))
		Expect(ncs.Namespace).Should(Equal(ns))
		Expect(ncs.Spec.TargetClusterName).Should(Equal(clusterName))
		Expect(ncs.Spec.TargetComponentNames).Should(Equal(componentNames))
	})
})
