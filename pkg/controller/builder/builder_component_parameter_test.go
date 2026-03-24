/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var _ = Describe("configuration builder", func() {
	It("should work well", func() {
		const (
			clusterName   = "test"
			componentName = "mysql"
			ns            = "default"
		)
		name := configcore.GenerateComponentConfigurationName(clusterName, componentName)
		config := NewComponentParameterBuilder(ns, name).
			ClusterRef(clusterName).
			Component(componentName).
			AddConfigurationItem(appsv1.ComponentFileTemplate{
				Name: "mysql-config",
			}).
			AddConfigurationItem(appsv1.ComponentFileTemplate{
				Name: "mysql-oteld-config",
			}).
			GetObject()

		Expect(config.Name).Should(BeEquivalentTo(name))
		Expect(config.Spec.ClusterName).Should(BeEquivalentTo(clusterName))
		Expect(config.Spec.ComponentName).Should(BeEquivalentTo(componentName))
		Expect(len(config.Spec.ConfigItemDetails)).Should(Equal(2))
	})
})
