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

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentWorkloadBuilder interface {
	//	runtime, config, script, env, volume, service, monitor, probe
	BuildEnv() componentWorkloadBuilder
	BuildConfig() componentWorkloadBuilder
	BuildWorkload() componentWorkloadBuilder
	BuildPDB() componentWorkloadBuilder
	BuildVolumeMount() componentWorkloadBuilder
	BuildTLSCert() componentWorkloadBuilder
	BuildTLSVolume() componentWorkloadBuilder
	BuildCustomVolumes() componentWorkloadBuilder

	Complete() error
}

type rsmComponentWorkloadBuilder struct {
	reqCtx        intctrlutil.RequestCtx
	client        client.Client
	comp          *rsmComponent
	defaultAction *model.Action
	error         error
	envConfig     *corev1.ConfigMap
	workload      client.Object
	localObjs     []client.Object // cache the objects needed for configuration, should remove this after refactoring the configuration
}

var _ componentWorkloadBuilder = &rsmComponentWorkloadBuilder{}

func (b *rsmComponentWorkloadBuilder) BuildEnv() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		envCfg := factory.BuildEnvConfig(b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		b.envConfig = envCfg
		b.localObjs = append(b.localObjs, envCfg)
		return []client.Object{envCfg}, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) BuildCustomVolumes() componentWorkloadBuilder {
	return b.BuildWrapper(func() ([]client.Object, error) {
		return nil, doBuildCustomVolumes(b.getRuntime(), b.comp.GetCluster(), b.comp.GetName(), b.comp.GetNamespace())
	})
}

func (b *rsmComponentWorkloadBuilder) BuildConfig() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.workload == nil {
			return nil, fmt.Errorf("build config but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}

		err := plan.RenderConfigNScriptFiles(
			&intctrlutil.ResourceCtx{
				Context:       b.reqCtx.Ctx,
				Client:        b.client,
				Namespace:     b.comp.GetNamespace(),
				ClusterName:   b.comp.GetClusterName(),
				ComponentName: b.comp.GetName(),
			},
			b.comp.GetClusterVersion(),
			b.comp.GetCluster(),
			b.comp.GetSynthesizedComponent(),
			b.workload,
			b.getRuntime(),
			b.localObjs)
		return nil, err
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) BuildWorkload() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		component := b.comp.GetSynthesizedComponent()
		obj, err := factory.BuildRSM(b.reqCtx, b.comp.GetCluster(), component, b.envConfig.Name)
		if err != nil {
			return nil, err
		}

		b.workload = obj

		return nil, nil // don't return sts here
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) BuildPDB() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		// if without this handler, the cluster controller will occur error during reconciling.
		// conditionally build PodDisruptionBudget
		synthesizedComponent := b.comp.GetSynthesizedComponent()
		if synthesizedComponent.MinAvailable != nil {
			pdb := factory.BuildPDB(b.comp.GetCluster(), synthesizedComponent)
			return []client.Object{pdb}, nil
		} else {
			panic("this shouldn't happen")
		}
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) BuildVolumeMount() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.workload == nil {
			return nil, fmt.Errorf("build volume mount but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
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

func (b *rsmComponentWorkloadBuilder) BuildTLSCert() componentWorkloadBuilder {
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
			if err := plan.CheckTLSSecretRef(b.reqCtx.Ctx, b.client, cluster.Namespace, component.Issuer.SecretRef); err != nil {
				return nil, err
			}
		case appsv1alpha1.IssuerKubeBlocks:
			secret, err := plan.ComposeTLSSecret(cluster.Namespace, cluster.Name, component.Name)
			if err != nil {
				return nil, err
			}
			objs = append(objs, secret)
			b.localObjs = append(b.localObjs, secret)
		}
		return objs, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) BuildTLSVolume() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.workload == nil {
			return nil, fmt.Errorf("build TLS volumes but workload is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}
		// build secret volume and volume mount
		return nil, updateTLSVolumeAndVolumeMount(b.getRuntime(), b.comp.GetClusterName(), *b.comp.GetSynthesizedComponent())
	}
	return b.BuildWrapper(buildfn)
}

func (b *rsmComponentWorkloadBuilder) Complete() error {
	if b.error != nil {
		return b.error
	}
	if b.workload == nil {
		return fmt.Errorf("fail to create component workloads, cluster: %s, component: %s",
			b.comp.GetClusterName(), b.comp.GetName())
	}
	b.comp.setWorkload(b.workload, b.defaultAction)
	return nil
}

func (b *rsmComponentWorkloadBuilder) BuildWrapper(buildfn func() ([]client.Object, error)) componentWorkloadBuilder {
	if b.error != nil || buildfn == nil {
		return b
	}
	objs, err := buildfn()
	if err != nil {
		b.error = err
	} else {
		cluster := b.comp.GetCluster()
		component := b.comp.GetSynthesizedComponent()
		if err = updateCustomLabelToObjs(cluster.Name, string(cluster.UID), component.Name, component.CustomLabelSpecs, objs); err != nil {
			b.error = err
		}
		for _, obj := range objs {
			b.comp.addResource(obj, b.defaultAction)
		}
	}
	return b
}

func (b *rsmComponentWorkloadBuilder) getRuntime() *corev1.PodSpec {
	switch w := b.workload.(type) {
	case *appsv1.StatefulSet:
		return &w.Spec.Template.Spec
	case *appsv1.Deployment:
		return &w.Spec.Template.Spec
	case *workloads.ReplicatedStateMachine:
		return &w.Spec.Template.Spec
	default:
		return nil
	}
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
		ca = factory.CAName
		cert = factory.CertName
		key = factory.KeyName
	case appsv1alpha1.IssuerUserProvided:
		secretName = component.Issuer.SecretRef.Name
		ca = component.Issuer.SecretRef.CA
		cert = component.Issuer.SecretRef.Cert
		key = component.Issuer.SecretRef.Key
	}
	volume := corev1.Volume{
		Name: factory.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: factory.CAName},
					{Key: cert, Path: factory.CertName},
					{Key: key, Path: factory.KeyName},
				},
				Optional: func() *bool { o := false; return &o }(),
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      factory.VolumeName,
		MountPath: factory.MountPath,
		ReadOnly:  true,
	}
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
