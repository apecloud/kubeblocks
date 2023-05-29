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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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

	// var clusterDefObj *appsv1alpha1.ClusterDefinition
	// var component *SynthesizedComponent
	// var clusterCompSpec *appsv1alpha1.ClusterComponentSpec
	// var clusterCompDef *appsv1alpha1.ClusterComponentDefinition
	int32Ptr := func(i int32) *int32 {
		return &i
	}

	Context("test getReferredComponent", func() {
		var clusterDefBuilder *testapps.MockClusterDefFactory
		var clusterBuilder *testapps.MockClusterFactory

		BeforeEach(func() {
			By("create cluster definition")
			clusterDefBuilder = testapps.NewClusterDefFactory(clusterDefName)
			// add one component definition
			clusterDefBuilder = clusterDefBuilder.AddComponentDef(testapps.StatefulMySQLComponent, referredCompDefName)
			// add one mysql component
			clusterDefBuilder = clusterDefBuilder.AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName)

			By("create cluster")
			clusterBuilder = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName)
		})

		It("get component from empty cluster, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(BeEmpty())
			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, "", referredCompName)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})

		It("get component with invalid name, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompDefName, mysqlCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(1))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, "", "some-invalid-name")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})

		It("get component with valid name, should pass", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(2))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, "", referredCompName)
			Expect(err).To(BeNil())
			Expect(compSpec).NotTo(BeNil())
			Expect(compDefSpec).NotTo(BeNil())
			Expect(compSpec.Name).To(Equal(referredCompName))
			Expect(compDefSpec.Name).To(Equal(referredCompDefName))
		})

		It("get component with invalid componentDef name, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(2))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, "some-invalid-comp", "")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})

		It("get component with valid componentDef name, should pass", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(2))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, referredCompDefName, "")
			Expect(err).To(BeNil())
			Expect(compSpec).NotTo(BeNil())
			Expect(compDefSpec).NotTo(BeNil())
			Expect(compSpec.Name).To(Equal(referredCompName))
			Expect(compDefSpec.Name).To(Equal(referredCompDefName))
		})

		It("get component with inconsistent componentDef and comp name, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(2))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, referredCompDefName, mysqlCompName)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not match"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})

		It("get component with consistent componentDef and comp name, should pass", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(2))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, referredCompDefName, referredCompName)
			Expect(err).To(BeNil())
			Expect(compSpec).NotTo(BeNil())
			Expect(compDefSpec).NotTo(BeNil())
			Expect(compSpec.Name).To(Equal(referredCompName))
			Expect(compDefSpec.Name).To(Equal(referredCompDefName))
		})

		It("get component with multi-occurrence componentDef name, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).
				// add another component with same componentDef name
				AddComponent("some-dup-comp", referredCompDefName).GetObject()
			Expect(cluster.Spec.ComponentSpecs).To(HaveLen(3))

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, referredCompDefName, "")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("found multiple components"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})

		It("get component with multi-occurrence componentDef name, should fail", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.AddComponent(mysqlCompName, mysqlCompDefName).
				AddComponent(referredCompName, referredCompDefName).GetObject()

			compSpec, compDefSpec, err := getReferredComponent(clusterDef, cluster, "", "")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).Should(ContainSubstring("must specify either componentName or componentDefName"))
			Expect(compSpec).To(BeNil())
			Expect(compDefSpec).To(BeNil())
		})
	})

	Context("test service port", func() {
		It("search test port", func() {
			portName := "test-port"
			port := int32(3306)
			mockSvcSpec := &appsv1alpha1.ServiceSpec{
				Ports: []appsv1alpha1.ServicePort{
					{
						Name: portName,
						Port: port,
					},
				},
			}
			svcPort, err := getServicePort(mockSvcSpec, portName)
			Expect(err).To(BeNil())
			Expect(svcPort.Port).To(Equal(port))

			svcPort, err = getServicePort(mockSvcSpec, "invalid-port-name")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(svcPort).To(BeNil())
		})
	})

	Context("test extract field path", func() {

		It("test extract field path", func() {
			compSpec := &appsv1alpha1.ClusterComponentSpec{}
			By("get invalid path, should fail")
			_, err := extractFieldPathAsString(compSpec, "some-path")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("not supported"))

			By("get primary index, should fail")
			_, err = extractFieldPathAsString(compSpec, "primaryIndex")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("primaryIndex not set"))

			By("get primary index 0, should pass")
			compSpec.PrimaryIndex = int32Ptr(0)
			path, err := extractFieldPathAsString(compSpec, "primaryIndex")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("0"))

			By("get replicas, should pass")
			path, err = extractFieldPathAsString(compSpec, "replicas")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("0"))

			compSpec.Replicas = 3
			path, err = extractFieldPathAsString(compSpec, "replicas")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("3"))

			By("get name, should pass")
			compSpec.Name = "test-name"
			path, err = extractFieldPathAsString(compSpec, "name")
			Expect(err).To(BeNil())
			Expect(path).To(Equal("test-name"))
		})
	})
	Context("test resource field path", func() {
		var comp *appsv1alpha1.ClusterComponentSpec

		BeforeEach(func() {
			// prepare component spec
			comp = &appsv1alpha1.ClusterComponentSpec{
				PrimaryIndex: int32Ptr(0),
				Replicas:     3,
				Name:         "test-name",
			}
		})

		type reousrceTestCase struct {
			fields   string
			expected string
		}

		It("test regular resources", func() {
			comp.Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			}

			testCases := []reousrceTestCase{
				{
					fields:   "limits.cpu",
					expected: "1",
				},
				{
					fields:   "limits.memory",
					expected: "1073741824",
				},
				{
					fields:   "requests.cpu",
					expected: "1",
				},
				{
					fields:   "requests.memory",
					expected: "536870912",
				},
			}

			resourceFieldRef := &appsv1alpha1.ComponentResourceFieldRef{
				EnvName: "test-container",
			}
			for _, test := range testCases {
				resourceFieldRef.Resource = test.fields
				path, err := extractComponentResourceValue(resourceFieldRef, comp)
				Expect(err).To(BeNil())
				Expect(path).To(Equal(test.expected))
			}
		})

		It("test hugepage resources", func() {
			comp.Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceHugePagesPrefix + "2Mi": resource.MustParse("1"),
					corev1.ResourceHugePagesPrefix + "1Gi": resource.MustParse("2Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceHugePagesPrefix + "2Mi": resource.MustParse("0.5"),
					corev1.ResourceHugePagesPrefix + "1Gi": resource.MustParse("1Gi"),
				},
			}

			testCases := []reousrceTestCase{
				{
					fields:   "limits." + corev1.ResourceHugePagesPrefix + "2Mi",
					expected: "1",
				},
				{
					fields:   "limits." + corev1.ResourceHugePagesPrefix + "1Gi",
					expected: "2147483648",
				},
				{
					fields:   "requests." + corev1.ResourceHugePagesPrefix + "2Mi",
					expected: "1",
				},
				{
					fields:   "requests." + corev1.ResourceHugePagesPrefix + "1Gi",
					expected: "1073741824",
				},
			}

			resourceFieldRef := &appsv1alpha1.ComponentResourceFieldRef{
				EnvName: "test-container",
			}
			for _, test := range testCases {
				resourceFieldRef.Resource = test.fields
				path, err := extractComponentResourceValue(resourceFieldRef, comp)
				Expect(err).To(BeNil())
				Expect(path).To(Equal(test.expected))
			}
		})
	})

	Context("test buildComponentEnv", func() {
		var clusterDefBuilder *testapps.MockClusterDefFactory
		var clusterBuilder *testapps.MockClusterFactory
		var componentRef *appsv1alpha1.ComponentRef

		BeforeEach(func() {
			By("create cluster definition")
			clusterDefBuilder = testapps.NewClusterDefFactory(clusterDefName)
			// add one component definition
			clusterDefBuilder = clusterDefBuilder.AddComponentDef(testapps.StatefulMySQLComponent, referredCompDefName)
			clusterDefBuilder = clusterDefBuilder.AddNamedServicePort("maxscale", 3306)
			// add one mysql component definition
			clusterDefBuilder = clusterDefBuilder.AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName)

			By("create cluster")
			clusterBuilder = testapps.NewClusterFactory(clusterNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(mysqlCompName, mysqlCompDefName)
			clusterBuilder = clusterBuilder.AddComponent(referredCompName, referredCompDefName).SetResources(corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			})

			componentRef = &appsv1alpha1.ComponentRef{
				ComponentDefName: referredCompDefName,
				FieldRefs: []*appsv1alpha1.ComponentFieldRef{
					{
						EnvName:   "MAXSCALE_REPLICAS",
						FieldPath: "replicas",
					},
					{
						EnvName:   "MAXSCALE_COMP_NAME",
						FieldPath: "name",
					},
				},
				ServiceRefs: []*appsv1alpha1.ComponentServiceRef{
					{
						EnvNamePrefix: "MAXSCALE_SVC",
						ServiceName:   "maxscale",
					},
				},
				ResourceFieldRefs: []*appsv1alpha1.ComponentResourceFieldRef{
					{
						EnvName:  "MAXSCALE_CPU_LIMIT",
						Resource: "limits.cpu",
					},
				},
			}
		})

		It("build componentRef info from ClusterDef", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.GetObject()
			component := &SynthesizedComponent{}

			clusterCompDef := clusterDef.GetComponentDefByName(mysqlCompDefName)
			Expect(clusterCompDef).NotTo(BeNil())
			Expect(clusterCompDef.ComponentRef).To(HaveLen(0))
			clusterComp := cluster.Spec.GetComponentByName(mysqlCompName)
			Expect(clusterComp).NotTo(BeNil())
			Expect(clusterComp.ComponentRef).To(HaveLen(0))

			// append component ref to cluster definition
			clusterCompDef.ComponentRef = append(clusterCompDef.ComponentRef, componentRef)
			err := buildCompoentRef(clusterDef, cluster, clusterCompDef, clusterComp, component)
			Expect(err).To(BeNil())
			Expect(component.ComponentRefEnvs).NotTo(BeEmpty())
			Expect(component.ComponentRefEnvs).To(HaveLen(6))

		})

		It("build componentRef info from ClusterComponent", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.GetObject()
			component := &SynthesizedComponent{}

			clusterCompDef := clusterDef.GetComponentDefByName(mysqlCompDefName)
			Expect(clusterCompDef).NotTo(BeNil())
			Expect(clusterCompDef.ComponentRef).To(HaveLen(0))
			clusterComp := cluster.Spec.GetComponentByName(mysqlCompName)
			Expect(clusterComp).NotTo(BeNil())
			Expect(clusterComp.ComponentRef).To(HaveLen(0))

			// append component ref to cluster definition
			clusterComp.ComponentRef = append(clusterComp.ComponentRef, componentRef)
			err := buildCompoentRef(clusterDef, cluster, clusterCompDef, clusterComp, component)
			Expect(err).To(BeNil())
			Expect(component.ComponentRefEnvs).NotTo(BeEmpty())
			Expect(component.ComponentRefEnvs).To(HaveLen(6))
		})

		It("build componentRef info overrides ClusterCompDef", func() {
			clusterDef := clusterDefBuilder.GetObject()
			cluster := clusterBuilder.GetObject()
			component := &SynthesizedComponent{}

			clusterCompDef := clusterDef.GetComponentDefByName(mysqlCompDefName)
			Expect(clusterCompDef).NotTo(BeNil())
			Expect(clusterCompDef.ComponentRef).To(HaveLen(0))
			clusterComp := cluster.Spec.GetComponentByName(mysqlCompName)
			Expect(clusterComp).NotTo(BeNil())
			Expect(clusterComp.ComponentRef).To(HaveLen(0))

			clusterCompDef.ComponentRef = append(clusterCompDef.ComponentRef, componentRef.DeepCopy())
			// append component ref to cluster definition
			compRefCopy := componentRef.DeepCopy()
			compRefCopy.ResourceFieldRefs = append(compRefCopy.ResourceFieldRefs, &appsv1alpha1.ComponentResourceFieldRef{
				EnvName:  "MAXSCALE_CPU_REQUEST",
				Resource: "requests.cpu",
			})
			compRefCopy.FieldRefs = nil
			compRefCopy.ServiceRefs = nil
			clusterComp.ComponentRef = append(clusterComp.ComponentRef, compRefCopy)

			err := buildCompoentRef(clusterDef, cluster, clusterCompDef, clusterComp, component)
			Expect(err).To(BeNil())
			Expect(component.ComponentRefEnvs).To(HaveLen(2))
		})
	})
})
