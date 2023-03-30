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

package plan

import (
	"context"
	b64 "encoding/base64"

	corev1 "k8s.io/api/core/v1"
	coreclient "sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/client"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

type envBuildInFunc func(container interface{}, envName string) (string, error)

func (c *configTemplateBuilder) wrapEnvByName(localObjects *intctrltypes.ReconcileTask) envBuildInFunc {
	return func(args interface{}, envName string) (string, error) {
		container, err := fromJSONObject[corev1.Container](args)
		if err != nil {
			return "", err
		}
		return c.getEnvName(container, envName, localObjects)
	}
}

func (c configTemplateBuilder) getEnvName(container *corev1.Container, envName string, localObjects *intctrltypes.ReconcileTask) (string, error) {
	for _, v := range container.Env {
		if v.Name != envName {
			continue
		}
		switch {
		case v.ValueFrom == nil:
			return v.Value, nil
		case v.ValueFrom.ConfigMapKeyRef != nil:
			return configMapValue(v.ValueFrom.ConfigMapKeyRef, c.ctx, c.cli, c.namespace, localObjects)
		case v.ValueFrom.SecretKeyRef != nil:
			return secretValue(v.ValueFrom.SecretKeyRef, c.ctx, c.cli, c.namespace, localObjects)
		case v.ValueFrom.FieldRef != nil:
			return fieldRefValue(v.ValueFrom.FieldRef, c.podSpec)
		case v.ValueFrom.ResourceFieldRef != nil:
			return resourceRefValue(v.ValueFrom.ResourceFieldRef, c.podSpec.Containers)
		}
	}
	return c.getEnvFromResources(container.EnvFrom, envName, localObjects)
}

func (c *configTemplateBuilder) getEnvFromResources(envSources []corev1.EnvFromSource, envName string, localObjects *intctrltypes.ReconcileTask) (string, error) {
	for _, source := range envSources {
		if value, err := c.getEnvFromResource(source, envName, localObjects); err != nil {
			return "", err
		} else if value != "" {
			return value, nil
		}
	}
	return "", nil
}

func (c *configTemplateBuilder) getEnvFromResource(envSource corev1.EnvFromSource, envName string, localObjects *intctrltypes.ReconcileTask) (string, error) {
	fromConfigMap := func(ConfigMapRef *corev1.ConfigMapEnvSource) *corev1.ConfigMapKeySelector {
		return &corev1.ConfigMapKeySelector{
			Key:                  envName,
			LocalObjectReference: corev1.LocalObjectReference{Name: ConfigMapRef.Name},
		}
	}
	fromSecret := func(SecretRef *corev1.SecretEnvSource) *corev1.SecretKeySelector {
		return &corev1.SecretKeySelector{
			Key:                  envName,
			LocalObjectReference: corev1.LocalObjectReference{Name: SecretRef.Name},
		}
	}

	if envSource.ConfigMapRef != nil {
		return configMapValue(fromConfigMap(envSource.ConfigMapRef), c.ctx, c.cli, c.namespace, localObjects)
	}
	if envSource.SecretRef != nil {
		return secretValue(fromSecret(envSource.SecretRef), c.ctx, c.cli, c.namespace, localObjects)
	}
	return "", nil
}

func resourceRefValue(resourceRef *corev1.ResourceFieldSelector, containers []corev1.Container) (string, error) {
	return "", cfgcore.MakeError("not support resource field ref")
}

func fieldRefValue(podReference *corev1.ObjectFieldSelector, podSpec *corev1.PodSpec) (string, error) {
	return "", cfgcore.MakeError("not support pod field ref")
}

func getResourceFromLocal(localObjects *intctrltypes.ReconcileTask, key coreclient.ObjectKey) coreclient.Object {
	if localObjects == nil {
		return nil
	}
	return localObjects.GetResourceWithObjectKey(key)
}

func getSecretResourceObject(key coreclient.ObjectKey, localObjects *intctrltypes.ReconcileTask, ctx context.Context, cli client.ReadonlyClient) (*corev1.Secret, error) {
	object := getResourceFromLocal(localObjects, key)
	if v, ok := object.(*corev1.Secret); ok {
		return v, nil
	}
	obj := &corev1.Secret{}
	if err := cli.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func getConfigMapResourceObject(key coreclient.ObjectKey, localObjects *intctrltypes.ReconcileTask, ctx context.Context, cli client.ReadonlyClient) (*corev1.ConfigMap, error) {
	object := getResourceFromLocal(localObjects, key)
	if v, ok := object.(*corev1.ConfigMap); ok {
		return v, nil
	}
	obj := &corev1.ConfigMap{}
	if err := cli.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func secretValue(secretRef *corev1.SecretKeySelector, ctx context.Context, cli client.ReadonlyClient, namespace string, localObjects *intctrltypes.ReconcileTask) (string, error) {
	if cli == nil {
		return "", cfgcore.MakeError("not support secret[%s] value in local mode, cli is nil", secretRef.Name)
	}

	secretKey := coreclient.ObjectKey{
		Name:      secretRef.Name,
		Namespace: namespace,
	}

	secret, err := getSecretResourceObject(secretKey, localObjects, ctx, cli)
	if err != nil {
		return "", err
	}

	if v, ok := secret.Data[secretRef.Key]; ok {
		return decodeString(v)
	}
	return "", nil
}

func decodeString(encoded []byte) (string, error) {
	decoded, err := b64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func configMapValue(configmapRef *corev1.ConfigMapKeySelector, ctx context.Context, cli client.ReadonlyClient, namespace string, localObjects *intctrltypes.ReconcileTask) (string, error) {
	if cli == nil {
		return "", cfgcore.MakeError("not support configmap[%s] value in local mode, cli is nil", configmapRef.Name)
	}

	cmKey := coreclient.ObjectKey{
		Name:      configmapRef.Name,
		Namespace: namespace,
	}
	cm, err := getConfigMapResourceObject(cmKey, localObjects, ctx, cli)
	if err != nil {
		return "", err
	}

	return cm.Data[configmapRef.Key], nil
}
