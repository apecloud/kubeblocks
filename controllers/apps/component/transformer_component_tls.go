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

package component

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

type tlsIssuer interface {
	create(ctx context.Context, cli client.Reader) (*corev1.Secret, error)
	delete(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error)
	update(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error)
}

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

	secretObj, err := t.secretObject(transCtx, synthesizedComp)
	if err != nil {
		return err
	}

	issuer := t.newTLSIssuer(compDef, synthesizedComp)
	if enabled {
		if secretObj == nil {
			if err = t.handleCreate(transCtx.Context, transCtx.Client, dag, issuer); err != nil {
				return err
			}
		} else {
			if err = t.handleUpdate(transCtx.Context, transCtx.Client, dag, issuer, secretObj); err != nil {
				return err
			}
		}
		return t.updateVolumeNVolumeMount(compDef, synthesizedComp)
	} else {
		// the issuer and secretObj may be nil
		if err = t.handleDelete(transCtx.Context, transCtx.Client, dag, issuer, secretObj); err != nil {
			return err
		}
		return t.removeVolumeNVolumeMount(compDef, synthesizedComp)
	}
}

func (t *componentTLSTransformer) enabled(compDef *appsv1.ComponentDefinition,
	synthesizedComp *component.SynthesizedComponent) (bool, error) {
	tls := synthesizedComp.TLSConfig
	if tls == nil || !tls.Enable {
		return false, nil
	}
	if tls.Issuer == nil {
		return false, fmt.Errorf("the issuer shouldn't be nil when the TLS is enabled")
	}
	if slices.Contains([]appsv1.IssuerName{appsv1.IssuerUserProvided, appsv1.IssuerKubeBlocks}, tls.Issuer.Name) {
		return false, fmt.Errorf("unknown TLS issuer %s", tls.Issuer.Name)
	}
	if compDef.Spec.TLS == nil {
		return false, fmt.Errorf("the TLS is enabled but the component definition %s doesn't support it", compDef.Name)
	}
	return true, nil
}

func (t *componentTLSTransformer) secretObject(transCtx *componentTransformContext,
	synthesizedComp *component.SynthesizedComponent) (*corev1.Secret, error) {
	secretKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      plan.GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name),
	}
	secret := &corev1.Secret{}
	err := transCtx.Client.Get(transCtx.Context, secretKey, secret)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return secret, nil
}

func (t *componentTLSTransformer) newTLSIssuer(compDef *appsv1.ComponentDefinition,
	synthesizedComp *component.SynthesizedComponent) tlsIssuer {
	var issuerName appsv1.IssuerName
	if synthesizedComp.TLSConfig != nil && synthesizedComp.TLSConfig.Issuer != nil {
		issuerName = synthesizedComp.TLSConfig.Issuer.Name
	}
	switch issuerName {
	case appsv1.IssuerUserProvided:
		return &tlsIssuerUserProvided{
			compDef:         compDef,
			synthesizedComp: synthesizedComp,
		}
	case appsv1.IssuerKubeBlocks:
		return &tlsIssuerKubeBlocks{
			compDef:         compDef,
			synthesizedComp: synthesizedComp,
		}
	default:
		return nil
	}
}

func (t *componentTLSTransformer) handleCreate(ctx context.Context, cli client.Reader, dag *graph.DAG, issuer tlsIssuer) error {
	secret, err := issuer.create(ctx, cli)
	if err != nil {
		return err
	}
	if secret != nil {
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Create(dag, secret)
	}
	return nil
}

func (t *componentTLSTransformer) handleDelete(ctx context.Context, cli client.Reader,
	dag *graph.DAG, issuer tlsIssuer, secretObj *corev1.Secret) error {
	var (
		secret = secretObj
		err    error
	)
	if issuer != nil {
		secret, err = issuer.delete(ctx, cli, secretObj)
		if err != nil {
			return err
		}
	}
	if secret != nil {
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Delete(dag, secret)
	}
	return nil
}

