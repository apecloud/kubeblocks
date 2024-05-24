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

package apps

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func delayUpdateInstanceSetSystemFields(obj v1alpha1.InstanceSetSpec, pobj *v1alpha1.InstanceSetSpec) {
	delayUpdatePodSpecSystemFields(obj.Template.Spec, &pobj.Template.Spec)

	if pobj.RoleProbe != nil && obj.RoleProbe != nil {
		pobj.RoleProbe.FailureThreshold = obj.RoleProbe.FailureThreshold
		pobj.RoleProbe.SuccessThreshold = obj.RoleProbe.SuccessThreshold
	}
}

// delayUpdatePodSpecSystemFields to delay the updating to system fields in pod spec.
func delayUpdatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		delayUpdateKubeBlocksToolsImage(obj.Containers, &pobj.Containers[i])
	}
	for i := range pobj.InitContainers {
		delayUpdateKubeBlocksToolsImage(obj.InitContainers, &pobj.InitContainers[i])
	}
	updateLorryContainer(obj.Containers, pobj.Containers)
}

func updateInstanceSetSystemFields(obj v1alpha1.InstanceSetSpec, pobj *v1alpha1.InstanceSetSpec) {
	updatePodSpecSystemFields(obj.Template.Spec, &pobj.Template.Spec)
	if pobj.RoleProbe != nil && obj.RoleProbe != nil {
		pobj.RoleProbe.FailureThreshold = obj.RoleProbe.FailureThreshold
		pobj.RoleProbe.SuccessThreshold = obj.RoleProbe.SuccessThreshold
	}
}

// updatePodSpecSystemFields to update system fields in pod spec.
func updatePodSpecSystemFields(obj corev1.PodSpec, pobj *corev1.PodSpec) {
	for i := range pobj.Containers {
		updateKubeBlocksToolsImage(&pobj.Containers[i])
	}

	updateLorryContainer(obj.Containers, pobj.Containers)
}

func updateLorryContainer(containers []corev1.Container, pcontainers []corev1.Container) {
	srcLorryContainer := controllerutil.GetLorryContainer(containers)
	dstLorryContainer := controllerutil.GetLorryContainer(pcontainers)
	if srcLorryContainer == nil || dstLorryContainer == nil {
		return
	}
	for i, c := range pcontainers {
		if c.Name == dstLorryContainer.Name {
			pcontainers[i] = *srcLorryContainer.DeepCopy()
			return
		}
	}
}

func delayUpdateKubeBlocksToolsImage(containers []corev1.Container, pc *corev1.Container) {
	if pc.Image != viper.GetString(constant.KBToolsImage) {
		return
	}
	for _, c := range containers {
		if c.Name == pc.Name {
			if getImageName(c.Image) == getImageName(pc.Image) {
				pc.Image = c.Image
			}
			break
		}
	}
}

func updateKubeBlocksToolsImage(pc *corev1.Container) {
	if getImageName(pc.Image) == getImageName(viper.GetString(constant.KBToolsImage)) {
		pc.Image = viper.GetString(constant.KBToolsImage)
	}
}

func getImageName(image string) string {
	subs := strings.Split(image, ":")
	switch len(subs) {
	case 2:
		return subs[0]
	case 3:
		lastIndex := strings.LastIndex(image, ":")
		return image[:lastIndex]
	default:
		return ""
	}
}
