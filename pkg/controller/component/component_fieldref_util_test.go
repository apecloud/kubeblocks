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

package component

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentRef Fields Tests", func() {
	const clusterDefName = "test-clusterdef"
	const clusterName = "test-cluster"
	const clusterVersionName = "test-clusterversion"
	const clusterNamespace = "test-compref"
	const mysqlCompDefName = "mysql-def"
	const referredCompDefName = "maxscale-def"

	const mysqlCompName = "mysql"
	const referredCompName = "maxscale"

	Context("test fieldRef", func() {
		var clusterDefBuilder *testapps.MockClusterDefFactory
		var clusterBuilder *testapps.MockClusterFactory

		BeforeEach(func() {
			By("create cluster definition builder")
			clusterDefBuilder = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, referredCompDefName)

			// add one mysql component
			clusterDefBuilder = clusterDefBuilder.AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName)

			By("create cluste builder")
			clusterBuilder = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName)
		})

		It("test fieldref", func() {
			clusterDef := clusterDefBuilder.GetObject()

			By("add one component to cluster")
			clusterBuilder = clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).AddComponent(referredCompName, referredCompDefName)
			cluster := clusterBuilder.GetObject()

			componentDef := clusterDef.GetComponentDefByName(referredCompDefName)
			Expect(componentDef).NotTo(BeNil())
			components := cluster.Spec.GetDefNameMappingComponents()[referredCompDefName]
			Expect(len(components)).To(Equal(1))

			By("lookup component name, should success")
			valueFrom := &appsv1alpha1.ComponentValueFrom{
				Type:      appsv1alpha1.FromFieldRef,
				FieldPath: "$.components[0].name",
			}
			value, err := resolveFieldRef(valueFrom, components, componentDef)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(referredCompName))

			By("lookup componentSpec name, should success")
			valueFrom = &appsv1alpha1.ComponentValueFrom{
				Type:      appsv1alpha1.FromFieldRef,
				FieldPath: "$.componentDef.name",
			}
			value, err = resolveFieldRef(valueFrom, components, componentDef)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(referredCompDefName))

			By("invalid json path, should fail")
			valueFrom = &appsv1alpha1.ComponentValueFrom{
				Type:      appsv1alpha1.FromFieldRef,
				FieldPath: "$.invalidField.name",
			}
			_, err = resolveFieldRef(valueFrom, components, componentDef)
			Expect(err).ShouldNot(BeNil())
		})

		It("test invalid serviceRef with no service", func() {
			clusterDef := clusterDefBuilder.GetObject()

			By("add one component to cluster")
			clusterBuilder = clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).AddComponent(referredCompName, referredCompDefName)
			cluster := clusterBuilder.GetObject()

			componentDef := clusterDef.GetComponentDefByName(referredCompDefName)
			Expect(componentDef).NotTo(BeNil())
			components := cluster.Spec.GetDefNameMappingComponents()[referredCompDefName]
			Expect(len(components)).To(Equal(1))

			By("lookup service name, should fail")
			_, err := resolveServiceRef(cluster.Name, components, componentDef)
			if componentDef.Service != nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring("does not have service"))
			}
		})

		It("test invalid serviceRef with multiple components", func() {
			clusterDef := clusterDefBuilder.GetObject()

			By("add one component to cluster")
			clusterBuilder = clusterBuilder.
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).
				// add component one more time
				AddComponent("oneMoreComp", referredCompDefName)
			cluster := clusterBuilder.GetObject()

			componentDef := clusterDef.GetComponentDefByName(referredCompDefName)
			Expect(componentDef).NotTo(BeNil())
			components := cluster.Spec.GetDefNameMappingComponents()[referredCompDefName]
			Expect(len(components)).To(Equal(2))

			By("lookup service name, should fail")
			_, err := resolveServiceRef(cluster.Name, components, componentDef)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("expect one component but got"))
		})

		It("test serviceRef with correct setting", func() {
			clusterDef := clusterDefBuilder.AddNamedServicePort("mysql", 3306).GetObject()

			By("add one component to cluster")
			clusterBuilder = clusterBuilder.
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName)
			cluster := clusterBuilder.GetObject()

			componentDef := clusterDef.GetComponentDefByName(referredCompDefName)
			Expect(componentDef).NotTo(BeNil())
			components := cluster.Spec.GetDefNameMappingComponents()[referredCompDefName]
			Expect(len(components)).To(Equal(1))

			By("lookup service name, should fail")
			value, err := resolveServiceRef(cluster.Name, components, componentDef)
			Expect(err).To(BeNil())
			Expect(value).To(Equal(fmt.Sprintf("%s-%s", cluster.Name, referredCompName)))
		})

		It("test headlessServiceSvc", func() {
			clusterDef := clusterDefBuilder.GetObject()

			By("add one component to cluster")
			var replicas int32 = 3
			clusterBuilder = clusterBuilder.
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).SetReplicas(replicas)
			cluster := clusterBuilder.GetObject()

			componentDef := clusterDef.GetComponentDefByName(referredCompDefName)
			Expect(componentDef).NotTo(BeNil())

			components := cluster.Spec.GetDefNameMappingComponents()[referredCompDefName]
			Expect(len(components)).To(Equal(1))

			By("construct headless service name")
			valueFrom := &appsv1alpha1.ComponentValueFrom{
				Type:     appsv1alpha1.FromHeadlessServiceRef,
				Format:   "",
				JoinWith: "",
			}

			value := resolveHeadlessServiceFieldRef(valueFrom, cluster, components)
			addrs := strings.Split(value, ",")
			Expect(len(addrs)).To(Equal(int(replicas)))
			for i, addr := range addrs {
				Expect(addr).To(Equal(fmt.Sprintf("%s-%s-%d.%s-%s-headless.%s.svc", cluster.Name, referredCompName, i, cluster.Name, referredCompName, cluster.Namespace)))
			}
		})
	})
})
