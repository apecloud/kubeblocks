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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
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
			AddConfigurationItem(appsv1.ComponentTemplateSpec{
				Name: "mysql-config",
			}).
			AddConfigurationItem(appsv1.ComponentTemplateSpec{
				Name: "mysql-oteld-config",
			}).
			GetObject()

		Expect(config.Name).Should(BeEquivalentTo(name))
		Expect(config.Spec.ClusterName).Should(BeEquivalentTo(clusterName))
		Expect(config.Spec.ComponentName).Should(BeEquivalentTo(componentName))
		Expect(len(config.Spec.ConfigItemDetails)).Should(Equal(2))
	})
})
