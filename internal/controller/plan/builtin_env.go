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

package plan

import (
	b64 "encoding/base64"
	"regexp"
	"strings"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/util/resource"
	coreclient "sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type envBuildInFunc func(container interface{}, envName string) (string, error)

type envWrapper struct {
	// prevent circular references.
	referenceCount int
	*configTemplateBuilder

	// configmap or secret not yet submitted.
	localObjects  []coreclient.Object
	clusterName   string
	clusterUID    string
	componentName string
	// cache remoted configmap and secret.
	cache map[schema.GroupVersionKind]map[coreclient.ObjectKey]coreclient.Object
}

const maxReferenceCount = 10

func wrapGetEnvByName(templateBuilder *configTemplateBuilder, component *component.SynthesizedComponent, localObjs []coreclient.Object) envBuildInFunc {
	wrapper := &envWrapper{
		configTemplateBuilder: templateBuilder,
		localObjects:          localObjs,
		cache:                 make(map[schema.GroupVersionKind]map[coreclient.ObjectKey]coreclient.Object),
	}
	// hack for test cases of cli update cmd...
	if component != nil {
		wrapper.clusterName = component.ClusterName
		wrapper.clusterUID = component.ClusterUID
		wrapper.componentName = component.Name
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
			return w.checkAndReplaceEnv(v.Value, container)
		case v.ValueFrom.ConfigMapKeyRef != nil:
			return w.configMapValue(v.ValueFrom.ConfigMapKeyRef, container)
		case v.ValueFrom.SecretKeyRef != nil:
			return w.secretValue(v.ValueFrom.SecretKeyRef, container)
		case v.ValueFrom.FieldRef != nil:
			return fieldRefValue(v.ValueFrom.FieldRef, w.podSpec)
		case v.ValueFrom.ResourceFieldRef != nil:
			return resourceRefValue(v.ValueFrom.ResourceFieldRef, w.podSpec.Containers, container)
		}
	}
	return w.getEnvFromResources(container.EnvFrom, envName, container)
}

func (w *envWrapper) getEnvFromResources(envSources []corev1.EnvFromSource, envName string, container *corev1.Container) (string, error) {
	for _, source := range envSources {
		if value, err := w.getEnvFromResource(source, envName, container); err != nil {
			return "", err
		} else if value != "" {
			return w.checkAndReplaceEnv(value, container)
		}
	}
	return "", nil
}

func (w *envWrapper) getEnvFromResource(envSource corev1.EnvFromSource, envName string, container *corev1.Container) (string, error) {
	fromConfigMap := func(configmapRef *corev1.ConfigMapEnvSource) (string, error) {
		return w.configMapValue(&corev1.ConfigMapKeySelector{
			Key:                  envName,
			LocalObjectReference: corev1.LocalObjectReference{Name: configmapRef.Name},
		}, container)
	}
	fromSecret := func(secretRef *corev1.SecretEnvSource) (string, error) {
		return w.secretValue(&corev1.SecretKeySelector{
			Key:                  envName,
			LocalObjectReference: corev1.LocalObjectReference{Name: secretRef.Name},
		}, container)
	}

	switch {
	default:
		return "", nil
	case envSource.ConfigMapRef != nil:
		return fromConfigMap(envSource.ConfigMapRef)
	case envSource.SecretRef != nil:
		return fromSecret(envSource.SecretRef)
	}
}

func (w *envWrapper) secretValue(secretRef *corev1.SecretKeySelector, container *corev1.Container) (string, error) {
	secretPlaintext := func(m map[string]string) (string, error) {
		if v, ok := m[secretRef.Key]; ok {
			return w.checkAndReplaceEnv(v, container)
		}
		return "", nil
	}
	secretCiphertext := func(m map[string][]byte) (string, error) {
		if v, ok := m[secretRef.Key]; ok {
			return decodeString(v)
		}
		return "", nil
	}

	if w.cli == nil {
		return "", cfgcore.MakeError("not support secret[%s] value in local mode, cli is nil", secretRef.Name)
	}

	secretName, err := w.checkAndReplaceEnv(secretRef.Name, container)
	if err != nil {
		return "", err
	}
	secretKey := coreclient.ObjectKey{
		Name:      secretName,
		Namespace: w.namespace,
	}
	secret, err := getResourceObject(w, &corev1.Secret{}, secretKey)
	if err != nil {
		return "", err
	}
	if secret.StringData != nil {
		return secretPlaintext(secret.StringData)
	}
	if secret.Data != nil {
		return secretCiphertext(secret.Data)
	}
	return "", nil
}

