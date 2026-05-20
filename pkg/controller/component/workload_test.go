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

package component

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("workload resource defaults", func() {
	AfterEach(func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, "")
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, true)
	})

	newInstanceSet := func() *workloads.InstanceSet {
		return &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "main"},
							{Name: "sidecar"},
						},
						InitContainers: []corev1.Container{
							{Name: "init"},
						},
					},
				},
			},
		}
	}

	It("should not inject zero resources when both flags are disabled", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, false)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":false}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Requests).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(BeNil())
	})

	It("should inject zero for main container via enableZeroResourceForUnset", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, true)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":false}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(BeNil())
	})

	It("should inject zero for sidecar and init containers via clusterDefaultResources.zero", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, false)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		Expect(its.Spec.Template.Spec.Containers[0].Resources.Limits).Should(BeNil())
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.Containers[1].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(its.Spec.Template.Spec.InitContainers[0].Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
	})

	It("should apply configured resources to init and sidecar containers", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, true)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		main := its.Spec.Template.Spec.Containers[0]
		sidecar := its.Spec.Template.Spec.Containers[1]
		initContainer := its.Spec.Template.Spec.InitContainers[0]
		Expect(main.Resources.Requests).Should(BeNil())
		Expect(main.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("0")))
		Expect(main.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("100m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
	})

	It("should let configured resource names override zero by resource name", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, false)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m"},"limits":{}}`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		sidecar := its.Spec.Template.Spec.Containers[1]
		initContainer := its.Spec.Template.Spec.InitContainers[0]
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(sidecar.Resources.Requests).ShouldNot(HaveKey(corev1.ResourceMemory))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
		Expect(initContainer.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("10m")))
		Expect(initContainer.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("0")))
	})

	It("should not override sidecar resource values already set by definitions", func() {
		viper.Set(constant.CfgKeyEnableZeroResourceForUnset, true)
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":true,"requests":{"cpu":"10m","memory":"16Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}`)
		its := newInstanceSet()
		its.Spec.Template.Spec.Containers[1].Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("250m"),
		}

		Expect(setDefaultResourceLimits(its)).Should(Succeed())

		sidecar := its.Spec.Template.Spec.Containers[1]
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
		Expect(sidecar.Resources.Requests).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("16Mi")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")))
		Expect(sidecar.Resources.Limits).Should(HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("64Mi")))
	})

	It("should return an error when cluster default resources are invalid", func() {
		viper.Set(constant.CfgKeyClusterDefaultResources, `{"zero":`)
		its := newInstanceSet()

		Expect(setDefaultResourceLimits(its)).ShouldNot(Succeed())
	})
})
