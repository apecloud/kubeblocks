/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("ConfigUtils", func() {
	Context("build config manager into lorry", func() {
		const (
			namespace                     = "namespace"
			databaseVolume                = "database-volume"
			databaseTpl                   = "database-tpl"
			databaseComponentTemplateSpec = "database-component-template-spec"
			databaseConfigConstraint      = "database-config-constraint"
			databaseConfigMountPath       = "database-config-mount-path"

			proxyVolume                = "proxy-volume"
			proxyTpl                   = "proxy-tpl"
			proxyComponentTemplateSpec = "proxy-component-template-spec"
			proxyConfigConstraint      = "proxy-config-constraint"
			proxyConfigMountPath       = "proxy-config-mount-path"
		)

		var component *SynthesizedComponent

		BeforeEach(func() {
			component = &SynthesizedComponent{}
			component.ConfigTemplates = []appsv1alpha1.ComponentConfigSpec{
				{
					ConfigConstraintRef: databaseConfigConstraint,
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:        databaseComponentTemplateSpec,
						Namespace:   namespace,
						VolumeName:  databaseVolume,
						TemplateRef: databaseTpl,
					},
				},
				{
					ConfigConstraintRef: proxyConfigConstraint,
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:        proxyComponentTemplateSpec,
						Namespace:   namespace,
						VolumeName:  proxyVolume,
						TemplateRef: proxyTpl,
					},
				},
			}

			component.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      databaseVolume,
								MountPath: databaseConfigMountPath,
							},
						},
					},
					{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      proxyVolume,
								MountPath: proxyConfigMountPath,
							},
						},
					},
				},
			}
		})

		It("can get using volumes by configSpecs", func() {
			volumeDirs, usingConfigSpecs := getUsingVolumesByConfigSpecs(component.PodSpec, component.ConfigTemplates)
			Expect(len(volumeDirs)).To(Equal(2))
			Expect(len(usingConfigSpecs)).To(Equal(2))
		})

		It("can get support reload configSpecs", func() {

		})
	})
})
