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
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type componentBuilder struct {
	ReqCtx intctrlutil.RequestCtx
	Client client.Client

	Comp *consensusComponent

	Error     error
	EnvConfig *corev1.ConfigMap
	Workload  *appsv1.StatefulSet
}

func (b *componentBuilder) buildEnv() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg, err := builder.BuildEnvConfigLow(b.ReqCtx, b.Client, &b.Comp.Cluster, b.Comp.Component)
		b.EnvConfig = envCfg
		return []client.Object{envCfg}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildConfig() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.Comp.Cluster.Name, b.Comp.Component.Name)
		}
		podSpec := &b.Workload.Spec.Template.Spec
		// TODO: where is buildCfg?
		configs, err := buildCfg(b.Task, b.Workload, podSpec, b.ReqCtx.Ctx, b.Client)
		return configs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildHeadlessService() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svc, err := builder.BuildHeadlessSvcLow(&b.Comp.Cluster, b.Comp.Component)
		return []client.Object{svc}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildService() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(&b.Comp.Cluster, b.Comp.Component)
		objs := make([]client.Object, len(svcList))
		if err == nil {
			for _, svc := range svcList {
				addLeaderSelectorLabels(svc, b.Comp.Component) // TODO: consensus logic
				objs = append(objs, svc)
			}
		}
		return objs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildWorkload() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.Cluster.Name, b.Comp.Component.Name)
		}
		sts, err := builder.BuildStsLow(b.ReqCtx, &b.Comp.Cluster, b.Comp.Component, b.EnvConfig.Name)
		if sts != nil {
			sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
		}
		b.Workload = sts
		return nil, err // don't add sts to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildTlsVolume() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build TLS volumes but workload is nil, cluster: %s, component: %s",
				b.Comp.Cluster.Name, b.Comp.Component.Name)
		}
		podSpec := &b.Workload.Spec.Template.Spec
		return nil, updateTLSVolumeAndVolumeMount(podSpec, b.Comp.Cluster.Name, *b.Comp.Component)
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildPDB() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Comp.Component.MaxUnavailable == nil {
			return nil, nil
		}
		pdb, err := builder.BuildPDBLow(&b.Comp.Cluster, b.Comp.Component)
		return []client.Object{pdb}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentBuilder) buildVolumeMount() *componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build volume mount but workload is nil, cluster: %s, component: %s",
				b.Comp.Cluster.Name, b.Comp.Component.Name)
		}
		podSpec := &b.Workload.Spec.Template.Spec
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

func (b *componentBuilder) complete() error {
	if b.Error != nil {
		return b.Error
	}
	if b.Workload == nil {
		return fmt.Errorf("fail to create consensus workload, cluster: %s, component: %s",
			b.Comp.Cluster.Name, b.Comp.Component.Name)
	}
	b.Comp.WorkloadVertex = b.Comp.createResource(b.Workload, nil)
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
