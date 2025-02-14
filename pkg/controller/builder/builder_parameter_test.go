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
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

var _ = Describe("parameter builder", func() {
	It("should work well", func() {
		const (
			clusterName   = "test"
			componentName = "mysql"
			ns            = "default"
		)
		name := core.GenerateComponentConfigurationName(clusterName, componentName)
		config := NewParameterBuilder(ns, name).
			ClusterRef(clusterName).
			SetComponentParameters(componentName, parametersv1alpha1.ComponentParameters{
				"param1": pointer.String("value1"),
				"param2": pointer.String("value2"),
			}).
			AddCustomTemplate(componentName, "mysql-config", appsv1.ConfigTemplateExtension{
				TemplateRef: "mysql-config-tpl",
				Namespace:   "default",
			}).
			AddCustomTemplate(componentName, "mysql-config2", appsv1.ConfigTemplateExtension{
				TemplateRef: "mysql-config-tpl2",
				Namespace:   "default",
			}).
			GetObject()

		Expect(config.Name).Should(BeEquivalentTo(name))
		Expect(config.Spec.ClusterName).Should(BeEquivalentTo(clusterName))
		Expect(config.Spec.ComponentParameters).Should(HaveLen(1))
		Expect(config.Spec.ComponentParameters[0].ComponentName).Should(BeEquivalentTo(componentName))
		Expect(config.Spec.ComponentParameters[0].Parameters).Should(HaveLen(2))
		Expect(config.Spec.ComponentParameters[0].CustomTemplates).Should(HaveLen(2))
	})
})
