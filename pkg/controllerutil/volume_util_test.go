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

package controllerutil

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("volume util", func() {
	It("preserves PVC dataSourceRef and annotations when converting app templates", func() {
		apiGroup := "dataprotection.kubeblocks.io"
		ref := &corev1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     "Backup",
			Name:     "backup",
		}
		templateAnnotationKey := "example.kubeblocks.io/template"
		pvcts := ToCoreV1PVCTs([]appsv1.PersistentVolumeClaimTemplate{
			{
				Name: "data",
				Annotations: map[string]string{
					templateAnnotationKey: "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: ref,
				},
			},
		})

		Expect(pvcts).Should(HaveLen(1))
		Expect(pvcts[0].Spec.DataSourceRef).Should(Equal(ref))
		Expect(pvcts[0].Annotations[templateAnnotationKey]).Should(Equal("data"))
	})
})
