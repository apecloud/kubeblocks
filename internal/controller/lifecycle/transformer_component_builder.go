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

package lifecycle

import (
	"fmt"
	"github.com/apecloud/kubeblocks/internal/controller/component"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO(refactor): define a custom workload to encapsulate all the resources.
//
//	runtime, config, script, env, volume, service, monitor, probe
type componentWorkloadBuilder interface {
	buildEnv() componentWorkloadBuilder
	buildHeadlessService() componentWorkloadBuilder
	buildService() componentWorkloadBuilder
	buildTLSCert() componentWorkloadBuilder

	// workload related
	buildConfig(idx int32) componentWorkloadBuilder
	buildWorkload(idx int32) componentWorkloadBuilder
	buildVolume(idx int32) componentWorkloadBuilder
	buildVolumeMount(idx int32) componentWorkloadBuilder
	buildTLSVolume(idx int32) componentWorkloadBuilder

	complete() error

	mutableWorkload(idx int32) client.Object
	mutableRuntime(idx int32) *corev1.PodSpec
}

type componentWorkloadBuilderBase struct {
	reqCtx          intctrlutil.RequestCtx
	client          client.Client
	comp            Component
	defaultAction   *Action
	concreteBuilder componentWorkloadBuilder
	error           error
	envConfig       *corev1.ConfigMap
}

// TODO(refactor): workload & scaling related
func (b *componentWorkloadBuilderBase) buildEnv() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg, err := builder.BuildEnvConfigLow(b.reqCtx, b.client, b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		b.envConfig = envCfg
		return []client.Object{envCfg}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) buildConfig(idx int32) componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.concreteBuilder.mutableWorkload(idx)
		if workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}

		return plan.BuildCfgLow(b.comp.GetVersion(), b.comp.GetCluster(), b.comp.GetSynthesizedComponent(), workload,
			b.concreteBuilder.mutableRuntime(idx), b.reqCtx.Ctx, b.client)
	}
	return b.buildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) buildHeadlessService() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svc, err := builder.BuildHeadlessSvcLow(b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		return []client.Object{svc}, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) buildService() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0)
		for _, svc := range svcList {
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) buildVolume(_ int32) componentWorkloadBuilder {
	return b.buildWrapper(nil)
}

func (b *componentWorkloadBuilderBase) buildVolumeMount(idx int32) componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.concreteBuilder.mutableWorkload(idx) == nil {
			return nil, fmt.Errorf("build volume mount but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}

		podSpec := b.concreteBuilder.mutableRuntime(idx)
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

func (b *componentWorkloadBuilderBase) buildTLSCert() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		cluster := b.comp.GetCluster()
		component := b.comp.GetSynthesizedComponent()
		if !component.TLS {
			return nil, nil
		}
		if component.Issuer == nil {
			return nil, fmt.Errorf("issuer shouldn't be nil when tls enabled")
		}

		objs := make([]client.Object, 0)
		switch component.Issuer.Name {
		case appsv1alpha1.IssuerUserProvided:
			if err := plan.CheckTLSSecretRef(b.reqCtx, b.client, cluster.Namespace, component.Issuer.SecretRef); err != nil {
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

func (b *componentWorkloadBuilderBase) buildTLSVolume(idx int32) componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.concreteBuilder.mutableWorkload(idx) == nil {
			return nil, fmt.Errorf("build TLS volumes but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}
		// build secret volume and volume mount
		podSpec := b.concreteBuilder.mutableRuntime(idx)
		return nil, updateTLSVolumeAndVolumeMount(podSpec, b.comp.GetClusterName(), *b.comp.GetSynthesizedComponent())
	}
	return b.buildWrapper(buildfn)
}

func (b *componentWorkloadBuilderBase) complete() error {
	if b.error != nil {
		return b.error
	}
	workload := b.concreteBuilder.mutableWorkload(0)
	if workload == nil {
		return fmt.Errorf("fail to create compoennt workloads, cluster: %s, component: %s",
			b.comp.GetClusterName(), b.comp.GetName())
	}
	b.comp.addWorkload(workload, b.defaultAction, nil)
	return nil
}

func (b *componentWorkloadBuilderBase) buildWrapper(buildfn func() ([]client.Object, error)) componentWorkloadBuilder {
	if b.error != nil || buildfn == nil {
		return b.concreteBuilder
	}
	objs, err := buildfn()
	if err != nil {
		b.error = err
	} else {
		for _, obj := range objs {
			b.comp.addResource(obj, b.defaultAction, nil)
		}
	}
	return b.concreteBuilder
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
