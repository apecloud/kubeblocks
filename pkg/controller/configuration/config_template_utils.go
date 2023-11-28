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

package configuration

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func builtinObjects(builder *configTemplateBuilder) map[string]any {
	return map[string]any{
		builtinClusterObject:           builder.cluster,
		builtinComponentObject:         builder.component,
		builtinPodObject:               builder.podSpec,
		builtinComponentResourceObject: builder.componentValues.Resource,
		builtinClusterDomainObject:     viper.GetString(constant.KubernetesClusterDomainEnv),
	}
}

func builtinVars(builder *configTemplateBuilder) map[string]any {
	var (
		comp    = builder.component
		ordinal = 0 // TODO: ordinal
	)
	if comp != nil {
		return map[string]any{
			constant.KBEnvNamespace:         comp.Namespace,
			constant.KBEnvClusterName:       comp.ClusterName,
			constant.KBEnvClusterUID:        comp.ClusterUID,
			constant.KBEnvComponentName:     comp.Name,
			constant.KBEnvComponentReplicas: fmt.Sprintf("%d", comp.Replicas),
			constant.KBEnvPodName:           constant.GeneratePodName(comp.ClusterName, comp.Name, ordinal),
			constant.KBEnvPodFQDN:           constant.GeneratePodFQDN(comp.Namespace, comp.ClusterName, comp.Name, ordinal),
			constant.KBEnvPodOrdinal:        fmt.Sprintf("%d", ordinal),
		}
	}
	return nil
}

func resolveClusterObjectRefVars(builder *configTemplateBuilder) (map[string]any, error) {
	if builder.component == nil {
		return nil, nil
	}
	vars := map[string]any{}
	for _, v := range builder.component.Env {
		switch {
		case len(v.Value) > 0:
			vars[v.Name] = v.Value
		case v.ValueFrom != nil:
			val, err := resolveClusterObjectRef(builder, *v.ValueFrom)
			if err != nil {
				return nil, err
			}
			vars[v.Name] = val
		default:
			vars[v.Name] = nil // TODO: define the default behaviour?
		}
	}
	return vars, nil
}

func resolveClusterObjectRef(builder *configTemplateBuilder, source appsv1alpha1.EnvVarSource) (any, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(builder, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(builder, *source.SecretKeyRef)
	case source.ServiceKeyRef != nil:
		return resolveServiceKeyRef(builder, *source.ServiceKeyRef)
	case source.CredentialKeyRef != nil:
		return resolveCredentialKeyRef(builder, *source.CredentialKeyRef)
	case source.ServiceRefKeyRef != nil:
		return resolveServiceRefKeyRef(builder, *source.ServiceRefKeyRef)
	}
	return nil, nil
}

func resolveConfigMapKeyRef(builder *configTemplateBuilder, selector corev1.ConfigMapKeySelector) (any, error) {
	return resolveNativeObjectKeyRef(builder, &corev1.ConfigMap{}, selector.Name, selector.Key, selector.Optional,
		func(obj client.Object) any {
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

func resolveSecretKeyRef(builder *configTemplateBuilder, selector corev1.SecretKeySelector) (any, error) {
	return resolveNativeObjectKeyRef(builder, &corev1.Secret{}, selector.Name, selector.Key, selector.Optional,
		func(obj client.Object) any {
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

func resolveNativeObjectKeyRef(builder *configTemplateBuilder, obj client.Object, objName, key string, optional *bool,
	resolve func(obj client.Object) any) (any, error) {
	_optional := func() bool {
		return optional != nil && *optional
	}
	if len(objName) == 0 || len(key) == 0 {
		if _optional() {
			return nil, nil
		}
		return nil, fmt.Errorf("")
	}

	objKey := types.NamespacedName{Namespace: builder.namespace, Name: objName}
	if err := builder.cli.Get(builder.ctx, objKey, obj); err != nil {
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

func resolveServiceKeyRef(builder *configTemplateBuilder, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	if selector.Host != nil {
		return resolveServiceHostRef(builder, selector)
	}
	if selector.Port != nil {
		return resolveServicePortRef(builder, selector)
	}
	return nil, nil
}

func resolveServiceHostRef(builder *configTemplateBuilder, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	return resolveServiceKeyLow(builder, selector, *selector.Host, func(svc appsv1alpha1.Service) any {
		comp := builder.component
		return constant.GenerateComponentServiceName(comp.ClusterName, comp.Name, svc.ServiceName)
	})
}

func resolveServicePortRef(builder *configTemplateBuilder, selector appsv1alpha1.ServiceKeySelector) (any, error) {
	return resolveServiceKeyLow(builder, selector, selector.Port.EnvKey, func(svc appsv1alpha1.Service) any {
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name == selector.Port.Name {
				return fmt.Sprintf("%d", svcPort.Port)
			}
		}
		return nil
	})
}

func resolveServiceKeyLow(builder *configTemplateBuilder, selector appsv1alpha1.ServiceKeySelector, key appsv1alpha1.EnvKey,
	resolve func(appsv1alpha1.Service) any) (any, error) {
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

	comp := builder.component
	services := comp.ComponentServices
	if len(selector.Component) != 0 && selector.Component != comp.CompDefName {
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

func resolveCredentialKeyRef(*configTemplateBuilder, appsv1alpha1.CredentialKeySelector) (any, error) {
	return nil, nil
}

func resolveServiceRefKeyRef(builder *configTemplateBuilder, selector appsv1alpha1.ServiceRefKeySelector) (any, error) {
	if selector.Endpoint != nil {
		return resolveServiceRefKeyLow(builder, selector, *selector.Endpoint, func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Endpoint == nil {
				return nil
			}
			return sd.Spec.Endpoint.Value

		})
	}
	if selector.Port != nil {
		return resolveServiceRefKeyLow(builder, selector, *selector.Endpoint, func(sd appsv1alpha1.ServiceDescriptor) any {
			if sd.Spec.Port == nil {
				return nil
			}
			return sd.Spec.Port.Value
		})
	}
	return nil, nil
}

func resolveServiceRefKeyLow(builder *configTemplateBuilder, selector appsv1alpha1.ServiceRefKeySelector, key appsv1alpha1.EnvKey,
	resolve func(appsv1alpha1.ServiceDescriptor) any) (any, error) {
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

	comp := builder.component
	var svcRef *appsv1alpha1.ServiceDescriptor
	for name, ref := range comp.ServiceReferences {
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
