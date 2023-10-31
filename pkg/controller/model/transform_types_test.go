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

package model

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("transform types test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("FindX function", func() {
		It("should work well", func() {
			root := builder.NewStatefulSetBuilder(namespace, name).GetObject()
			vertex := &ObjectVertex{Obj: root}
			Expect(vertex.String()).Should(Equal("{obj:*v1.StatefulSet, name: bar, action: nil}"))
			vertex.Action = ActionCreatePtr()
			Expect(vertex.String()).Should(Equal("{obj:*v1.StatefulSet, name: bar, action: CREATE}"))
		})
	})
})
