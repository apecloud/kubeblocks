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

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func setEnvNTemplateVars(templateVars map[string]any, envVars []corev1.EnvVar, synthesizedComp *SynthesizedComponent) {
	synthesizedComp.TemplateVars = templateVars

	for _, cc := range []*[]corev1.Container{
		&synthesizedComp.PodSpec.InitContainers,
		&synthesizedComp.PodSpec.Containers,
	} {
		for i := range *cc {
			// have injected variables placed at the front of the slice
			c := &(*cc)[i]
			if c.Env == nil {
				c.Env = envVars
			} else {
				c.Env = append(envVars, c.Env...)
			}
		}
	}
}

func resolveEnvNTemplateVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	annotations map[string]string, definedEnvVars []appsv1alpha1.EnvVar) (map[string]any, []corev1.EnvVar, error) {
	templateVars, credentialVars, err := resolveTemplateVars(ctx, cli, synthesizedComp, definedEnvVars)
	if err != nil {
		return nil, nil, err
	}

	envVars := make([]corev1.EnvVar, 0)
	envVars = append(envVars, templateVars2EnvVar(templateVars)...)
	envVars = append(envVars, templateVars2EnvVar(credentialVars)...)
	envVars = append(envVars, buildDefaultEnv()...)
	envVars = append(envVars, buildEnv4TLS(synthesizedComp)...)
	vars, err := buildEnv4UserDefined(annotations)
	if err != nil {
		return nil, nil, err
	}
	envVars = append(envVars, vars...)
	// TODO: remove this later
	envVars = append(envVars, synthesizedComp.ComponentRefEnvs...)

	return templateVars, envVars, nil
}

func templateVars2EnvVar(vars map[string]any) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)
	for name, val := range vars {
		value := ""
		if val != nil {
			value = val.(string)
		}
		envVars = append(envVars, corev1.EnvVar{Name: name, Value: value})
	}
	return envVars
}

func resolveTemplateVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedEnvVars []appsv1alpha1.EnvVar) (map[string]any, map[string]any, error) {
	vars := builtinTemplateVars(synthesizedComp)
	vars1, vars2, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, definedEnvVars)
	if err != nil {
		return nil, nil, err
	}
	maps.Copy(vars, vars1)
	return vars, vars2, nil
}

func builtinTemplateVars(synthesizedComp *SynthesizedComponent) map[string]any {
	var kbClusterPostfix8 string
	if len(synthesizedComp.ClusterUID) > 8 {
		kbClusterPostfix8 = synthesizedComp.ClusterUID[len(synthesizedComp.ClusterUID)-8:]
	} else {
		kbClusterPostfix8 = synthesizedComp.ClusterUID
	}
	if synthesizedComp != nil {
		return map[string]any{
			constant.KBEnvNamespace:                    synthesizedComp.Namespace,
			constant.KBEnvClusterName:                  synthesizedComp.ClusterName,
			constant.KBEnvClusterUID:                   synthesizedComp.ClusterUID,
			constant.KBEnvClusterCompName:              constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name),
			constant.KBEnvCompName:                     synthesizedComp.Name,
			constant.KBEnvCompReplicas:                 strconv.Itoa(int(synthesizedComp.Replicas)),
			constant.KBEnvClusterUIDPostfix8Deprecated: kbClusterPostfix8,
		}
	}
	return nil
}

