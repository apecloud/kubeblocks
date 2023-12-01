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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func ResolveEnvNTemplateVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	annotations map[string]string, definedVars []appsv1alpha1.EnvVar) error {
	templateVars, credentialVars, err := resolveBuiltinNObjectRefVars(ctx, cli, synthesizedComp, definedVars)
	if err != nil {
		return err
	}

	envVars := make([]corev1.EnvVar, 0)
	envVars = append(envVars, templateVars...)
	envVars = append(envVars, credentialVars...)
	envVars = append(envVars, buildDefaultEnv()...)
	envVars = append(envVars, buildEnv4TLS(synthesizedComp)...)
	vars, err := buildEnv4UserDefined(annotations)
	if err != nil {
		return err
	}
	envVars = append(envVars, vars...)
	// TODO: remove this later
	envVars = append(envVars, synthesizedComp.ComponentRefEnvs...)

	setEnvNTemplateVars(templateVars, envVars, synthesizedComp)

	return nil
}

func setEnvNTemplateVars(templateVars []corev1.EnvVar, envVars []corev1.EnvVar, synthesizedComp *SynthesizedComponent) {
	if synthesizedComp.TemplateVars == nil {
		synthesizedComp.TemplateVars = make(map[string]any)
	}
	for _, v := range templateVars {
		synthesizedComp.TemplateVars[v.Name] = v.Value
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
}

func resolveBuiltinNObjectRefVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedVars []appsv1alpha1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	vars := builtinTemplateVars(synthesizedComp)
	vars1, vars2, err := resolveClusterObjectRefVars(ctx, cli, synthesizedComp, definedVars)
	if err != nil {
		return nil, nil, err
	}
	vars = append(vars, vars1...)
	return vars, vars2, nil
}

func builtinTemplateVars(synthesizedComp *SynthesizedComponent) []corev1.EnvVar {
	var kbClusterPostfix8 string
	if len(synthesizedComp.ClusterUID) > 8 {
		kbClusterPostfix8 = synthesizedComp.ClusterUID[len(synthesizedComp.ClusterUID)-8:]
	} else {
		kbClusterPostfix8 = synthesizedComp.ClusterUID
	}
	if synthesizedComp != nil {
		return []corev1.EnvVar{
			{Name: constant.KBEnvNamespace, Value: synthesizedComp.Namespace},
			{Name: constant.KBEnvClusterName, Value: synthesizedComp.ClusterName},
			{Name: constant.KBEnvClusterUID, Value: synthesizedComp.ClusterUID},
			{Name: constant.KBEnvClusterCompName, Value: constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name)},
			{Name: constant.KBEnvCompName, Value: synthesizedComp.Name},
			{Name: constant.KBEnvCompReplicas, Value: strconv.Itoa(int(synthesizedComp.Replicas))},
			{Name: constant.KBEnvClusterUIDPostfix8Deprecated, Value: kbClusterPostfix8},
		}
	}
	return []corev1.EnvVar{}
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
	definedVars []appsv1alpha1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	if synthesizedComp == nil {
		return nil, nil, nil
	}
	vars1, vars2 := make([]corev1.EnvVar, 0), make([]corev1.EnvVar, 0)
	for _, v := range definedVars {
		switch {
		case len(v.Value) > 0:
			vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: v.Value})
		case v.ValueFrom != nil:
			val1, val2, err := resolveClusterObjectVarRef(ctx, cli, synthesizedComp, *v.ValueFrom)
			if err != nil {
				return nil, nil, err
			}
			if val1 != nil {
				vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: val1.(string)})
			}
			if val2 != nil {
				vars2 = append(vars2, corev1.EnvVar{Name: v.Name, Value: val2.(string)})
			}
		default:
			vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: ""})
		}
	}
	return vars1, vars2, nil
}

func resolveClusterObjectVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	source appsv1alpha1.VarSource) (any, any, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(ctx, cli, synthesizedComp, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(ctx, cli, synthesizedComp, *source.SecretKeyRef)
	case source.ServiceVarRef != nil:
		return resolveServiceVarRef(ctx, cli, synthesizedComp, *source.ServiceVarRef)
	case source.CredentialVarRef != nil:
		return resolveCredentialVarRef(ctx, cli, synthesizedComp, *source.CredentialVarRef)
	case source.ServiceRefVarRef != nil:
		return resolveServiceRefVarRef(synthesizedComp, *source.ServiceRefVarRef)
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

func resolveServiceVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector) (any, any, error) {
	if selector.Host != nil {
		return resolveServiceHostRef(ctx, cli, synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServicePortRef(ctx, cli, synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveServiceHostRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector) (any, any, error) {
	resolveHost := func(obj any) (any, any) {
		svc := obj.(*corev1.Service)
		return svc.Name, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, resolveHost)
}

func resolveServicePortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector) (any, any, error) {
	resolvePort := func(obj any) (any, any) {
		svc := obj.(*corev1.Service)
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name == selector.Port.Name {
				return strconv.Itoa(int(svcPort.Port)), nil
			}
		}
		return nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Port.Option, resolvePort)
}

func resolveCredentialVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector) (any, any, error) {
	if selector.Username != nil {
		return resolveCredentialUsernameRef(ctx, cli, synthesizedComp, selector)
	}
	if selector.Password != nil {
		return resolveCredentialPasswordRef(ctx, cli, synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveCredentialUsernameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector) (any, any, error) {
	resolveUsername := func(obj any) (any, any) {
		secret := obj.(*corev1.Secret)
		if secret.Data != nil {
			if val, ok := secret.Data[constant.AccountNameForSecret]; ok {
				return nil, string(val)
			}
		}
		return nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveCredentialPasswordRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector) (any, any, error) {
	resolvePassword := func(obj any) (any, any) {
		secret := obj.(*corev1.Secret)
		if secret.Data != nil {
			if val, ok := secret.Data[constant.AccountPasswdForSecret]; ok {
				return nil, string(val)
			}
		}
		return nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveServiceRefVarRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector) (any, any, error) {
	if selector.Endpoint != nil {
		return resolveServiceRefEndpointRef(synthesizedComp, selector)
	}
	if selector.Port != nil {
		return resolveServiceRefPortRef(synthesizedComp, selector)
	}
	if selector.Username != nil {
		return resolveServiceRefUsernameRef(synthesizedComp, selector)
	}
	if selector.Password != nil {
		return resolveServiceRefPasswordRef(synthesizedComp, selector)
	}
	return nil, nil, nil
}

func resolveServiceRefEndpointRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector) (any, any, error) {
	resolveEndpoint := func(obj any) (any, any) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Endpoint == nil {
			return nil, nil
		}
		return sd.Spec.Endpoint.Value, nil
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Endpoint, resolveEndpoint)
}

func resolveServiceRefPortRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector) (any, any, error) {
	resolvePort := func(obj any) (any, any) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Port == nil {
			return nil, nil
		}
		return sd.Spec.Port.Value, nil
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Port, resolvePort)
}

func resolveServiceRefUsernameRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector) (any, any, error) {
	resolveUsername := func(obj any) (any, any) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Username == nil {
			return nil, nil
		}
		return nil, sd.Spec.Auth.Username.Value
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveServiceRefPasswordRef(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector) (any, any, error) {
	resolvePassword := func(obj any) (any, any) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Password == nil {
			return nil, nil
		}
		return nil, sd.Spec.Auth.Password.Value
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveServiceVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (any, any)) (any, any, error) {
	resolveObj := func() (any, error) {
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
	}
	return resolveClusterObjectVar("Service", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveCredentialVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (any, any)) (any, any, error) {
	resolveObj := func() (any, error) {
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
	}
	return resolveClusterObjectVar("Credential", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveServiceRefVarRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector,
	option *appsv1alpha1.VarOption, resolveVar func(any) (any, any)) (any, any, error) {
	resolveObj := func() (any, error) {
		if synthesizedComp.ServiceReferences == nil {
			return nil, nil
		}
		return synthesizedComp.ServiceReferences[selector.Name], nil
	}
	return resolveClusterObjectVar("ServiceRef", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveClusterObjectVar(kind string, objRef appsv1alpha1.ClusterObjectReference, option *appsv1alpha1.VarOption,
	resolveObj func() (any, error), resolveVar func(any) (any, any)) (any, any, error) {
	objOptional := func() bool {
		return objRef.Optional != nil && *objRef.Optional
	}
	varOptional := func() bool {
		return option != nil && *option == appsv1alpha1.VarOptional
	}
	if len(objRef.Name) == 0 {
		if objOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("the name of %s object is empty when resolving vars", kind)
	}

	obj, err := resolveObj()
	if err != nil {
		return nil, nil, fmt.Errorf("resolving vars from %s object %s error: %s", kind, objRef.Name, err.Error())
	}
	if obj == nil {
		if varOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("%s object %s is not found when resolving vars", kind, objRef.Name)
	}

	val1, val2 := resolveVar(obj)
	if val1 == nil && val2 == nil {
		if varOptional() {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("the required var is not found in %s object %s", kind, objRef.Name)
	}
	return val1, val2, nil
}
