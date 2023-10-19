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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/pkg/controller/client"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

// ComponentTLSTransformer handles component configuration render
type ComponentTLSTransformer struct{}

var _ graph.Transformer = &ComponentTLSTransformer{}

func (t *ComponentTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)
	synthesizeComp := transCtx.SynthesizeComponent

	// update podSpec tls volume and volumeMount
	if err := updateTLSVolumeAndVolumeMount(synthesizeComp.PodSpec, synthesizeComp.ClusterName, *synthesizeComp); err != nil {
		return err
	}

	// build tls cert
	if err := buildTLSCert(transCtx.Context, transCtx.Client, transCtx.Cluster, *synthesizeComp, dag); err != nil {
		return err
	}

	return nil
}

func buildTLSCert(ctx context.Context, cli roclient.ReadonlyClient, cluster *appsv1alpha1.Cluster, synthesizeComp component.SynthesizedComponent, dag *graph.DAG) error {
	if !synthesizeComp.TLS {
		return nil
	}
	if synthesizeComp.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}

	switch synthesizeComp.Issuer.Name {
	case appsv1alpha1.IssuerUserProvided:
		if err := plan.CheckTLSSecretRef(ctx, cli, cluster.Namespace, synthesizeComp.Issuer.SecretRef); err != nil {
			return err
		}
	case appsv1alpha1.IssuerKubeBlocks:
		secret, err := plan.ComposeTLSSecret(cluster.Namespace, cluster.Name, synthesizeComp.Name)
		if err != nil {
			return err
		}
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Create(dag, secret)
	}

	return nil
}

func updateTLSVolumeAndVolumeMount(podSpec *corev1.PodSpec, clusterName string, synthesizeComp component.SynthesizedComponent) error {
	if !synthesizeComp.TLS {
		return nil
	}

	// update volume
	volumes := podSpec.Volumes
	volume, err := composeTLSVolume(clusterName, synthesizeComp)
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

func composeTLSVolume(clusterName string, synthesizeComp component.SynthesizedComponent) (*corev1.Volume, error) {
	if !synthesizeComp.TLS {
		return nil, fmt.Errorf("can't compose TLS volume when TLS not enabled")
	}
	if synthesizeComp.Issuer == nil {
		return nil, fmt.Errorf("issuer shouldn't be nil when TLS enabled")
	}
	if synthesizeComp.Issuer.Name == appsv1alpha1.IssuerUserProvided && synthesizeComp.Issuer.SecretRef == nil {
		return nil, fmt.Errorf("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch synthesizeComp.Issuer.Name {
	case appsv1alpha1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, synthesizeComp.Name)
		ca = factory.CAName
		cert = factory.CertName
		key = factory.KeyName
	case appsv1alpha1.IssuerUserProvided:
		secretName = synthesizeComp.Issuer.SecretRef.Name
		ca = synthesizeComp.Issuer.SecretRef.CA
		cert = synthesizeComp.Issuer.SecretRef.Cert
		key = synthesizeComp.Issuer.SecretRef.Key
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
