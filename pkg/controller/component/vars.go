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
	"fmt"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func buildPodSpecEnvVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) error {
	envVars := make([]corev1.EnvVar, 0)
	addEnvVars := func(vars map[string]any) {
		for name, val := range vars {
			value := ""
			if val != nil {
				value = val.(string)
			}
			envVars = append(envVars, corev1.EnvVar{Name: name, Value: value})
		}
	}

	addEnvVars(builtinVars(synthesizedComp))
	vars, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, true)
	if err != nil {
		return err
	}
	addEnvVars(vars)

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

func ResolveVars4Template(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent) (map[string]any, error) {
	vars := builtinVars(synthesizedComp)
	objVars, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, false)
	if err != nil {
		return nil, err
	}
	maps.Copy(vars, objVars)
	return vars, nil
}

func builtinVars(synthesizedComp *SynthesizedComponent) map[string]any {
	var (
		ordinal = 0 // TODO: ordinal
	)
	if synthesizedComp != nil {
		return map[string]any{
			constant.KBEnvNamespace:         synthesizedComp.Namespace,
			constant.KBEnvClusterName:       synthesizedComp.ClusterName,
			constant.KBEnvClusterUID:        synthesizedComp.ClusterUID,
			constant.KBEnvComponentName:     synthesizedComp.Name,
			constant.KBEnvComponentReplicas: fmt.Sprintf("%d", synthesizedComp.Replicas),
			constant.KBEnvPodName:           constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, ordinal),
			constant.KBEnvPodFQDN:           constant.GeneratePodFQDN(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, ordinal),
			constant.KBEnvPodOrdinal:        fmt.Sprintf("%d", ordinal),
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
	_optional := func() bool {
		return optional != nil && *optional
	}
	if len(objName) == 0 || len(key) == 0 {
		if _optional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	objKey := types.NamespacedName{Namespace: synthesizedComp.Namespace, Name: objName}
	if err := cli.Get(ctx, objKey, obj); err != nil {
		if apierrors.IsNotFound(err) && _optional() {
			return nil, nil
		}
		return nil, err
	}

	if v := resolve(obj); v != nil {
		return v, nil
	}
	if _optional() {
		return nil, nil
	}
	return nil, fmt.Errorf("")
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

func resolveServiceKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceKeySelector,
	key appsv1alpha1.EnvKey, resolve func(appsv1alpha1.Service) any) (any, error) {
	objOptional := func() bool {
		return selector.Optional != nil && *selector.Optional
	}
	keyOptional := func() bool {
		return key.Option != nil && *key.Option == appsv1alpha1.EnvKeyOptional
	}
	if len(selector.Name) == 0 {
		if objOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	services := synthesizedComp.ComponentServices
	if len(selector.Component) != 0 && selector.Component != synthesizedComp.CompDefName {
		services = []appsv1alpha1.Service{} // TODO: other component?
	}

	// TODO: default headless service
	var service *appsv1alpha1.Service
	for i, svc := range services {
		if svc.Name == selector.Name {
			service = &services[i]
			break
		}
	}
	if service == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	val := resolve(*service)
	if val == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}
	return val, nil
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

func resolveCredentialKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.CredentialKeySelector,
	key appsv1alpha1.EnvKey, resolve func(appsv1alpha1.SystemAccount) any) (any, error) {
	objOptional := func() bool {
		return selector.Optional != nil && *selector.Optional
	}
	keyOptional := func() bool {
		return key.Option != nil && *key.Option == appsv1alpha1.EnvKeyOptional
	}
	if len(selector.Name) == 0 {
		if objOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	var account *appsv1alpha1.SystemAccount
	for i, a := range synthesizedComp.SystemAccounts {
		if a.Name == selector.Name {
			account = &synthesizedComp.SystemAccounts[i]
			break
		}
	}
	if account == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	val := resolve(*account)
	if val == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}
	return val, nil
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

func resolveServiceRefKeyRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefKeySelector,
	key appsv1alpha1.EnvKey, resolve func(appsv1alpha1.ServiceDescriptor) any) (any, error) {
	objOptional := func() bool {
		return selector.Optional != nil && *selector.Optional
	}
	keyOptional := func() bool {
		return key.Option != nil && *key.Option == appsv1alpha1.EnvKeyOptional
	}
	if len(selector.Name) == 0 {
		if objOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	var svcRef *appsv1alpha1.ServiceDescriptor
	for name, ref := range synthesizedComp.ServiceReferences {
		if name == selector.Name {
			svcRef = ref
			break
		}
	}
	if svcRef == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	val := resolve(*svcRef)
	if val == nil {
		if keyOptional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}
	return val, nil
}
