/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

var _ = Describe("parameter builder", func() {
	It("should work well", func() {
		const (
			clusterName   = "test"
			componentName = "mysql"
			ns            = "default"
		)
		name := core.GenerateComponentParameterName(clusterName, componentName)
		config := NewParameterBuilder(ns, name).
			ClusterRef(clusterName).
			SetComponentParameters(componentName, appsv1.ComponentParameters{
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
