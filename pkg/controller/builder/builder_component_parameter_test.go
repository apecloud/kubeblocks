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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/ptr"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

var _ = Describe("component parameter builder", func() {
	It("should set component parameter spec fields", func() {
		initial := &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{
				"max_connections": ptr.To("100"),
			},
		}

		obj := NewComponentParameterBuilder("default", "mysql-params").
			SetClusterName("cluster").
			SetCompName("mysql").
			SetInitial(initial).
			GetObject()

		Expect(obj.Namespace).Should(Equal("default"))
		Expect(obj.Name).Should(Equal("mysql-params"))
		Expect(obj.Spec.ClusterName).Should(Equal("cluster"))
		Expect(obj.Spec.ComponentName).Should(Equal("mysql"))
		Expect(obj.Spec.Initial).Should(Equal(initial))
	})
})
