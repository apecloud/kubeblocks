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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

var _ = Describe("workload PVC templates", func() {
	It("preserves PVC dataSourceRef and template annotations", func() {
		ref := dprestore.BackupDataSourceRef("backup")
		pvcs := toPersistentVolumeClaims(&SynthesizedComponent{
			StaticAnnotations:  map[string]string{"static": "true"},
			DynamicAnnotations: map[string]string{"dynamic": "true"},
		}, []corev1.PersistentVolumeClaimTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
					Annotations: map[string]string{
						dptypes.RestoreOptionsAnnotationKey: `{"volumeSource":"data"}`,
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: ref,
				},
			},
		})

		Expect(pvcs).Should(HaveLen(1))
		Expect(pvcs[0].Spec.DataSourceRef).Should(Equal(ref))
		Expect(pvcs[0].Annotations[dptypes.RestoreOptionsAnnotationKey]).Should(Equal(`{"volumeSource":"data"}`))
		Expect(pvcs[0].Annotations["static"]).Should(Equal("true"))
		Expect(pvcs[0].Annotations["dynamic"]).Should(Equal("true"))
	})

	It("propagates dataSourceRef cleanup", func() {
		pvcs := toPersistentVolumeClaims(&SynthesizedComponent{}, []corev1.PersistentVolumeClaimTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec:       corev1.PersistentVolumeClaimSpec{},
			},
		})

		Expect(pvcs).Should(HaveLen(1))
		Expect(pvcs[0].Spec.DataSourceRef).Should(BeNil())
	})
})