func (w *envWrapper) configMapValue(configmapRef *corev1.ConfigMapKeySelector, container *corev1.Container) (string, error) {
	if w.cli == nil {
		return "", cfgcore.MakeError("not supported configmap[%s] value in local mode, cli is nil", configmapRef.Name)
	}

	cmName, err := w.checkAndReplaceEnv(configmapRef.Name, container)
	if err != nil {
		return "", err
	}
	cmKey := coreclient.ObjectKey{
		Name:      cmName,
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
	return findMatchedLocalObject(w.localObjects, key, gvk)
}

var envPlaceHolderRegexp = regexp.MustCompile(`\$\(\w+\)`)

func (w *envWrapper) checkAndReplaceEnv(value string, container *corev1.Container) (string, error) {
	// env value replace,e.g: $(CONN_CREDENTIAL_SECRET_NAME), $(KB_CLUSTER_COMP_NAME)
	// - name: KB_POD_FQDN
	//      value: $(KB_POD_NAME).$(KB_CLUSTER_COMP_NAME)-headless.$(KB_NAMESPACE).svc
	//
	// - name: MYSQL_ROOT_USER
	//      valueFrom:
	//        secretKeyRef:
	//          key: username
	//          name: $(CONN_CREDENTIAL_SECRET_NAME)
	// var := "$(KB_POD_NAME).$(KB_CLUSTER_COMP_NAME)-headless.$(KB_NAMESPACE).svc"
	//
	// loop reference
	// - name: LOOP_REF_A
	//   value: $(LOOP_REF_B)
	// - name: LOOP_REF_B
	//   value: $(LOOP_REF_A)

	if len(value) == 0 || strings.IndexByte(value, '$') < 0 {
		return value, nil
	}
	envHolderVec := envPlaceHolderRegexp.FindAllString(value, -1)
	if len(envHolderVec) == 0 {
		return value, nil
	}
	return w.doEnvReplace(set.NewLinkedHashSetString(envHolderVec...), value, container)
}

func (w *envWrapper) doEnvReplace(replacedVars *set.LinkedHashSetString, oldValue string, container *corev1.Container) (string, error) {
	var (
		clusterName   = w.clusterName
		clusterUID    = w.clusterUID
		componentName = w.componentName
		builtInEnvMap = component.GetReplacementMapForBuiltInEnv(clusterName, clusterUID, componentName)
	)

	builtInEnvMap[constant.KBConnCredentialPlaceHolder] = component.GenerateConnCredential(clusterName)
	kbInnerEnvReplaceFn := func(envName string, strToReplace string) string {
		return strings.ReplaceAll(strToReplace, envName, builtInEnvMap[envName])
	}

	if !w.incAndCheckReferenceCount() {
		return "", cfgcore.MakeError("too many reference count, maybe there is a cycled reference: [%s] more than %d times ", oldValue, w.referenceCount)
	}

	replacedValue := oldValue
	for envHolder := range replacedVars.Iter() {
		if len(envHolder) <= 3 {
			continue
		}
		if _, ok := builtInEnvMap[envHolder]; ok {
			replacedValue = kbInnerEnvReplaceFn(envHolder, replacedValue)
			continue
		}
		envName := envHolder[2 : len(envHolder)-1]
		envValue, err := w.getEnvByName(container, envName)
		if err != nil {
			w.decReferenceCount()
			return envValue, err
		}
		replacedValue = strings.ReplaceAll(replacedValue, envHolder, envValue)
	}
	w.decReferenceCount()
	return replacedValue, nil
}

func (w *envWrapper) incReferenceCount() {
	w.referenceCount++
}

func (w *envWrapper) decReferenceCount() {
	w.referenceCount--
}

func (w *envWrapper) incAndCheckReferenceCount() bool {
	w.incReferenceCount()
	return w.referenceCount <= maxReferenceCount
}

func getResourceObject[T generics.Object, PT generics.PObject[T]](w *envWrapper, obj PT, key coreclient.ObjectKey) (PT, error) {
	gvk := generics.ToGVK(obj)
	object := w.getResourceFromLocal(key, gvk)
	if object != nil {
		if v, ok := object.(PT); ok {
			return v, nil
		}
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

func resourceRefValue(resourceRef *corev1.ResourceFieldSelector, containers []corev1.Container, curContainer *corev1.Container) (string, error) {
	if resourceRef.ContainerName == "" {
		return containerResourceRefValue(resourceRef, curContainer)
	}
	for _, v := range containers {
		if v.Name == resourceRef.ContainerName {
			return containerResourceRefValue(resourceRef, &v)
		}
	}
	return "", cfgcore.MakeError("not found named[%s] container", resourceRef.ContainerName)
}

func containerResourceRefValue(fieldSelector *corev1.ResourceFieldSelector, c *corev1.Container) (string, error) {
	return resource.ExtractContainerResourceValue(fieldSelector, c)
}

func fieldRefValue(podReference *corev1.ObjectFieldSelector, podSpec *corev1.PodSpec) (string, error) {
	return "", cfgcore.MakeError("not support pod field ref")
}
