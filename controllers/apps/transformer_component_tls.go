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
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

// componentTLSTransformer handles component configuration render
type componentTLSTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentTLSTransformer{}

func (t *componentTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	synthesizedComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	// update podSpec tls volume and volumeMount
	if err := updateTLSVolumeAndVolumeMount(synthesizedComp.PodSpec, synthesizedComp.ClusterName, *synthesizedComp); err != nil {
		return err
	}

	// build tls cert
	return buildTLSCert(transCtx.Context, transCtx.Client, *synthesizedComp, dag)
}

func buildTLSCert(ctx context.Context, cli client.Reader, synthesizedComp component.SynthesizedComponent, dag *graph.DAG) error {
	tls := synthesizedComp.TLSConfig
	if tls == nil || !tls.Enable {
		return nil
	}
	if tls.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}

	switch tls.Issuer.Name {
	case appsv1.IssuerUserProvided:
		if err := plan.CheckTLSSecretRef(ctx, cli, synthesizedComp.Namespace, tls.Issuer.SecretRef); err != nil {
			return err
		}
	case appsv1.IssuerKubeBlocks:
		graphCli, _ := cli.(model.GraphClient)
		secretName := plan.GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)
		existSecret := &corev1.Secret{}
		err := cli.Get(ctx, types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: secretName}, existSecret)
		if err != nil {
			if errors.IsNotFound(err) {
				secret, err := plan.ComposeTLSSecret(synthesizedComp)
				if err != nil {
					return err
				}
				graphCli.Create(dag, secret)
				return nil
			}
			return err
		} else {
			updateTLSSecretMeta(existSecret, graphCli, dag, synthesizedComp)
		}
	}
	return nil
}

func updateTLSSecretMeta(existSecret *corev1.Secret, graphCli model.GraphClient, dag *graph.DAG, synthesizedComp component.SynthesizedComponent) {
	secretProto := plan.BuildTLSSecret(synthesizedComp)
	existSecretCopy := existSecret.DeepCopy()
	existSecretCopy.Labels = secretProto.Labels
	existSecretCopy.Annotations = secretProto.Annotations
	if !reflect.DeepEqual(existSecret, existSecretCopy) {
		graphCli.Update(dag, existSecret, existSecretCopy)
	}
}

func updateTLSVolumeAndVolumeMount(podSpec *corev1.PodSpec, clusterName string, synthesizeComp component.SynthesizedComponent) error {
	tls := synthesizeComp.TLSConfig
	if tls == nil || !tls.Enable {
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
	tls := synthesizeComp.TLSConfig
	if tls == nil || !tls.Enable {
		return nil, fmt.Errorf("can't compose TLS volume when TLS not enabled")
	}
	if tls.Issuer == nil {
		return nil, fmt.Errorf("issuer shouldn't be nil when TLS enabled")
	}
	if tls.Issuer.Name == appsv1.IssuerUserProvided && tls.Issuer.SecretRef == nil {
		return nil, fmt.Errorf("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch tls.Issuer.Name {
	case appsv1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, synthesizeComp.Name)
		ca = constant.CAName
		cert = constant.CertName
		key = constant.KeyName
	case appsv1.IssuerUserProvided:
		secretName = tls.Issuer.SecretRef.Name
		ca = tls.Issuer.SecretRef.CA
		cert = tls.Issuer.SecretRef.Cert
		key = tls.Issuer.SecretRef.Key
	}
	mode := int32(0600)
	volume := corev1.Volume{
		Name: constant.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: constant.CAName},
					{Key: cert, Path: constant.CertName},
					{Key: key, Path: constant.KeyName},
				},
				Optional:    func() *bool { o := false; return &o }(),
				DefaultMode: &mode,
			},
		},
	}

	return &volume, nil
}

func composeTLSVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      constant.VolumeName,
		MountPath: constant.MountPath,
		ReadOnly:  true,
	}
}
