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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentTLSTransformer handles component configuration render
type componentTLSTransformer struct{}

var _ graph.Transformer = &componentTLSTransformer{}

func (t *componentTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	synthesizedComp := transCtx.SynthesizeComponent

	// update podSpec tls volume and volumeMount
	if err := updateTLSVolumeAndVolumeMount(synthesizedComp.PodSpec, synthesizedComp.ClusterName, *synthesizedComp); err != nil {
		return err
	}

	// build tls cert
	if err := buildTLSCert(transCtx.Context, transCtx.Client, *synthesizedComp, dag); err != nil {
		return err
	}

	if err := checkAndTriggerReRender(transCtx.Context, transCtx.Client, *synthesizedComp, dag); err != nil {
		return err
	}

	return nil
}

// a hack way to notify the configuration controller to re-render config
func checkAndTriggerReRender(ctx context.Context, cli client.Reader, synthesizedComp component.SynthesizedComponent, dag *graph.DAG) error {
	cm := &corev1.ConfigMap{}
	if len(synthesizedComp.ConfigTemplates) == 0 {
		return nil
	}

	tlsKeyword := plan.GetTLSKeyWord(synthesizedComp.CharacterType)
	if tlsKeyword == "unsupported-character-type" {
		return nil
	}

	// we assume the database config is always the first item of configSpecs, this is true for now
	cmName := cfgcore.GetComponentCfgName(synthesizedComp.ClusterName, synthesizedComp.Name, synthesizedComp.ConfigTemplates[0].Name)
	if err := cli.Get(ctx, types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: cmName}, cm); err != nil {
		return client.IgnoreNotFound(err)
	}

	tlsEnabledInCM := false
	// search all config files
	// NODE: The check logic may have bugs and the parameters may be commented.
	for _, configData := range cm.Data {
		if strings.Index(configData, tlsKeyword) > 0 {
			tlsEnabledInCM = true
			break
		}
	}

	tls := synthesizedComp.TLSConfig
	if ((tls == nil || !tls.Enable) && tlsEnabledInCM) ||
		(tls != nil && tls.Enable && !tlsEnabledInCM) {
		// tls config changed
		conf := &appsv1alpha1.Configuration{}
		confKey := types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: cfgcore.GenerateComponentConfigurationName(synthesizedComp.ClusterName, synthesizedComp.Name)}
		if err := cli.Get(ctx, confKey, conf); err != nil {
			return client.IgnoreNotFound(err)
		}
		// update payload for tls
		confCopy := conf.DeepCopy()
		// confCopy.Spec.ConfigItemDetails[0].Version = fmt.Sprint(time.Now().UnixMilli())
		updated, err := intctrlutil.CheckAndPatchPayload(&confCopy.Spec.ConfigItemDetails[0], constant.TLSPayload, tls)
		if err != nil {
			return err
		}
		if !updated {
			return nil
		}

		// NODE: The check logic may have bugs, the configuration requires that it can only be updated through patch
		// bad case:
		// thread1: fetch latest configuration(id: 1000)  // e.g cluster reconcile thread
		// thread2: fetch latest configuration(id: 1000), // e.g reconfiguring operation
		// thread1: update payload without submit
		// thread2: update configuration.Spec.ConfigItemDetails[*].configFileParams[*]
		// thread2: patch configuration(id: 1001)
		// thread1: submit configuration
		// result: thread2's update will be lost
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Update(dag, conf, confCopy)
	}

	return nil
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
	case appsv1alpha1.IssuerUserProvided:
		if err := plan.CheckTLSSecretRef(ctx, cli, synthesizedComp.Namespace, tls.Issuer.SecretRef); err != nil {
			return err
		}
	case appsv1alpha1.IssuerKubeBlocks:
		secret, err := plan.ComposeTLSSecret(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
		if err != nil {
			return err
		}
		graphCli, _ := cli.(model.GraphClient)
		graphCli.Create(dag, secret)
	}

	return nil
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
	if tls.Issuer.Name == appsv1alpha1.IssuerUserProvided && tls.Issuer.SecretRef == nil {
		return nil, fmt.Errorf("secret ref shouldn't be nil when issuer is UserProvided")
	}

	var secretName, ca, cert, key string
	switch tls.Issuer.Name {
	case appsv1alpha1.IssuerKubeBlocks:
		secretName = plan.GenerateTLSSecretName(clusterName, synthesizeComp.Name)
		ca = constant.CAName
		cert = constant.CertName
		key = constant.KeyName
	case appsv1alpha1.IssuerUserProvided:
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
