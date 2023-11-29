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

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func ResolveVars4Template(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) (map[string]any, error) {
	vars := builtinVars(synthesizedComp)
	objVars, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, false)
	if err != nil {
		return nil, err
	}
	maps.Copy(vars, objVars)
	return vars, nil
}

func buildTemplatePodSpecEnv(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) error {
	envVars := make([]corev1.EnvVar, 0)

	addVars := func(vars map[string]any) {
		for name, val := range vars {
			value := ""
			if val != nil {
				value = val.(string)
			}
			envVars = append(envVars, corev1.EnvVar{Name: name, Value: value})
		}
	}
	addVars(builtinVars(synthesizedComp))
	vars, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, true)
	if err != nil {
		return err
	}
	addVars(vars)

	for _, build := range []func(*SynthesizedComponent) []corev1.EnvVar{
		// TODO: ude
		buildDefaultEnv, buildEnv4TLS /* buildEnv4UserDefined, */, buildEnv4CompRef,
	} {
		envVars = append(envVars, build(synthesizedComp)...)
	}

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
	return nil
}

func buildDefaultEnv(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	// can not use map, it is unordered
	namedFields := []struct {
		name      string
		fieldPath string
	}{
		{name: constant.KBEnvNamespace, fieldPath: "metadata.namespace"},
		{name: constant.KBEnvPodName, fieldPath: "metadata.name"},
		{name: constant.KBEnvPodUID, fieldPath: "metadata.uid"},
		{name: constant.KBEnvPodIP, fieldPath: "status.podIP"},
		{name: constant.KBEnvPodIPs, fieldPath: "status.podIPs"},
		{name: constant.KBEnvNodeName, fieldPath: "spec.nodeName"},
		{name: constant.KBEnvHostIP, fieldPath: "status.hostIP"},
		{name: constant.KBEnvServiceAccountName, fieldPath: "spec.serviceAccountName"},
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

	var kbClusterPostfix8 string
	if len(synthesizedComp.ClusterUID) > 8 {
		kbClusterPostfix8 = synthesizedComp.ClusterUID[len(synthesizedComp.ClusterUID)-8:]
	} else {
		kbClusterPostfix8 = synthesizedComp.ClusterUID
	}
	vars = append(vars, []corev1.EnvVar{
		{Name: constant.KBEnvClusterName, Value: synthesizedComp.ClusterName},
		{Name: constant.KBEnvCompName, Value: synthesizedComp.Name},
		{Name: constant.KBEnvClusterCompName, Value: synthesizedComp.ClusterName + "-" + synthesizedComp.Name},
		{Name: constant.KBEnvClusterUIDPostfix8Deprecated, Value: kbClusterPostfix8},
		{Name: constant.KBEnvPodFQDN, Value: fmt.Sprintf("%s.%s-headless.%s.svc",
			constant.EnvPlaceHolder(constant.KBEnvPodName),
			constant.EnvPlaceHolder(constant.KBEnvClusterCompName),
			constant.EnvPlaceHolder(constant.KBEnvNamespace))},
	}...)

	return vars
}

func buildEnv4TLS(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	if synthesizedComp.TLSConfig != nil && synthesizedComp.TLSConfig.Enable {
		return []corev1.EnvVar{
			{Name: constant.KBEnvTLSCertPath, Value: constant.MountPath},
			{Name: constant.KBEnvTLSCAFile, Value: constant.CAName},
			{Name: constant.KBEnvTLSCertFile, Value: constant.CertName},
			{Name: constant.KBEnvTLSKeyFile, Value: constant.KeyName},
		}
	}
	return []corev1.EnvVar{}
}

func buildEnv4UserDefined(cluster *appsv1alpha1.Cluster, synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	str, ok := cluster.Annotations[constant.ExtraEnvAnnotationKey]
	if !ok {
		return vars
	}

	udeMap := make(map[string]string)
	if err := json.Unmarshal([]byte(str), &udeMap); err != nil {
		return nil // TODO: error
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
	return vars
}

func buildEnv4CompRef(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	for _, env := range synthesizedComp.ComponentRefEnvs {
		vars = append(vars, *env)
	}
	return vars
}

func builtinVars(synthesizedComp *SynthesizedComponent) map[string]any {
	var (
		ordinal = 0 // TODO: ordinal
	)
	if synthesizedComp != nil {
		return map[string]any{
			constant.KBEnvNamespace:    synthesizedComp.Namespace,
			constant.KBEnvClusterName:  synthesizedComp.ClusterName,
			constant.KBEnvClusterUID:   synthesizedComp.ClusterUID,
			constant.KBEnvCompName:     synthesizedComp.Name,
			constant.KBEnvCompReplicas: fmt.Sprintf("%d", synthesizedComp.Replicas),
			constant.KBEnvPodName:      constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, ordinal),
			constant.KBEnvPodFQDN:      constant.GeneratePodFQDN(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, ordinal),
			constant.KBEnvPodOrdinal:   fmt.Sprintf("%d", ordinal),
		}
	}
	return nil
}

func resolveClusterObjectRefVars(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, resolveCredential bool) (map[string]any, error) {
	if synthesizedComp == nil {
		return nil, nil
	}
	vars := map[string]any{}
	for _, v := range synthesizedComp.Env {
		switch {
		case len(v.Value) > 0:
			vars[v.Name] = v.Value
		case v.ValueFrom != nil:
			val, err := resolveClusterObjectRef(ctx, cli, synthesizedComp, *v.ValueFrom, resolveCredential)
			if err != nil {
				return nil, err
			}
			if val != nil {
				vars[v.Name] = val
			}
		default:
			vars[v.Name] = nil
		}
	}
	return vars, nil
}

func resolveClusterObjectRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, source appsv1alpha1.EnvVarSource, resolveCredential bool) (any, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(ctx, cli, synthesizedComp, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(ctx, cli, synthesizedComp, *source.SecretKeyRef)
	case source.ServiceKeyRef != nil:
		return resolveServiceKeyRef(synthesizedComp, *source.ServiceKeyRef)
	case source.CredentialKeyRef != nil:
		return resolveCredentialKeyRef(synthesizedComp, *source.CredentialKeyRef, resolveCredential)
	case source.ServiceRefKeyRef != nil:
		return resolveServiceRefKeyRef(synthesizedComp, *source.ServiceRefKeyRef, resolveCredential)
	}
	return nil, nil
}

func resolveConfigMapKeyRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, selector corev1.ConfigMapKeySelector) (any, error) {
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
	synthesizedComp *SynthesizedComponent, selector corev1.SecretKeySelector) (any, error) {
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
	obj client.Object, objName, key string, optional *bool, resolve func(obj client.Object) any) (any, error) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	_optional := func() bool {
		return optional != nil && *optional
	}
	if len(objName) == 0 || len(key) == 0 {
		if _optional() {
			return nil, nil
		}
		return nil, fmt.Errorf("the name of %s object is empty when resolving vars", kind)
	}

	objKey := types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: objName}
	if err := cli.Get(ctx, objKey, obj); err != nil {
		if apierrors.IsNotFound(err) && _optional() {
			return nil, nil
		}
		return nil, fmt.Errorf("resolving vars from %s object %s error: %s", kind, objName, err.Error())
	}

	if v := resolve(obj); v != nil {
		return v, nil
	}
	if _optional() {
		return nil, nil
	}
	return nil, fmt.Errorf("the required var is not found in %s object %s", kind, objName)
}

func resolveServiceKeyRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	if selector.Host != nil {
		return resolveServiceKeyHostRef(synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServiceKeyPortRef(synthesizedComp, selector)
	}
	return nil, nil
}

func resolveServiceKeyHostRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	return resolveServiceKeyRefLow(synthesizedComp, selector, *selector.Host, func(svc appsv1alpha1.Service) any {
		return constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, svc.ServiceName)
	})
}

func resolveServiceKeyPortRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	return resolveServiceKeyRefLow(synthesizedComp, selector, selector.Port.EnvKey, func(svc appsv1alpha1.Service) any {
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name == selector.Port.Name {
				return fmt.Sprintf("%d", svcPort.Port)
			}
		}
		return nil
	})
}

func resolveCredentialKeyRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector,
	resolveCredential bool) (any, error) {
	if resolveCredential && selector.Username != nil {
		return resolveCredentialKeyUsernameRef(synthesizedComp, selector)
	}
	if resolveCredential && selector.Password != nil {
		return resolveCredentialKeyPasswordRef(synthesizedComp, selector)
	}
	return nil, nil
}

func resolveCredentialKeyUsernameRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector) (any, error) {
	return resolveCredentialKeyRefLow(synthesizedComp, selector, *selector.Username,
		func(account appsv1alpha1.SystemAccount) any {
			return account.Name
		})
}

func resolveCredentialKeyPasswordRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector) (any, error) {
	return resolveCredentialKeyRefLow(synthesizedComp, selector, *selector.Password,
		func(account appsv1alpha1.SystemAccount) any {
			return "" // TODO: get the password
		})
}

func resolveServiceRefKeyRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector,
	resolveCredential bool) (any, error) {
	if selector.Endpoint != nil {
		return resolveServiceRefKeyEndpointRef(synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServiceRefKeyPortRef(synthesizedComp, selector)
	}
	if resolveCredential && selector.Username != nil {
		return resolveServiceRefKeyUsernameRef(synthesizedComp, selector)
	}
	if resolveCredential && selector.Password != nil {
		return resolveServiceRefKeyPasswordRef(synthesizedComp, selector)
	}
	return nil, nil
}

func resolveServiceRefKeyEndpointRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Endpoint,
		func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Endpoint == nil {
				return nil
			}
			return sd.Spec.Endpoint.Value
		})
}

func resolveServiceRefKeyPortRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Port,
		func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Port == nil {
				return nil
			}
			return sd.Spec.Port.Value
		})
}

func resolveServiceRefKeyUsernameRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Username,
		func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Auth == nil || sd.Spec.Auth.Username == nil {
				return nil
			}
			return sd.Spec.Auth.Username.Value
		})
}

func resolveServiceRefKeyPasswordRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector) (any, error) {
	return resolveServiceRefKeyRefLow(synthesizedComp, selector, *selector.Password,
		func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Auth == nil || sd.Spec.Auth.Password == nil {
				return nil
			}
			return sd.Spec.Auth.Password.Value
		})
}

func resolveServiceKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceKeySelector,
	key appsv1alpha1.EnvKey, resolveKey func(appsv1alpha1.Service) any) (any, error) {
	return resolveClusterObjectKeyRef("Service", selector.ClusterObjectReference, key,
		func() any {
			services := synthesizedComp.ComponentServices
			if len(selector.Component) != 0 && selector.Component != synthesizedComp.CompDefName {
				services = []appsv1alpha1.Service{} // TODO: other component?
			}

			// TODO: default headless service
			for i, svc := range services {
				if svc.Name == selector.Name {
					return &services[i]
				}
			}
			return nil
		},
		func(obj any) any {
			return resolveKey(*(obj.(*appsv1alpha1.Service)))
		})
}

func resolveCredentialKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector,
	key appsv1alpha1.EnvKey, resolveKey func(appsv1alpha1.SystemAccount) any) (any, error) {
	return resolveClusterObjectKeyRef("Credential", selector.ClusterObjectReference, key,
		func() any {
			for i, account := range synthesizedComp.SystemAccounts {
				if account.Name == selector.Name {
					return &synthesizedComp.SystemAccounts[i]
				}
			}
			return nil
		},
		func(obj any) any {
			return resolveKey(*(obj.(*appsv1alpha1.SystemAccount)))
		})
}

func resolveServiceRefKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector,
	key appsv1alpha1.EnvKey, resolveKey func(appsv1alpha1.ServiceDescriptor) any) (any, error) {
	return resolveClusterObjectKeyRef("ServiceRef", selector.ClusterObjectReference, key,
		func() any {
			return synthesizedComp.ServiceReferences[selector.Name]
		},
		func(obj any) any {
			return resolveKey(*(obj.(*appsv1alpha1.ServiceDescriptor)))
		})
}

func resolveClusterObjectKeyRef(kind string, objRef appsv1alpha1.ClusterObjectReference, key appsv1alpha1.EnvKey,
	matchObj func() any, resolveKey func(any) any) (any, error) {
	objOptional := func() bool {
		return objRef.Optional != nil && *objRef.Optional
	}
	keyOptional := func() bool {
		return key.Option != nil && *key.Option == appsv1alpha1.EnvKeyOptional
	}
	if len(objRef.Name) == 0 {
		if objOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("the name of %s object is empty when resolving vars", kind)
	}

	obj := matchObj()
	if obj == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("%s object %s is not found when resolving vars", kind, objRef.Name)
	}

	val := resolveKey(obj)
	if val == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("the required var is not found in %s object %s", kind, objRef.Name)
	}
	return val, nil
}
