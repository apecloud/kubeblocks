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

package components

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO(impl): define a custom workload to encapsulate all the resources.

type componentWorkloadBuilder interface {
	//	runtime, config, script, env, volume, service, monitor, probe
	BuildEnv() componentWorkloadBuilder
	BuildConfig() componentWorkloadBuilder
	BuildWorkload() componentWorkloadBuilder
	BuildPDB() componentWorkloadBuilder
	BuildVolumeMount() componentWorkloadBuilder
	BuildService() componentWorkloadBuilder
	BuildHeadlessService() componentWorkloadBuilder
	BuildTLSCert() componentWorkloadBuilder
	BuildTLSVolume() componentWorkloadBuilder

	Complete() error
}

type componentWorkloadBuilderBase struct {
	ReqCtx          intctrlutil.RequestCtx
	Client          client.Client
	Comp            Component
	DefaultAction   *ictrltypes.LifecycleAction
	ConcreteBuilder componentWorkloadBuilder
	Error           error
	EnvConfig       *corev1.ConfigMap
	Workload        client.Object
	LocalObjs       []client.Object // cache the objects needed for configuration, should remove this after refactoring the configuration
}

func (b *componentWorkloadBuilderBase) BuildEnv() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg, err := builder.BuildEnvConfig(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		b.EnvConfig = envCfg
		b.LocalObjs = append(b.LocalObjs, envCfg)
		return []client.Object{envCfg}, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildConfig() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}
		err := plan.RenderConfigNScriptFiles(b.Comp.GetClusterVersion(), b.Comp.GetCluster(),
			b.Comp.GetSynthesizedComponent(), b.Workload, b.getRuntime(), b.LocalObjs, b.ReqCtx.Ctx, b.Client)
		return nil, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildWorkload4StatefulSet(workloadType string) componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build %s workload but env config is nil, cluster: %s, component: %s",
				workloadType, b.Comp.GetClusterName(), b.Comp.GetName())
		}

		component := b.Comp.GetSynthesizedComponent()
		sts, err := builder.BuildSts(b.ReqCtx, b.Comp.GetCluster(), component, b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}

		b.Workload = sts

		return nil, nil // don't return sts here
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildPDB() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		// if without this handler, the cluster controller will occur error during reconciling.
		// conditionally build PodDisruptionBudget
		synthesizedComponent := b.Comp.GetSynthesizedComponent()
		if synthesizedComponent.MinAvailable != nil {
			pdb, err := builder.BuildPDB(b.Comp.GetCluster(), synthesizedComponent)
			if err != nil {
				return nil, err
			}
			return []client.Object{pdb}, nil
		} else {
			panic("this shouldn't happen")
		}
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildVolumeMount() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build volume mount but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		podSpec := b.getRuntime()
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
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildService() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcList(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0)
		for _, svc := range svcList {
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildHeadlessService() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svc, err := builder.BuildHeadlessSvc(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		return []client.Object{svc}, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildTLSCert() componentWorkloadBuilder {
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
			if err := plan.CheckTLSSecretRef(b.ReqCtx.Ctx, b.Client, cluster.Namespace, component.Issuer.SecretRef); err != nil {
				return nil, err
			}
		case appsv1alpha1.IssuerKubeBlocks:
			secret, err := plan.ComposeTLSSecret(cluster.Namespace, cluster.Name, component.Name)
			if err != nil {
				return nil, err
			}
			objs = append(objs, secret)
			b.LocalObjs = append(b.LocalObjs, secret)
		}
		return objs, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) BuildTLSVolume() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build TLS volumes but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}
		// build secret volume and volume mount
		return nil, updateTLSVolumeAndVolumeMount(b.getRuntime(), b.Comp.GetClusterName(), *b.Comp.GetSynthesizedComponent())
	}
	return b.BuildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) Complete() error {
	if b.Error != nil {
		return b.Error
	}
	if b.Workload == nil {
		return fmt.Errorf("fail to create component workloads, cluster: %s, component: %s",
			b.Comp.GetClusterName(), b.Comp.GetName())
	}
	b.Comp.SetWorkload(b.Workload, b.DefaultAction, nil)
	return nil
}

func (b *componentWorkloadBuilderBase) BuildWrapper(buildfn func() ([]client.Object, error)) componentWorkloadBuilder {
	if b.Error != nil || buildfn == nil {
		return b.ConcreteBuilder
	}
	objs, err := buildfn()
	if err != nil {
		b.Error = err
	} else {
		for _, obj := range objs {
			b.Comp.AddResource(obj, b.DefaultAction, nil)
		}
	}
	return b.ConcreteBuilder
}

func (b *componentWorkloadBuilderBase) getRuntime() *corev1.PodSpec {
	if sts, ok := b.Workload.(*appsv1.StatefulSet); ok {
		return &sts.Spec.Template.Spec
	}
	if deploy, ok := b.Workload.(*appsv1.Deployment); ok {
		return &deploy.Spec.Template.Spec
	}
	return nil
}

func updateTLSVolumeAndVolumeMount(podSpec *corev1.PodSpec, clusterName string, component component.SynthesizedComponent) error {
	if !component.TLS {
		return nil
	}

	// update volume
	volumes := podSpec.Volumes
	volume, err := composeTLSVolume(clusterName, component)
	if err != nil {
		return err
	}
	volumes = append(volumes, *volume)
	podSpec.Volumes = volumes

	// update volumeMount
	for index, container := range podSpec.Containers {
		volumeMounts := container.VolumeMounts
		volumeMount := composeTLSVolumeMount()
		volumeMounts = append(volumeMounts, volumeMount)
		podSpec.Containers[index].VolumeMounts = volumeMounts
	}

	return nil
}

func composeTLSVolume(clusterName string, component component.SynthesizedComponent) (*corev1.Volume, error) {
	if !component.TLS {
		return nil, fmt.Errorf("can't compose TLS volume when TLS not enabled")
	}
	if component.Issuer == nil {
		return nil, fmt.Errorf("issuer shouldn't be nil when TLS enabled")
	}
	if component.Issuer.Name == appsv1alpha1.IssuerUserProvided && component.Issuer.SecretRef == nil {
		return nil, fmt.Errorf("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch component.Issuer.Name {
	case appsv1alpha1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, component.Name)
		ca = builder.CAName
		cert = builder.CertName
		key = builder.KeyName
	case appsv1alpha1.IssuerUserProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := corev1.Volume{
		Name: builder.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: builder.CAName},
					{Key: cert, Path: builder.CertName},
					{Key: key, Path: builder.KeyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      builder.VolumeName,
		MountPath: builder.MountPath,
		ReadOnly:  true,
	}
}
