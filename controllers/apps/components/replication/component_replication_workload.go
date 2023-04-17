/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package replication

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type replicationComponentWorkloadBuilder struct {
	internal.ComponentWorkloadBuilderBase
	workloads []*appsv1.StatefulSet
}

func (b *replicationComponentWorkloadBuilder) MutableWorkload(idx int32) client.Object {
	return b.workloads[idx]
}

func (b *replicationComponentWorkloadBuilder) BuildService() internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0, len(svcList))
		for _, svc := range svcList {
			svc.Spec.Selector[constant.RoleLabelKey] = string(Primary)
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *replicationComponentWorkloadBuilder) BuildWorkload(idx int32) internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		component := b.Comp.GetSynthesizedComponent()
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build replication workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), component.Name)
		}

		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), component, b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}

		// sts.Name renamed with suffix "-<sts-idx>" for subsequent sts workload
		if idx != 0 {
			sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, idx)
		}
		if idx == component.GetPrimaryIndex() {
			sts.Labels[constant.RoleLabelKey] = string(Primary)
		} else {
			sts.Labels[constant.RoleLabelKey] = string(Secondary)
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.workloads = append(b.workloads, sts)

		return nil, nil // don't return sts here
	}
	return b.BuildWrapper(buildfn)
}

func (b *replicationComponentWorkloadBuilder) BuildVolume(idx int32) internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.MutableWorkload(idx)
		// if workload == nil {
		// 	return nil, fmt.Errorf("build replication volumes but workload is nil, cluster: %s, component: %s",
		// 		b.comp.GetClusterName(), b.comp.GetName())
		// }

		component := b.Comp.GetSynthesizedComponent()
		sts := workload.(*appsv1.StatefulSet)
		objs := make([]client.Object, 0)

		// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
		pvcMap := GeneratePVCFromVolumeClaimTemplates(sts, component.VolumeClaimTemplates)
		for pvcTplName, pvc := range pvcMap {
			builder.BuildPersistentVolumeClaimLabels(sts, pvc, component, pvcTplName)
			objs = append(objs, pvc)
		}

		// binding persistentVolumeClaim to podSpec.Volumes
		podSpec := &sts.Spec.Template.Spec
		if podSpec == nil {
			return objs, nil
		}

		podVolumes := podSpec.Volumes
		for _, pvc := range pvcMap {
			volumeName := strings.Split(pvc.Name, "-")[0]
			podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, volumeName, func(volumeName string) corev1.Volume {
				return corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				}
			}, nil)
		}
		podSpec.Volumes = podVolumes

		return objs, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *replicationComponentWorkloadBuilder) Complete() error {
	if b.Error != nil {
		return b.Error
	}
	if len(b.workloads) == 0 || b.workloads[0] == nil {
		return fmt.Errorf("fail to create compoennt workloads, cluster: %s, component: %s",
			b.Comp.GetClusterName(), b.Comp.GetName())
	}

	for _, obj := range b.workloads {
		b.Comp.AddWorkload(obj, b.DefaultAction, nil)
	}
	return nil
}