func buildDefaultEnv() []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	// can not use map, it is unordered
	namedFields := []struct {
		name      string
		fieldPath string
	}{
		{name: constant.KBEnvPodName, fieldPath: "metadata.name"},
		{name: constant.KBEnvPodUID, fieldPath: "metadata.uid"},
		{name: constant.KBEnvPodIP, fieldPath: "status.podIP"},
		{name: constant.KBEnvPodIPs, fieldPath: "status.podIPs"},
		{name: constant.KBEnvNodeName, fieldPath: "spec.nodeName"},
		{name: constant.KBEnvHostIP, fieldPath: "status.hostIP"},
		{name: constant.KBEnvServiceAccountName, fieldPath: "spec.serviceAccountName"},
		// deprecated
		{name: constant.KBEnvHostIPDeprecated, fieldPath: "status.hostIP"},
		{name: constant.KBEnvPodIPDeprecated, fieldPath: "status.podIP"},
		{name: constant.KBEnvPodIPsDeprecated, fieldPath: "status.podIPs"},
	}
	for _, v := range namedFields {
		vars = append(vars, corev1.EnvVar{
			Name: v.name,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  v.fieldPath,
				},
			},
		})
	}
	vars = append(vars, corev1.EnvVar{
		Name: constant.KBEnvPodFQDN,
		Value: fmt.Sprintf("%s.%s-headless.%s.svc", constant.EnvPlaceHolder(constant.KBEnvPodName),
			constant.EnvPlaceHolder(constant.KBEnvClusterCompName), constant.EnvPlaceHolder(constant.KBEnvNamespace)),
	})
	return vars
}

func buildEnv4TLS(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	if synthesizedComp.TLSConfig == nil || !synthesizedComp.TLSConfig.Enable {
		return []corev1.EnvVar{}
	}
	return []corev1.EnvVar{
		{Name: constant.KBEnvTLSCertPath, Value: constant.MountPath},
		{Name: constant.KBEnvTLSCAFile, Value: constant.CAName},
		{Name: constant.KBEnvTLSCertFile, Value: constant.CertName},
		{Name: constant.KBEnvTLSKeyFile, Value: constant.KeyName},
	}
}

func buildEnv4UserDefined(annotations map[string]string) ([]corev1.EnvVar, error) {
	vars := make([]corev1.EnvVar, 0)
	if annotations == nil {
		return vars, nil
	}
	str, ok := annotations[constant.ExtraEnvAnnotationKey]
	if !ok {
		return vars, nil
	}

	udeMap := make(map[string]string)
	if err := json.Unmarshal([]byte(str), &udeMap); err != nil {
		return nil, err
	}
	keys := make([]string, 0)
	for k := range udeMap {
		if k == "" || udeMap[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		vars = append(vars, corev1.EnvVar{Name: k, Value: udeMap[k]})
	}
	return vars, nil
}

func resolveClusterObjectRefVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedEnvVars []appsv1alpha1.EnvVar) (map[string]any, map[string]any, error) {
	if synthesizedComp == nil {
		return nil, nil, nil
	}
	vars1, vars2 := map[string]any{}, map[string]any{}
	for _, v := range definedEnvVars {
		switch {
		case len(v.Value) > 0:
			vars1[v.Name] = v.Value
		case v.ValueFrom != nil:
			val1, val2, err := resolveClusterObjectRef(ctx, cli, synthesizedComp, *v.ValueFrom)
			if err != nil {
				return nil, nil, err
			}
			if val1 != nil {
				vars1[v.Name] = val1
			}
			if val2 != nil {
				vars2[v.Name] = val2
			}
		default:
			vars1[v.Name] = nil
		}
	}
	return vars1, vars2, nil
}

func resolveClusterObjectRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, source appsv1alpha1.EnvVarSource) (any, any, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(ctx, cli, synthesizedComp, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(ctx, cli, synthesizedComp, *source.SecretKeyRef)
	case source.ServiceKeyRef != nil:
		return resolveServiceKeyRef(ctx, cli, synthesizedComp, *source.ServiceKeyRef)
	case source.CredentialKeyRef != nil:
		return resolveCredentialKeyRef(ctx, cli, synthesizedComp, *source.CredentialKeyRef)
	case source.ServiceRefKeyRef != nil:
		return resolveServiceRefKeyRef(synthesizedComp, *source.ServiceRefKeyRef)
	}
	return nil, nil, nil
}

func resolveConfigMapKeyRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, selector corev1.ConfigMapKeySelector) (any, any, error) {
	return resolveNativeObjectKey(ctx, cli, synthesizedComp, &corev1.ConfigMap{},
		selector.Name, selector.Key, selector.Optional, func(obj client.Object) any {
			cm := obj.(*corev1.ConfigMap)
			if v, ok := cm.Data[selector.Key]; ok {
				return v
			}
			if v, ok := cm.BinaryData[selector.Key]; ok {
				return string(v)
			}
			return nil
		})
}

func resolveSecretKeyRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, selector corev1.SecretKeySelector) (any, any, error) {
	return resolveNativeObjectKey(ctx, cli, synthesizedComp, &corev1.Secret{},
		selector.Name, selector.Key, selector.Optional, func(obj client.Object) any {
			secret := obj.(*corev1.Secret)
			if v, ok := secret.Data[selector.Key]; ok {
				return string(v)
			}
			if v, ok := secret.StringData[selector.Key]; ok {
				return v
			}
			return nil
		})
}

func resolveNativeObjectKey(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	obj client.Object, objName, key string, optional *bool, resolve func(obj client.Object) any) (any, any, error) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	_optional := func() bool {
		return optional != nil && *optional
	}
	if len(objName) == 0 || len(key) == 0 {
		if _optional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("the name of %s object is empty when resolving vars", kind)
	}

	objKey := types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: objName}
	if err := cli.Get(ctx, objKey, obj); err != nil {
		if apierrors.IsNotFound(err) && _optional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("resolving vars from %s object %s error: %s", kind, objName, err.Error())
	}

	if v := resolve(obj); v != nil {
		return v, nil, nil
	}
	if _optional() {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("the required var is not found in %s object %s", kind, objName)
}

func resolveServiceKeyRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceKeySelector) (any, any, error) {
	if selector.Host != nil {
		return resolveServiceKeyHostRef(ctx, cli, synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServiceKeyPortRef(ctx, cli, synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveServiceKeyHostRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceKeySelector) (any, any, error) {
	return resolveServiceKeyRefLow(ctx, cli, synthesizedComp, selector, *selector.Host,
		func(svc corev1.Service) (any, any) {
			return svc.Name, nil
		})
}

func resolveServiceKeyPortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceKeySelector) (any, any, error) {
	return resolveServiceKeyRefLow(ctx, cli, synthesizedComp, selector, selector.Port.EnvKey,
		func(svc corev1.Service) (any, any) {
			for _, svcPort := range svc.Spec.Ports {
				if svcPort.Name == selector.Port.Name {
					return fmt.Sprintf("%d", svcPort.Port), nil
				}
			}
			return nil, nil
		})
}

func resolveCredentialKeyRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialKeySelector) (any, any, error) {
	if selector.Username != nil {
		return resolveCredentialKeyUsernameRef(ctx, cli, synthesizedComp, selector)
	}
	if selector.Password != nil {
		return resolveCredentialKeyPasswordRef(ctx, cli, synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveCredentialKeyUsernameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector) (any, any, error) {
	return resolveCredentialKeyRefLow(ctx, cli, synthesizedComp, selector, *selector.Username,
		func(secret corev1.Secret) (any, any) {
			if secret.Data == nil {
				if val, ok := secret.Data[constant.AccountNameForSecret]; ok {
					return nil, val
				}
			}
			return nil, nil
		})
}

func resolveCredentialKeyPasswordRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialKeySelector) (any, any, error) {
	return resolveCredentialKeyRefLow(ctx, cli, synthesizedComp, selector, *selector.Password,
		func(secret corev1.Secret) (any, any) {
			if secret.Data == nil {
				if val, ok := secret.Data[constant.AccountPasswdForSecret]; ok {
					return nil, val
				}
			}
			return nil, nil
		})
}

func resolveServiceRefKeyRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, any, error) {
	if selector.Endpoint != nil {
		return resolveServiceRefKeyEndpointRef(synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServiceRefKeyPortRef(synthesizedComp, selector)
	}
	if selector.Username != nil {
		return resolveServiceRefKeyUsernameRef(synthesizedComp, selector)
	}
	if selector.Password != nil {
		return resolveServiceRefKeyPasswordRef(synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveServiceRefKeyEndpointRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Endpoint,
		func(sd appsv1alpha1.ServiceDescriptor) (any, any) {
			if sd.Spec.Endpoint == nil {
				return nil, nil
			}
			return sd.Spec.Endpoint.Value, nil
		})
}

func resolveServiceRefKeyPortRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Port,
		func(sd appsv1alpha1.ServiceDescriptor) (any, any) {
			if sd.Spec.Port == nil {
				return nil, nil
			}
			return sd.Spec.Port.Value, nil
		})
}

func resolveServiceRefKeyUsernameRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Username,
		func(sd appsv1alpha1.ServiceDescriptor) (any, any) {
			if sd.Spec.Auth == nil || sd.Spec.Auth.Username == nil {
				return nil, nil
			}
			return nil, sd.Spec.Auth.Username.Value
		})
}

func resolveServiceRefKeyPasswordRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Password,
		func(sd appsv1alpha1.ServiceDescriptor) (any, any) {
			if sd.Spec.Auth == nil || sd.Spec.Auth.Password == nil {
				return nil, nil
			}
			return nil, sd.Spec.Auth.Password.Value
		})
}

func resolveServiceKeyRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceKeySelector, key appsv1alpha1.EnvKey, resolveKey func(corev1.Service) (any, any)) (any, any, error) {
	return resolveClusterObjectKeyRef("Service", selector.ClusterObjectReference, key,
		func() (any, error) {
			compName := selector.Component
			if len(compName) == 0 {
				compName = synthesizedComp.Name
			}
			svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName, selector.Name)
			if selector.Name == "headless" {
				svcName = constant.GenerateDefaultComponentHeadlessServiceName(synthesizedComp.ClusterName, compName)
			}
			svcKey := types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      svcName,
			}
			svc := &corev1.Service{}
			if err := cli.Get(ctx, svcKey, svc); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
			return svc, nil
		},
		func(obj any) (any, any) {
			return resolveKey(*(obj.(*corev1.Service)))
		})
}

func resolveCredentialKeyRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector,
	key appsv1alpha1.EnvKey, resolveKey func(corev1.Secret) (any, any)) (any, any, error) {
	return resolveClusterObjectKeyRef("Credential", selector.ClusterObjectReference, key,
		func() (any, error) {
			compName := selector.Component
			if len(compName) == 0 {
				compName = synthesizedComp.Name
			}
			secretKey := types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName, selector.Name),
			}
			secret := &corev1.Secret{}
			if err := cli.Get(ctx, secretKey, secret); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
			return secret, nil
		},
		func(obj any) (any, any) {
			return resolveKey(*(obj.(*corev1.Secret)))
		})
}

func resolveServiceRefKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector,
	key appsv1alpha1.EnvKey, resolveKey func(appsv1alpha1.ServiceDescriptor) (any, any)) (any, any, error) {
	return resolveClusterObjectKeyRef("ServiceRef", selector.ClusterObjectReference, key,
		func() (any, error) {
			if synthesizedComp.ServiceReferences == nil {
				return nil, nil
			}
			return synthesizedComp.ServiceReferences[selector.Name], nil
		},
		func(obj any) (any, any) {
			return resolveKey(*(obj.(*appsv1alpha1.ServiceDescriptor)))
		})
}

func resolveClusterObjectKeyRef(kind string, objRef appsv1alpha1.ClusterObjectReference, key appsv1alpha1.EnvKey,
	matchObj func() (any, error), resolveKey func(any) (any, any)) (any, any, error) {
	objOptional := func() bool {
		return objRef.Optional != nil && *objRef.Optional
	}
	keyOptional := func() bool {
		return key.Option != nil && *key.Option == appsv1alpha1.EnvKeyOptional
	}
	if len(objRef.Name) == 0 {
		if objOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("the name of %s object is empty when resolving vars", kind)
	}

	obj, err := matchObj()
	if err != nil {
		return nil, nil, fmt.Errorf("resolving vars from %s object %s error: %s", kind, objRef.Name, err.Error())
	}
	if obj == nil {
		if keyOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("%s object %s is not found when resolving vars", kind, objRef.Name)
	}

	val1, val2 := resolveKey(obj)
	if val1 == nil && val2 == nil {
		if keyOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("the required var is not found in %s object %s", kind, objRef.Name)
	}
	return val1, val2, nil
}