func (t *componentTLSTransformer) handleUpdate(ctx context.Context, cli client.Reader,
	dag *graph.DAG, issuer tlsIssuer, secretObj *corev1.Secret) error {
	secret, err := issuer.update(ctx, cli, secretObj)
	if err != nil {
		return err
	}
	if secret != nil {
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Update(dag, secretObj, secret)
	}
	return nil
}

type tlsIssuerKubeBlocks struct {
	compDef         *appsv1.ComponentDefinition
	synthesizedComp *component.SynthesizedComponent
}

func (i *tlsIssuerKubeBlocks) create(ctx context.Context, cli client.Reader) (*corev1.Secret, error) {
	return plan.ComposeTLSSecret(i.compDef, *i.synthesizedComp, nil)
}

func (i *tlsIssuerKubeBlocks) delete(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error) {
	return secret, nil
}

func (i *tlsIssuerKubeBlocks) update(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error) {
	proto := plan.BuildTLSSecret(*i.synthesizedComp)

	// TODO: update secret data if needed
	secretCopy := secret.DeepCopy()
	secretCopy.Labels = proto.Labels
	secretCopy.Annotations = proto.Annotations

	if !reflect.DeepEqual(secret, secretCopy) {
		return secretCopy, nil
	}
	return nil, nil
}

type tlsIssuerUserProvided struct {
	compDef         *appsv1.ComponentDefinition
	synthesizedComp *component.SynthesizedComponent
}

func (i *tlsIssuerUserProvided) create(ctx context.Context, cli client.Reader) (*corev1.Secret, error) {
	secret, err := i.referenced(ctx, cli)
	if err != nil {
		return nil, err
	}

	proto := plan.BuildTLSSecret(*i.synthesizedComp)

	secretRef := i.synthesizedComp.TLSConfig.Issuer.SecretRef
	if i.compDef.Spec.TLS.CAFile != nil {
		proto.Data[*i.compDef.Spec.TLS.CAFile] = secret.Data[secretRef.CA]
	}
	if i.compDef.Spec.TLS.CertFile != nil {
		proto.Data[*i.compDef.Spec.TLS.CertFile] = secret.Data[secretRef.Cert]
	}
	if i.compDef.Spec.TLS.KeyFile != nil {
		proto.Data[*i.compDef.Spec.TLS.KeyFile] = secret.Data[secretRef.Key]
	}

	return proto, nil
}

func (i *tlsIssuerUserProvided) delete(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error) {
	return secret, nil
}

func (i *tlsIssuerUserProvided) update(ctx context.Context, cli client.Reader, secret *corev1.Secret) (*corev1.Secret, error) {
	proto := plan.BuildTLSSecret(*i.synthesizedComp)

	// TODO: update secret data if needed
	secretCopy := secret.DeepCopy()
	secretCopy.Labels = proto.Labels
	secretCopy.Annotations = proto.Annotations

	if !reflect.DeepEqual(secret, secretCopy) {
		return secretCopy, nil
	}
	return nil, nil
}

func (i *tlsIssuerUserProvided) referenced(ctx context.Context, cli client.Reader) (*corev1.Secret, error) {
	var (
		secretRef = i.synthesizedComp.TLSConfig.Issuer.SecretRef
	)
	secretKey := types.NamespacedName{
		Namespace: secretRef.Namespace,
		Name:      secretRef.Name,
	}
	secret := &corev1.Secret{}
	if err := cli.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}
	// TODO: should keep aligned with the cmpd
	if secret.Data == nil {
		return nil, fmt.Errorf("tls secret's data field shouldn't be nil")
	}
	keys := []string{secretRef.CA, secretRef.Cert, secretRef.Key}
	for _, key := range keys {
		if len(secret.Data[key]) == 0 {
			return nil, fmt.Errorf("tls secret's data[%s] field shouldn't be empty", key)
		}
	}
	return secret, nil
}

