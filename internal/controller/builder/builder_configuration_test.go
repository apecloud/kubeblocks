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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/configuration/core"
)

var _ = Describe("configuration builder", func() {
	It("should work well", func() {
		const (
			clusterName    = "test"
			componentName  = "mysql"
			clusterDefName = "mysql-cd"
			clusterVerName = "mysql-cv"
			ns             = "default"
		)
		name := core.GenerateComponentConfigurationName(clusterName, componentName)
		config := NewConfigurationBuilder(ns, name).
			ClusterRef(clusterName).
			Component(componentName).
			ClusterDefRef(clusterDefName).
			ClusterVerRef(clusterVerName).
			AddConfigurationItem("mysql-config").
			AddConfigurationItem("mysql-oteld-config").
			GetObject()

		Expect(config.Name).Should(BeEquivalentTo(name))
		Expect(config.Spec.ClusterRef).Should(BeEquivalentTo(clusterName))
		Expect(config.Spec.ComponentName).Should(BeEquivalentTo(componentName))
		Expect(config.Spec.ClusterVersionRef).Should(BeEquivalentTo(clusterVerName))
		Expect(config.Spec.ClusterDefRef).Should(BeEquivalentTo(clusterDefName))
		Expect(len(config.Spec.ConfigItemDetails)).Should(Equal(2))
	})
})
