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

package component

import (
	"fmt"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type componentBuilder struct {
	ReqCtx intctrlutil.RequestCtx
	Client client.Client

	Comp Component

	Error     error
	EnvConfig *corev1.ConfigMap
	Workloads []*appsv1.StatefulSet
}

func (b *componentBuilder) buildEnv() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg, err := builder.BuildEnvConfigLow(b.ReqCtx, b.Client, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		b.EnvConfig = envCfg
		return []client.Object{envCfg}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildConfig(idx int32) *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.Workloads[idx]
		if workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}
		podSpec := &workload.Spec.Template.Spec
		return plan.BuildCfgLow(b.Comp.GetVersion(), b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent(), workload, podSpec, b.ReqCtx.Ctx, b.Client)
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildHeadlessService() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svc, err := builder.BuildHeadlessSvcLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		return []client.Object{svc}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildService(patchfn func(svc *corev1.Service)) *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, len(svcList))
		for _, svc := range svcList {
			if patchfn != nil {
				patchfn(svc)
			}
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildConsensusWorkload() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent(), b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.Workloads[0] = sts

		return nil, nil // don't return sts here, and it will not add to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildReplicationWorkload(idx int32) *componentBuilder {
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
			sts.Labels[constant.RoleLabelKey] = string(replicationset.Primary)
		} else {
			sts.Labels[constant.RoleLabelKey] = string(replicationset.Secondary)
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.Workloads[idx] = sts // TODO: more than one sts objects

		return nil, nil // don't return sts here, and it will not add to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildReplicationVolume(idx int32) *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workloads[idx] == nil {
			return nil, fmt.Errorf("build replication volumes but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		component := b.Comp.GetSynthesizedComponent()
		workload := b.Workloads[idx]
		objs := make([]client.Object, 0)

		// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
		pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(workload, component.VolumeClaimTemplates)
		for pvcTplName, pvc := range pvcMap {
			builder.BuildPersistentVolumeClaimLabels(workload, pvc, component, pvcTplName)
			objs = append(objs, pvc)
		}

		// binding persistentVolumeClaim to podSpec.Volumes
		podSpec := &workload.Spec.Template.Spec
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
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildVolumeMount(idx int32) *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.Workloads[idx]
		if workload == nil {
			return nil, fmt.Errorf("build volume mount but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		podSpec := &workload.Spec.Template.Spec
		for _, cc := range []*[]corev1.Container{&podSpec.Containers, &podSpec.InitContainers} {
			volumes := podSpec.Volumes
			for _, c := range *cc {
				for _, v := range c.VolumeMounts {
					// if persistence is not found, add emptyDir pod.spec.volumes[]
					createfn := func(_ string) corev1.Volume {
						return corev1.Volume{
							Name: v.Name,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						}
					}
					volumes, _ = intctrlutil.CreateOrUpdateVolume(volumes, v.Name, createfn, nil)
				}
			}
			podSpec.Volumes = volumes
		}
		return nil, nil
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildTLSCert() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		cluster := b.Comp.GetCluster()
		component := b.Comp.GetSynthesizedComponent()
		if !component.TLS {
			return nil, nil
		}
		if component.Issuer == nil {
			return nil, fmt.Errorf("issuer shouldn't be nil when tls enabled")
		}

		objs := make([]client.Object, 0)
		switch component.Issuer.Name {
		case appsv1alpha1.IssuerUserProvided:
			if err := plan.CheckTLSSecretRef(b.ReqCtx, b.Client, cluster.Namespace, component.Issuer.SecretRef); err != nil {
				return nil, err
			}
		case appsv1alpha1.IssuerKubeBlocks:
			secret, err := plan.ComposeTLSSecret(cluster.Namespace, cluster.Name, component.Name)
			if err != nil {
				return nil, err
			}
			objs = append(objs, secret)
		}
		return objs, nil
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildTLSVolume(idx int32) *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.Workloads[idx]
		if workload == nil {
			return nil, fmt.Errorf("build TLS volumes but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}
		// build secret volume and volume mount
		podSpec := &workload.Spec.Template.Spec
		return nil, updateTLSVolumeAndVolumeMount(podSpec, b.Comp.GetClusterName(), *b.Comp.GetSynthesizedComponent())
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildPDB() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Comp.GetSynthesizedComponent().MaxUnavailable == nil {
			return nil, nil
		}
		pdb, err := builder.BuildPDBLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		return []client.Object{pdb}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) complete() error {
	if b.Error != nil {
		return b.Error
	}
	if len(b.Workloads) == 0 || b.Workloads[0] == nil {
		return fmt.Errorf("fail to create compoennt workloads, cluster: %s, component: %s",
			b.Comp.GetClusterName(), b.Comp.GetName())
	}

	for i, obj := range b.Workloads {
		b.Comp.WorkloadVertexs[i] = b.Comp.createResource(obj, nil)
	}
	return nil
}

func (b *componentBuilder) buildWrapper(buildfn func() ([]client.Object, error)) *componentBuilder {
	if b.Error != nil {
		return b
	}
	objs, err := buildfn()
	if err != nil {
		b.Error = err
	} else if objs != nil {
		for _, obj := range objs {
			b.Comp.createResource(obj, nil)
		}
	}
	return b
}