func (t *componentTLSTransformer) updateVolumeNVolumeMount(compDef *appsv1.ComponentDefinition,
	synthesizedComp *component.SynthesizedComponent) error {
	// update volume
	volumes := synthesizedComp.PodSpec.Volumes
	volume, err := t.composeTLSVolume(compDef, synthesizedComp)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(volumes, func(v corev1.Volume) bool {
		return v.Name == volume.Name
	}) {
		return fmt.Errorf("the TLS volume %s already exists", volume.Name)
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
		if !slices.ContainsFunc(mounts, func(m corev1.VolumeMount) bool {
			return m.Name == mount.Name
		}) {
			mounts = append(mounts, mount)
			synthesizedComp.PodSpec.Containers[i].VolumeMounts = mounts
		}
	}

	return nil
}

func (t *componentTLSTransformer) composeTLSVolume(compDef *appsv1.ComponentDefinition,
	synthesizedComp *component.SynthesizedComponent) (*corev1.Volume, error) {
	var secretName string
	var ca, cert, key *string

	tls := synthesizedComp.TLSConfig
	switch tls.Issuer.Name {
	case appsv1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)
		ca = compDef.Spec.TLS.CAFile
		cert = compDef.Spec.TLS.CertFile
		key = compDef.Spec.TLS.KeyFile
	case appsv1.IssuerUserProvided:
		secretName = tls.Issuer.SecretRef.Name
		if len(tls.Issuer.SecretRef.CA) > 0 {
			ca = &tls.Issuer.SecretRef.CA
		}
		if len(tls.Issuer.SecretRef.Cert) > 0 {
			cert = &tls.Issuer.SecretRef.Cert
		}
		if len(tls.Issuer.SecretRef.Key) > 0 {
			key = &tls.Issuer.SecretRef.Key
		}
	}

	volume := corev1.Volume{
		Name: compDef.Spec.TLS.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items:      []corev1.KeyToPath{},
				Optional:   ptr.To(false),
			},
		},
	}

	addItem := func(source, target *string) error {
		if target != nil && source == nil {
			return fmt.Errorf("%s is required but not provided", *target)
		}
		if target != nil && source != nil {
			volume.VolumeSource.Secret.Items =
				append(volume.VolumeSource.Secret.Items, corev1.KeyToPath{Key: *source, Path: *target})
		}
		return nil
	}
	if err := addItem(ca, compDef.Spec.TLS.CAFile); err != nil {
		return nil, err
	}
	if err := addItem(cert, compDef.Spec.TLS.CertFile); err != nil {
		return nil, err
	}
	if err := addItem(key, compDef.Spec.TLS.KeyFile); err != nil {
		return nil, err
	}

	if compDef.Spec.TLS.DefaultMode != nil {
		volume.VolumeSource.Secret.DefaultMode = ptr.To(*compDef.Spec.TLS.DefaultMode)
	} else {
		volume.VolumeSource.Secret.DefaultMode = ptr.To(int32(0600))
	}

	return &volume, nil
}

func (t *componentTLSTransformer) removeVolumeNVolumeMount(compDef *appsv1.ComponentDefinition,
	synthesizedComp *component.SynthesizedComponent) error {
	if compDef.Spec.TLS == nil || len(compDef.Spec.TLS.VolumeName) == 0 {
		return nil
	}

	// remove the volume
	synthesizedComp.PodSpec.Volumes = slices.DeleteFunc(synthesizedComp.PodSpec.Volumes, func(vol corev1.Volume) bool {
		return vol.Name == compDef.Spec.TLS.VolumeName
	})

	// remove the volume mount
	for i := range synthesizedComp.PodSpec.Containers {
		synthesizedComp.PodSpec.Containers[i].VolumeMounts =
			slices.DeleteFunc(synthesizedComp.PodSpec.Containers[i].VolumeMounts, func(m corev1.VolumeMount) bool {
				return m.Name == compDef.Spec.TLS.VolumeName
			})
	}

	return nil
}
