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
	b64 "encoding/base64"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreclient "sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type envBuildInFunc func(container interface{}, envName string) (string, error)

type envWrapper struct {
	*configTemplateBuilder

	// configmap or secret not yet submitted.
	localObjects *intctrltypes.ReconcileTask
	// cache remoted configmap and secret.
	cache map[schema.GroupVersionKind]map[coreclient.ObjectKey]coreclient.Object
}

func wrapGetEnvByName(templateBuilder *configTemplateBuilder, localObjects *intctrltypes.ReconcileTask) envBuildInFunc {
	wrapper := &envWrapper{
		configTemplateBuilder: templateBuilder,
		localObjects:          localObjects,
		cache:                 make(map[schema.GroupVersionKind]map[coreclient.ObjectKey]coreclient.Object),
	}
	return func(args interface{}, envName string) (string, error) {
		container, err := fromJSONObject[corev1.Container](args)
		if err != nil {
			return "", err
		}
		return wrapper.getEnvByName(container, envName)
	}
}

func (w *envWrapper) getEnvByName(container *corev1.Container, envName string) (string, error) {
	for _, v := range container.Env {
		if v.Name != envName {
			continue
		}
		switch {
		case v.ValueFrom == nil:
			return v.Value, nil
		case v.ValueFrom.ConfigMapKeyRef != nil:
			return w.configMapValue(v.ValueFrom.ConfigMapKeyRef)
		case v.ValueFrom.SecretKeyRef != nil:
			return w.secretValue(v.ValueFrom.SecretKeyRef)
		case v.ValueFrom.FieldRef != nil:
			return fieldRefValue(v.ValueFrom.FieldRef, w.podSpec)
		case v.ValueFrom.ResourceFieldRef != nil:
			return resourceRefValue(v.ValueFrom.ResourceFieldRef, w.podSpec.Containers)
		}
	}
	return w.getEnvFromResources(container.EnvFrom, envName)
}

func (w *envWrapper) getEnvFromResources(envSources []corev1.EnvFromSource, envName string) (string, error) {
	for _, source := range envSources {
		if value, err := w.getEnvFromResource(source, envName); err != nil {
			return "", err
		} else if value != "" {
			return value, nil
		}
	}
	return "", nil
}

func (w *envWrapper) getEnvFromResource(envSource corev1.EnvFromSource, envName string) (string, error) {
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
		return w.configMapValue(fromConfigMap(envSource.ConfigMapRef))
	}
	if envSource.SecretRef != nil {
		return w.secretValue(fromSecret(envSource.SecretRef))
	}
	return "", nil
}

func (w *envWrapper) secretValue(secretRef *corev1.SecretKeySelector) (string, error) {
	if w.cli == nil {
		return "", cfgcore.MakeError("not support secret[%s] value in local mode, cli is nil", secretRef.Name)
	}

	secretKey := coreclient.ObjectKey{
		Name:      secretRef.Name,
		Namespace: w.namespace,
	}
	secret, err := getResourceObject(w, &corev1.Secret{}, secretKey)
	if err != nil {
		return "", err
	}
	if v, ok := secret.Data[secretRef.Key]; ok {
		return decodeString(v)
	}
	return "", nil
}

func (w *envWrapper) configMapValue(configmapRef *corev1.ConfigMapKeySelector) (string, error) {
	if w.cli == nil {
		return "", cfgcore.MakeError("not support configmap[%s] value in local mode, cli is nil", configmapRef.Name)
	}

	cmKey := coreclient.ObjectKey{
		Name:      configmapRef.Name,
		Namespace: w.namespace,
	}
	cm, err := getResourceObject(w, &corev1.ConfigMap{}, cmKey)
	if err != nil {
		return "", err
	}
	return cm.Data[configmapRef.Key], nil
}

func (w *envWrapper) getResourceFromLocal(key coreclient.ObjectKey, gvk schema.GroupVersionKind) coreclient.Object {
	if _, ok := w.cache[gvk]; !ok {
		w.cache[gvk] = make(map[coreclient.ObjectKey]coreclient.Object)
	}
	if v, ok := w.cache[gvk][key]; ok {
		return v
	}
	if w.localObjects == nil {
		return nil
	}
	return w.localObjects.GetLocalResourceWithObjectKey(key, gvk)
}

func getResourceObject[T generics.Object, PT generics.PObject[T]](w *envWrapper, obj PT, key coreclient.ObjectKey) (PT, error) {
	gvk := generics.ToGVK(obj)
	object := w.getResourceFromLocal(key, gvk)
	if v, ok := object.(PT); ok {
		return v, nil
	}
	if err := w.cli.Get(w.ctx, key, obj); err != nil {
		return nil, err
	}
	w.cache[gvk][key] = obj
	return obj, nil
}

func decodeString(encoded []byte) (string, error) {
	decoded, err := b64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func resourceRefValue(resourceRef *corev1.ResourceFieldSelector, containers []corev1.Container) (string, error) {
	return "", cfgcore.MakeError("not support resource field ref")
}

func fieldRefValue(podReference *corev1.ObjectFieldSelector, podSpec *corev1.PodSpec) (string, error) {
	return "", cfgcore.MakeError("not support pod field ref")
}
