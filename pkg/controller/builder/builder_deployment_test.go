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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("pdb builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)

		commonLabels := map[string]string{
			constant.AppManagedByLabelKey: constant.AppName,
			constant.AppNameLabelKey:      "apecloudoteld",
			constant.AppInstanceLabelKey:  "apecloudoteld",
		}

		labelSelector := &metav1.LabelSelector{
			MatchLabels: commonLabels,
		}

		podTemplate := corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: commonLabels,
			},
			Spec: NewPodBuilder("", "").
				AddServiceAccount("oteld-controller").
				AddContainer(corev1.Container{}).
				AddVolumes(corev1.Volume{
					Name: "oteldlog",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/log/oteld",
							Type: cfgutil.ToPointer(corev1.HostPathDirectoryOrCreate),
						}},
				}).
				AddVolumes(corev1.Volume{
					Name: "root",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{Path: "/"}},
				}).
				SetSecurityContext(corev1.PodSecurityContext{
					RunAsUser:    cfgutil.ToPointer(int64(0)),
					RunAsGroup:   cfgutil.ToPointer(int64(0)),
					FSGroup:      cfgutil.ToPointer(int64(65534)),
					RunAsNonRoot: cfgutil.ToPointer(false),
				}).
				GetObject().Spec,
		}

		deployment := NewDeploymentBuilder(ns, name).
			SetTemplate(podTemplate).
			AddLabelsInMap(commonLabels).
			AddMatchLabelsInMap(commonLabels).
			SetSelector(labelSelector).
			GetObject()

		Expect(deployment.Name).Should(BeEquivalentTo(name))
		Expect(deployment.Namespace).Should(BeEquivalentTo(ns))
		Expect(deployment.Spec.Template).Should(BeEquivalentTo(podTemplate))
		Expect(deployment.Spec.Selector.MatchLabels).Should(BeEquivalentTo(commonLabels))
		Expect(deployment.Labels).Should(BeEquivalentTo(commonLabels))
	})
})
