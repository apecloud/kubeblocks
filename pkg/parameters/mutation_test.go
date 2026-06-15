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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

var _ = Describe("component parameter mutation", func() {
	It("merges labels, owners, additions, deletions, and updated items", func() {
		expected := &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"expected": "true",
					"shared":   "expected",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps.kubeblocks.io/v1",
					Kind:       "Cluster",
					Name:       "mysql",
					UID:        "uid",
				}},
			},
			Spec: parametersv1alpha1.ComponentParameterSpec{
				ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
					{Name: "keep"},
					{Name: "add"},
				},
			},
		}
		existing := &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"existing": "true",
					"shared":   "existing",
				},
			},
			Spec: parametersv1alpha1.ComponentParameterSpec{
				ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
					{Name: "keep"},
					{Name: "delete"},
				},
			},
		}

		updated := MergeComponentParameter(expected, existing, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
			dest.Payload = parametersv1alpha1.Payload{
				"source": []byte(`"mutated"`),
			}
		})

		Expect(updated).NotTo(BeIdenticalTo(existing))
		Expect(updated.Labels).To(Equal(map[string]string{
			"expected": "true",
			"existing": "true",
			"shared":   "expected",
		}))
		Expect(updated.OwnerReferences).To(Equal(expected.OwnerReferences))
		Expect(updated.Spec.ConfigItemDetails).To(HaveLen(2))
		Expect(updated.Spec.ConfigItemDetails[0].Name).To(Equal("keep"))
		Expect(updated.Spec.ConfigItemDetails[0].Payload).To(HaveKey("source"))
		Expect(updated.Spec.ConfigItemDetails[1].Name).To(Equal("add"))
	})

	It("keeps existing owners when expected owners are empty", func() {
		existing := &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{Name: "existing-owner"}},
			},
		}

		updated := MergeComponentParameter(&parametersv1alpha1.ComponentParameter{}, existing, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {})
		Expect(updated.OwnerReferences).To(Equal(existing.OwnerReferences))
		Expect(updated.Spec.ConfigItemDetails).To(BeEmpty())
	})
})
