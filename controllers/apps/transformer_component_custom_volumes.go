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

package apps

import (
	"fmt"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentCustomVolumesTransformer handles component custom volumes.
type componentCustomVolumesTransformer struct{}

var _ graph.Transformer = &componentCustomVolumesTransformer{}

func (t *componentCustomVolumesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	cluster := transCtx.Cluster
	compOrig := transCtx.ComponentOrig
	synthesizeComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	if err := doBuildCustomVolumes(synthesizeComp.PodSpec, cluster, synthesizeComp.Name, synthesizeComp.Namespace); err != nil {
		return err
	}

	return nil
}

func newSourceFromResource(name string, source any) corev1.Volume {
	volume := corev1.Volume{
		Name: name,
	}
	switch t := source.(type) {
	default:
		panic(fmt.Sprintf("unknown volume source type: %T", t))
	case *corev1.ConfigMapVolumeSource:
		volume.VolumeSource.ConfigMap = t
	case *corev1.SecretVolumeSource:
		volume.VolumeSource.Secret = t
	}
	return volume
}

func doBuildCustomVolumes(podSpec *corev1.PodSpec, cluster *appsv1alpha1.Cluster, componentName string, namespace string) error {
	comp := cluster.Spec.GetComponentByName(componentName)
	if comp == nil || comp.UserResourceRefs == nil {
		return nil
	}

	volumes := podSpec.Volumes
	for _, configMap := range comp.UserResourceRefs.ConfigMapRefs {
		volumes = append(volumes, newSourceFromResource(configMap.Name, configMap.ConfigMap.DeepCopy()))
	}
	for _, secret := range comp.UserResourceRefs.SecretRefs {
		volumes = append(volumes, newSourceFromResource(secret.Name, secret.Secret.DeepCopy()))
	}
	podSpec.Volumes = volumes
	buildVolumeMountForContainers(podSpec, *comp.UserResourceRefs)
	return nil
}

func buildVolumeMountForContainers(podSpec *corev1.PodSpec, resourceRefs appsv1alpha1.UserResourceRefs) {
	for _, configMap := range resourceRefs.ConfigMapRefs {
		newVolumeMount(podSpec, configMap.ResourceMeta)
	}
	for _, secret := range resourceRefs.SecretRefs {
		newVolumeMount(podSpec, secret.ResourceMeta)
	}
}

func newVolumeMount(podSpec *corev1.PodSpec, res appsv1alpha1.ResourceMeta) {
	for i := range podSpec.Containers {
		container := &podSpec.Containers[i]
		if slices.Contains(res.AsVolumeFrom, container.Name) {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      res.Name,
				MountPath: res.MountPoint,
				SubPath:   res.SubPath,
			})
		}
	}
}
