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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("volume util", func() {
	It("preserves PVC dataSourceRef and annotations when converting app templates", func() {
		apiGroup := "example.kubeblocks.io"
		ref := &corev1.TypedObjectReference{
			APIGroup: &apiGroup,
			Kind:     "ExampleSource",
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

	It("uses explicit and configured default storage class when converting app templates", func() {
		defer viper.Set(constant.CfgKeyDefaultStorageClass, "")
		explicit := "fast"
		viper.Set(constant.CfgKeyDefaultStorageClass, "standard")

		pvcts := ToCoreV1PVCTs([]appsv1.PersistentVolumeClaimTemplate{
			{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &explicit}},
			{Name: "log"},
		})

		Expect(pvcts).Should(HaveLen(2))
		Expect(*pvcts[0].Spec.StorageClassName).Should(Equal("fast"))
		Expect(*pvcts[1].Spec.StorageClassName).Should(Equal("standard"))
	})

	It("creates missing volumes and composes pvc names", func() {
		volumes := CreateVolumeIfNotExist(nil, "config", func(volumeName string) corev1.Volume {
			return corev1.Volume{Name: volumeName}
		})
		Expect(volumes).Should(HaveLen(1))
		volumes = CreateVolumeIfNotExist(volumes, "config", func(volumeName string) corev1.Volume {
			return corev1.Volume{Name: volumeName + "-new"}
		})
		Expect(volumes).Should(HaveLen(1))

		template := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
		Expect(ComposePVCName(template, "its", "its-0")).Should(Equal("data-its-0"))
		template.Annotations = map[string]string{constant.PVCNamePrefixAnnotationKey: "custom"}
		Expect(ComposePVCName(template, "its", "its-0")).Should(Equal("custom-0"))
		Expect(ComposePVCName(template, "its", "other")).Should(Equal("custom"))
	})
})
