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

package instancetemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("Validation", func() {
	Describe("validateOrdinals", func() {
		It("should validate ordinals successfully", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Instances: []workloads.InstanceTemplate{
						{
							Name: "template1",
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1, 2},
							},
						},
					},
				},
			}
			err := validateOrdinals(its)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail validation for negative ordinals", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Instances: []workloads.InstanceTemplate{
						{
							Name: "template1",
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{-1, 0, 1},
							},
						},
					},
				},
			}
			err := validateOrdinals(its)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ordinal(-1) must >= 0"))
		})

		It("should fail validation for duplicate ordinals", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					DefaultTemplateOrdinals: kbappsv1.Ordinals{
						Discrete: []int32{1},
					},
					Instances: []workloads.InstanceTemplate{
						{
							Name: "template1",
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1},
							},
						},
					},
				},
			}
			err := validateOrdinals(its)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate ordinal(1)"))
		})

		It("should take offlineInstances into consideration", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					DefaultTemplateOrdinals: kbappsv1.Ordinals{
						Discrete: []int32{2},
					},
					OfflineInstances: []string{"instance-1"},
					Instances: []workloads.InstanceTemplate{
						{
							Name: "template1",
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1},
							},
						},
					},
				},
			}
			err := validateOrdinals(its)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ordinal(1) exists in offlineInstances"))
		})
	})

	Describe("ValidateInstanceTemplates", func() {
		It("should validate instance templates successfully", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](2),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1},
							},
						},
						{
							Name:     "template2",
							Replicas: ptr.To[int32](1),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{2},
							},
						},
					},
				},
			}
			tree := &kubebuilderx.ObjectTree{}
			err := ValidateInstanceTemplates(its, tree)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail validation for duplicate template names", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](2),
						},
						{
							Name:     "template1",
							Replicas: ptr.To[int32](1),
						},
					},
				},
			}
			tree := &kubebuilderx.ObjectTree{}
			err := ValidateInstanceTemplates(its, tree)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate instance template name: template1"))
		})

		It("should fail validation when total replicas exceed spec.replicas", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](2),
						},
						{
							Name:     "template2",
							Replicas: ptr.To[int32](2),
						},
					},
				},
			}
			tree := &kubebuilderx.ObjectTree{}
			err := ValidateInstanceTemplates(its, tree)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("total replicas in instances(4) should not greater than replicas in spec(3)"))
		})
	})
})
