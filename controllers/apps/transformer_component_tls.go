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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

// componentTLSTransformer handles the TLS configuration for the component.
type componentTLSTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentTLSTransformer{}

func (t *componentTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	var (
		transCtx        = ctx.(*componentTransformContext)
		compDef         = transCtx.CompDef
		synthesizedComp = transCtx.SynthesizeComponent
	)

	enabled, err := t.enabled(compDef, synthesizedComp)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if synthesizedComp.TLSConfig.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}

	// build tls cert
	if err := buildTLSCert(transCtx.Context, transCtx.Client, compDef, *synthesizedComp, dag); err != nil {
		return err
	}

	if err := t.updateVolumeNVolumeMount(compDef, synthesizedComp); err != nil {
		return err
	}

	return nil
}

func (t *componentTLSTransformer) enabled(compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) (bool, error) {
	if synthesizedComp.TLSConfig == nil || !synthesizedComp.TLSConfig.Enable {
		return false, nil
	}
	if compDef.Spec.TLS == nil {
		return false, fmt.Errorf("the TLS is not supported by the component definition %s", compDef.Name)
	}
	return true, nil
}

func buildTLSCert(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, synthesizedComp component.SynthesizedComponent, dag *graph.DAG) error {
	var (
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
		tls         = synthesizedComp.TLSConfig
	)
	switch tls.Issuer.Name {
	case appsv1.IssuerUserProvided:
		if err := plan.CheckTLSSecretRef(ctx, cli, namespace, tls.Issuer.SecretRef); err != nil {
			return err
		}
	case appsv1.IssuerKubeBlocks:
		secretName := plan.GenerateTLSSecretName(clusterName, compName)
		existSecret := &corev1.Secret{}
		err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, existSecret)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		graphCli, _ := cli.(model.GraphClient)
		if err != nil {
			secret, err := plan.ComposeTLSSecret(compDef, synthesizedComp)
			if err != nil {
				return err
			}
			graphCli.Create(dag, secret)
		} else {
			updateTLSSecretMeta(graphCli, dag, synthesizedComp, existSecret)
		}
	}
	return nil
}

func updateTLSSecretMeta(graphCli model.GraphClient, dag *graph.DAG, synthesizedComp component.SynthesizedComponent, secret *corev1.Secret) {
	proto := plan.BuildTLSSecret(synthesizedComp)
	secretCopy := secret.DeepCopy()
	secretCopy.Labels = proto.Labels
	secretCopy.Annotations = proto.Annotations
	if !reflect.DeepEqual(secret, secretCopy) {
		graphCli.Update(dag, secret, secretCopy)
	}
}

func (t *componentTLSTransformer) updateVolumeNVolumeMount(
	compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) error {
	// update volume
	volumes := synthesizedComp.PodSpec.Volumes
	volume, err := t.composeTLSVolume(compDef, synthesizedComp)
	if err != nil {
		return err
	}
	volumes = append(volumes, *volume)
	synthesizedComp.PodSpec.Volumes = volumes

	// update volumeMount
	mount := corev1.VolumeMount{
		Name:      compDef.Spec.TLS.VolumeName,
		MountPath: compDef.Spec.TLS.MountPath,
		ReadOnly:  true,
	}
	for i := range synthesizedComp.PodSpec.Containers {
		mounts := synthesizedComp.PodSpec.Containers[i].VolumeMounts
		synthesizedComp.PodSpec.Containers[i].VolumeMounts = append(mounts, mount)
	}

	return nil
}

func (t *componentTLSTransformer) composeTLSVolume(
	compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) (*corev1.Volume, error) {
	var secretName, ca, cert, key string

	tls := synthesizedComp.TLSConfig
	switch tls.Issuer.Name {
	case appsv1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)
		ca = *compDef.Spec.TLS.CAFile
		cert = *compDef.Spec.TLS.CertFile
		key = *compDef.Spec.TLS.KeyFile
	case appsv1.IssuerUserProvided:
		secretName = tls.Issuer.SecretRef.Name
		ca = tls.Issuer.SecretRef.CA
		cert = tls.Issuer.SecretRef.Cert
		key = tls.Issuer.SecretRef.Key
	}
	volume := corev1.Volume{
		Name: compDef.Spec.TLS.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{Key: ca, Path: *compDef.Spec.TLS.CAFile},
					{Key: cert, Path: *compDef.Spec.TLS.CertFile},
					{Key: key, Path: *compDef.Spec.TLS.KeyFile},
				},
				Optional:    ptr.To(false),
				DefaultMode: ptr.To(*compDef.Spec.TLS.DefaultMode),
			},
		},
	}
	return &volume, nil
}
