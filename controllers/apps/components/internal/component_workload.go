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

package internal

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO(impl): define a custom workload to encapsulate all the resources.

type ComponentWorkloadBuilder interface {
	//	runtime, config, script, env, volume, service, monitor, probe
	BuildEnv() ComponentWorkloadBuilder
	BuildConfig() ComponentWorkloadBuilder
	BuildWorkload() ComponentWorkloadBuilder
	BuildVolumeMount() ComponentWorkloadBuilder
	BuildService() ComponentWorkloadBuilder
	BuildHeadlessService() ComponentWorkloadBuilder
	BuildTLSCert() ComponentWorkloadBuilder
	BuildTLSVolume() ComponentWorkloadBuilder

	Complete() error
}

type ComponentWorkloadBuilderBase struct {
	ReqCtx          intctrlutil.RequestCtx
	Client          client.Client
	Comp            types.Component
	DefaultAction   *ictrltypes.LifecycleAction
	ConcreteBuilder ComponentWorkloadBuilder
	Error           error
	EnvConfig       *corev1.ConfigMap
	Workload        client.Object
	LocalObjs       []client.Object // cache the objects needed for configuration, should remove this after refactoring the configuration
}

func (b *ComponentWorkloadBuilderBase) BuildEnv() ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg, err := builder.BuildEnvConfigLow(b.ReqCtx, b.Client, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		b.EnvConfig = envCfg
		b.LocalObjs = append(b.LocalObjs, envCfg)
		return []client.Object{envCfg}, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *ComponentWorkloadBuilderBase) BuildConfig() ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.Workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		objs, err := plan.BuildCfgLow(b.Comp.GetClusterVersion(), b.Comp.GetCluster(),
			b.Comp.GetSynthesizedComponent(), b.Workload, b.getRuntime(), b.LocalObjs, b.ReqCtx.Ctx, b.Client)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			if cm, ok := obj.(*corev1.ConfigMap); ok {
				b.LocalObjs = append(b.LocalObjs, cm)
			}
		}
		return objs, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *ComponentWorkloadBuilderBase) BuildVolumeMount() ComponentWorkloadBuilder {
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

func (b *ComponentWorkloadBuilderBase) BuildService() ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
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

func (b *ComponentWorkloadBuilderBase) BuildHeadlessService() ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svc, err := builder.BuildHeadlessSvcLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		return []client.Object{svc}, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *ComponentWorkloadBuilderBase) BuildTLSCert() ComponentWorkloadBuilder {
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
			b.LocalObjs = append(b.LocalObjs, secret)
		}
		return objs, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *ComponentWorkloadBuilderBase) BuildTLSVolume() ComponentWorkloadBuilder {
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

func (b *ComponentWorkloadBuilderBase) Complete() error {
	if b.Error != nil {
		return b.Error
	}
	if b.Workload == nil {
		return fmt.Errorf("fail to create compoennt workloads, cluster: %s, component: %s",
			b.Comp.GetClusterName(), b.Comp.GetName())
	}
	b.Comp.SetWorkload(b.Workload, b.DefaultAction, nil)
	return nil
}

func (b *ComponentWorkloadBuilderBase) BuildWrapper(buildfn func() ([]client.Object, error)) ComponentWorkloadBuilder {
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

func (b *ComponentWorkloadBuilderBase) getRuntime() *corev1.PodSpec {
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
