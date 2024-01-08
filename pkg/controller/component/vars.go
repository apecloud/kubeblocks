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
	"regexp"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// ResolveTemplateNEnvVars resolves all built-in and user-defined vars for config template and Env usage.
func ResolveTemplateNEnvVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	annotations map[string]string, definedVars []appsv1alpha1.EnvVar) (map[string]any, []corev1.EnvVar, error) {
	return resolveTemplateNEnvVars(ctx, cli, synthesizedComp, annotations, definedVars, false)
}

func ResolveEnvVars4LegacyCluster(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	annotations map[string]string, definedVars []appsv1alpha1.EnvVar) (map[string]any, []corev1.EnvVar, error) {
	return resolveTemplateNEnvVars(ctx, cli, synthesizedComp, annotations, definedVars, true)
}

func InjectEnvVars(synthesizedComp *SynthesizedComponent, envVars []corev1.EnvVar, envFromSources []corev1.EnvFromSource) {
	InjectEnvVars4Containers(synthesizedComp, envVars, envFromSources, nil)
}

func InjectEnvVars4Containers(synthesizedComp *SynthesizedComponent, envVars []corev1.EnvVar,
	envFromSources []corev1.EnvFromSource, filter func(container *corev1.Container) bool) {
	for _, cc := range []*[]corev1.Container{&synthesizedComp.PodSpec.InitContainers, &synthesizedComp.PodSpec.Containers} {
		for i := range *cc {
			// have injected variables placed at the front of the slice
			c := &(*cc)[i]
			if filter != nil && !filter(c) {
				continue
			}
			if envVars != nil {
				if c.Env == nil {
					newEnv := make([]corev1.EnvVar, len(envVars))
					copy(newEnv, envVars)
					c.Env = newEnv
				} else {
					newEnv := make([]corev1.EnvVar, len(envVars), common.SafeAddInt(len(c.Env), len(envVars)))
					copy(newEnv, envVars)
					newEnv = append(newEnv, c.Env...)
					c.Env = newEnv
				}
			}
			if envFromSources != nil {
				if c.EnvFrom == nil {
					c.EnvFrom = make([]corev1.EnvFromSource, 0)
				}
				c.EnvFrom = append(c.EnvFrom, envFromSources...)
			}
		}
	}
}

func resolveTemplateNEnvVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	annotations map[string]string, definedVars []appsv1alpha1.EnvVar, legacy bool) (map[string]any, []corev1.EnvVar, error) {
	templateVars, envVars, err := resolveNewTemplateNEnvVars(ctx, cli, synthesizedComp, definedVars)
	if err != nil {
		return nil, nil, err
	}

	implicitEnvVars, err := buildLegacyImplicitEnvVars(synthesizedComp, annotations, legacy)
	if err != nil {
		return nil, nil, err
	}

	if legacy {
		envVars = implicitEnvVars
	} else {
		// TODO: duplicated
		envVars = append(envVars, implicitEnvVars...)
	}

	formattedTemplateVars := func() map[string]any {
		vars := make(map[string]any)
		for _, v := range templateVars {
			vars[v.Name] = v.Value
		}
		return vars
	}
	return formattedTemplateVars(), envVars, nil
}

func resolveNewTemplateNEnvVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedVars []appsv1alpha1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	vars, credentialVars, err := resolveBuiltinNObjectRefVars(ctx, cli, synthesizedComp, definedVars)
	if err != nil {
		return nil, nil, err
	}
	envVars, templateVars := resolveVarsReferenceNEscaping(vars, credentialVars)
	return templateVars, append(envVars, credentialVars...), nil
}

func buildLegacyImplicitEnvVars(synthesizedComp *SynthesizedComponent, annotations map[string]string, legacy bool) ([]corev1.EnvVar, error) {
	envVars := make([]corev1.EnvVar, 0)
	envVars = append(envVars, buildDefaultEnvVars(synthesizedComp, legacy)...)
	envVars = append(envVars, buildEnv4TLS(synthesizedComp)...)
	userDefinedVars, err := buildEnv4UserDefined(annotations)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, userDefinedVars...)
	return envVars, nil
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
	if synthesizedComp != nil {
		return []corev1.EnvVar{
			{Name: constant.KBEnvNamespace, Value: synthesizedComp.Namespace},
			{Name: constant.KBEnvClusterName, Value: synthesizedComp.ClusterName},
			{Name: constant.KBEnvClusterUID, Value: synthesizedComp.ClusterUID},
			{Name: constant.KBEnvClusterCompName, Value: constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name)},
			{Name: constant.KBEnvCompName, Value: synthesizedComp.Name},
			{Name: constant.KBEnvCompReplicas, Value: strconv.Itoa(int(synthesizedComp.Replicas))},
			{Name: constant.KBEnvClusterUIDPostfix8Deprecated, Value: clusterUIDPostfix(synthesizedComp)},
		}
	}
	return []corev1.EnvVar{}
}

func clusterUIDPostfix(synthesizedComp *SynthesizedComponent) string {
	if len(synthesizedComp.ClusterUID) > 8 {
		return synthesizedComp.ClusterUID[len(synthesizedComp.ClusterUID)-8:]
	}
	return synthesizedComp.ClusterUID
}

func resolveVarsReferenceNEscaping(templateVars []corev1.EnvVar, credentialVars []corev1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar) {
	l2m := func(vars []corev1.EnvVar) map[string]corev1.EnvVar {
		m := make(map[string]corev1.EnvVar)
		for i, v := range vars {
			m[v.Name] = vars[i]
		}
		return m
	}
	templateVarsMapping := l2m(templateVars)
	credentialVarsMapping := l2m(credentialVars)

	vars1, vars2 := make([]corev1.EnvVar, len(templateVars)), make([]corev1.EnvVar, len(templateVars))
	for i := range templateVars {
		var1, var2 := resolveVarReferenceNEscaping(templateVarsMapping, credentialVarsMapping, &templateVars[i])
		vars1[i] = *var1
		vars2[i] = *var2
	}
	return vars1, vars2
}

func resolveVarReferenceNEscaping(templateVars, credentialVars map[string]corev1.EnvVar, v *corev1.EnvVar) (*corev1.EnvVar, *corev1.EnvVar) {
	if len(v.Value) == 0 {
		return v, v
	}
	re := regexp.MustCompile(`\$\(([^)]+)\)`)
	matches := re.FindAllStringSubmatchIndex(v.Value, -1)
	if len(matches) == 0 {
		return v, v
	}
	return resolveValueReferenceNEscaping(templateVars, credentialVars, *v, matches)
}

func resolveValueReferenceNEscaping(templateVars, credentialVars map[string]corev1.EnvVar,
	v corev1.EnvVar, matches [][]int) (*corev1.EnvVar, *corev1.EnvVar) {
	isEscapingMatch := func(match []int) bool {
		return match[0] > 0 && v.Value[match[0]-1] == '$'
	}
	resolveValue := func(match []int, resolveCredential bool) (string, *corev1.EnvVarSource) {
		if isEscapingMatch(match) {
			return v.Value[match[0]:match[1]], nil
		} else {
			name := v.Value[match[2]:match[3]]
			if vv, ok := templateVars[name]; ok {
				return vv.Value, nil
			}
			if resolveCredential {
				if vv, ok := credentialVars[name]; ok {
					if vv.ValueFrom == nil {
						return vv.Value, nil
					} else {
						// returns the token and matched valueFrom
						return v.Value[match[0]:match[1]], vv.ValueFrom
					}
				}
			}
			// not found
			return v.Value[match[0]:match[1]], nil
		}
	}

	tokens := make([]func(bool) (string, *corev1.EnvVarSource), 0)
	for idx, pos := 0, 0; pos < len(v.Value); idx++ {
		if idx >= len(matches) {
			lpos := pos
			tokens = append(tokens, func(bool) (string, *corev1.EnvVarSource) { return v.Value[lpos:len(v.Value)], nil })
			break
		}
		match := matches[idx]
		mpos := match[0]
		if isEscapingMatch(match) {
			mpos = match[0] - 1
		}
		if pos < mpos {
			lpos := pos
			tokens = append(tokens, func(bool) (string, *corev1.EnvVarSource) { return v.Value[lpos:mpos], nil })
		}
		tokens = append(tokens, func(credential bool) (string, *corev1.EnvVarSource) { return resolveValue(match, credential) })
		pos = match[1]
	}

	isFullyMatched := func() bool {
		return len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(v.Value)
	}

	buildValue := func(resolveCredential bool) (string, *corev1.EnvVarSource) {
		builder := strings.Builder{}
		for _, token := range tokens {
			value, valueFrom := token(resolveCredential)
			if valueFrom != nil && isFullyMatched() {
				return "", valueFrom
			} else {
				// matched as value, or valueFrom but cannot dereference
				builder.WriteString(value)
			}
		}
		return builder.String(), nil
	}

	v1, v2 := v.DeepCopy(), v.DeepCopy()
	v1.Value, v1.ValueFrom = buildValue(true)
	v2.Value, v2.ValueFrom = buildValue(false)
	return v1, v2
}

func buildDefaultEnvVars(synthesizedComp *SynthesizedComponent, legacy bool) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	// can not use map, it is unordered
	namedFields := []struct {
		name      string
		fieldPath string
	}{
		{name: constant.KBEnvPodName, fieldPath: "metadata.name"},
		{name: constant.KBEnvPodUID, fieldPath: "metadata.uid"},
		{name: constant.KBEnvNamespace, fieldPath: "metadata.namespace"},
		{name: constant.KBEnvServiceAccountName, fieldPath: "spec.serviceAccountName"},
		{name: constant.KBEnvNodeName, fieldPath: "spec.nodeName"},
		{name: constant.KBEnvHostIP, fieldPath: "status.hostIP"},
		{name: constant.KBEnvPodIP, fieldPath: "status.podIP"},
		{name: constant.KBEnvPodIPs, fieldPath: "status.podIPs"},
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
	if legacy {
		vars = append(vars, []corev1.EnvVar{
			{Name: constant.KBEnvClusterName, Value: synthesizedComp.ClusterName},
			{Name: constant.KBEnvCompName, Value: synthesizedComp.Name},
			{Name: constant.KBEnvClusterCompName, Value: constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name)},
			{Name: constant.KBEnvClusterUIDPostfix8Deprecated, Value: clusterUIDPostfix(synthesizedComp)}}...)
	}
	vars = append(vars, corev1.EnvVar{
		Name:  constant.KBEnvPodFQDN,
		Value: fmt.Sprintf("%s.%s-headless.%s.svc", constant.EnvPlaceHolder(constant.KBEnvPodName), constant.EnvPlaceHolder(constant.KBEnvClusterCompName), constant.EnvPlaceHolder(constant.KBEnvNamespace)),
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
			var1, var2, err := resolveClusterObjectVarRef(ctx, cli, synthesizedComp, v.Name, *v.ValueFrom)
			if err != nil {
				return nil, nil, err
			}
			vars1 = append(vars1, var1...)
			vars2 = append(vars2, var2...)
		default:
			vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: ""})
		}
	}
	return vars1, vars2, nil
}

// resolveClusterObjectVarRef resolves vars referred from cluster objects, returns the resolved non-credential and credential vars respectively.
func resolveClusterObjectVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, source appsv1alpha1.VarSource) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(ctx, cli, synthesizedComp, defineKey, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(ctx, cli, synthesizedComp, defineKey, *source.SecretKeyRef)
	case source.ServiceVarRef != nil:
		return resolveServiceVarRef(ctx, cli, synthesizedComp, defineKey, *source.ServiceVarRef)
	case source.CredentialVarRef != nil:
		return resolveCredentialVarRef(ctx, cli, synthesizedComp, defineKey, *source.CredentialVarRef)
	case source.ServiceRefVarRef != nil:
		return resolveServiceRefVarRef(synthesizedComp, defineKey, *source.ServiceRefVarRef)
	}
	return nil, nil, nil
}

func resolveConfigMapKeyRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, defineKey string, selector corev1.ConfigMapKeySelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var1, var2, err := resolveNativeObjectKey(ctx, cli, synthesizedComp, &corev1.ConfigMap{},
		selector.Name, selector.Key, selector.Optional, func(obj client.Object) (*corev1.EnvVar, *corev1.EnvVar) {
			cm := obj.(*corev1.ConfigMap)
			if v, ok := cm.Data[selector.Key]; ok {
				return &corev1.EnvVar{
					Name:  defineKey,
					Value: v,
				}, nil
			}
			if v, ok := cm.BinaryData[selector.Key]; ok {
				return &corev1.EnvVar{
					Name:  defineKey,
					Value: string(v),
				}, nil
			}
			return nil, nil
		})
	if err != nil {
		return nil, nil, err
	}
	return checkNBuildVars(var1, var2)
}

func resolveSecretKeyRef(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, defineKey string, selector corev1.SecretKeySelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var1, var2, err := resolveNativeObjectKey(ctx, cli, synthesizedComp, &corev1.Secret{},
		selector.Name, selector.Key, selector.Optional, func(obj client.Object) (*corev1.EnvVar, *corev1.EnvVar) {
			secret := obj.(*corev1.Secret)
			_, ok1 := secret.Data[selector.Key]
			_, ok2 := secret.StringData[selector.Key]
			if ok1 || ok2 {
				return nil, &corev1.EnvVar{
					Name: defineKey,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: selector.Key,
						},
					},
				}
			}
			return nil, nil
		})
	if err != nil {
		return nil, nil, err
	}
	return checkNBuildVars(var1, var2)
}

func resolveNativeObjectKey(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	obj client.Object, objName, key string, optional *bool, resolve func(obj client.Object) (*corev1.EnvVar, *corev1.EnvVar)) (*corev1.EnvVar, *corev1.EnvVar, error) {
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

	if v1, v2 := resolve(obj); v1 != nil || v2 != nil {
		return v1, v2, nil
	}
	if _optional() {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("the required var is not found in %s object %s", kind, objName)
}

func resolveServiceVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {

	// if serviceVarRef is defined with podOrdinalServiceVar, generate the pod ordinal service var
	if selector.GeneratePodOrdinalServiceVar {
		return resolveServiceVarRefWithPodOrdinal(ctx, cli, synthesizedComp, defineKey, selector)
	}

	var1, var2, err := resolveServiceVarRefWithSelector(ctx, cli, synthesizedComp, defineKey, selector)
	if err != nil {
		return nil, nil, err
	}
	return checkNBuildVars(var1, var2)
}

func resolveServiceVarRefWithPodOrdinal(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	if synthesizedComp == nil || synthesizedComp.Replicas == 0 {
		return nil, nil, nil
	}
	if len(selector.Name) == 0 || len(selector.CompDef) == 0 {
		return nil, nil, fmt.Errorf("the name and compDef of ServiceVarRef is required when generatePodOrdinalServiceVar is true")
	}

	vars1, vars2 := make([]corev1.EnvVar, 0, synthesizedComp.Replicas), make([]corev1.EnvVar, 0, synthesizedComp.Replicas)
	resolveAndAppend := func(defineKey string, serviceSelector appsv1alpha1.ServiceVarSelector) error {
		var1, var2, err := resolveServiceVarRefWithSelector(ctx, cli, synthesizedComp, defineKey, serviceSelector)
		if err != nil {
			return err
		}
		if var1 != nil {
			vars1 = append(vars1, *var1)
		}
		if var2 != nil {
			vars2 = append(vars2, *var2)
		}
		return nil
	}

	for i := int32(0); i < synthesizedComp.Replicas; i++ {
		genDefineKey := fmt.Sprintf("%s_%d", defineKey, i)
		genServiceSelector := selector.DeepCopy()
		if len(genServiceSelector.Name) > 0 {
			genServiceSelector.Name = fmt.Sprintf("%s-%d", genServiceSelector.Name, i)
		}
		err := resolveAndAppend(genDefineKey, *genServiceSelector)
		if err != nil {
			return nil, nil, err
		}
	}

	return vars1, vars2, nil
}

func resolveServiceVarRefWithSelector(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.ServiceVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error)
	switch {
	case selector.Host != nil:
		resolveFunc = resolveServiceHostRef
	case selector.Port != nil:
		resolveFunc = resolveServicePortRef
	case selector.NodePort != nil:
		resolveFunc = resolveServiceNodePortRef
	default:
		return nil, nil, nil
	}
	return resolveFunc(ctx, cli, synthesizedComp, defineKey, selector)
}

func resolveServiceHostRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveHost := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		svc := obj.(*corev1.Service)
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: svc.Name,
		}, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, resolveHost)
}

func resolveServicePortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolvePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		svc := obj.(*corev1.Service)
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.Name == selector.Port.Name {
				return &corev1.EnvVar{
					Name:  defineKey,
					Value: strconv.Itoa(int(svcPort.Port)),
				}, nil
			}
		}
		if len(svc.Spec.Ports) == 1 && (len(svc.Spec.Ports[0].Name) == 0 || len(selector.Port.Name) == 0) {
			return &corev1.EnvVar{
				Name:  defineKey,
				Value: strconv.Itoa(int(svc.Spec.Ports[0].Port)),
			}, nil
		}
		return nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Port.Option, resolvePort)
}

func resolveServiceNodePortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveNodePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		svc := obj.(*corev1.Service)
		if svc.Spec.Type != corev1.ServiceTypeNodePort {
			return nil, nil
		}
		for _, svcPort := range svc.Spec.Ports {
			if svcPort.NodePort == 0 {
				continue
			}
			if svcPort.Name == selector.NodePort.Name {
				return &corev1.EnvVar{
					Name:  defineKey,
					Value: strconv.Itoa(int(svcPort.NodePort)),
				}, nil
			}
		}
		return nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.NodePort.Option, resolveNodePort)
}

func resolveCredentialVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.CredentialVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error)
	switch {
	case selector.Username != nil:
		resolveFunc = resolveCredentialUsernameRef
	case selector.Password != nil:
		resolveFunc = resolveCredentialPasswordRef
	default:
		return nil, nil, nil
	}

	var1, var2, err := resolveFunc(ctx, cli, synthesizedComp, defineKey, selector)
	if err != nil {
		return nil, nil, err
	}
	return checkNBuildVars(var1, var2)
}

func resolveCredentialUsernameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveUsername := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		secret := obj.(*corev1.Secret)
		if secret.Data != nil {
			if _, ok := secret.Data[constant.AccountNameForSecret]; ok {
				return nil, &corev1.EnvVar{
					Name: defineKey,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: constant.AccountNameForSecret,
						},
					},
				}
			}
		}
		return nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveCredentialPasswordRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolvePassword := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		secret := obj.(*corev1.Secret)
		if secret.Data != nil {
			if _, ok := secret.Data[constant.AccountPasswdForSecret]; ok {
				return nil, &corev1.EnvVar{
					Name: defineKey,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secret.Name,
							},
							Key: constant.AccountPasswdForSecret,
						},
					},
				}
			}
		}
		return nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveServiceRefVarRef(synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(*SynthesizedComponent, string, appsv1alpha1.ServiceRefVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error)
	switch {
	case selector.Endpoint != nil:
		resolveFunc = resolveServiceRefEndpointRef
	case selector.Port != nil:
		resolveFunc = resolveServiceRefPortRef
	case selector.Username != nil:
		resolveFunc = resolveServiceRefUsernameRef
	case selector.Password != nil:
		resolveFunc = resolveServiceRefPasswordRef
	default:
		return nil, nil, nil
	}

	var1, var2, err := resolveFunc(synthesizedComp, defineKey, selector)
	if err != nil {
		return nil, nil, err
	}
	return checkNBuildVars(var1, var2)
}

func resolveServiceRefEndpointRef(synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceRefVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveEndpoint := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Endpoint == nil {
			return nil, nil
		}
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: sd.Spec.Endpoint.Value,
		}, nil
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Endpoint, resolveEndpoint)
}

func resolveServiceRefPortRef(synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceRefVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolvePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Port == nil {
			return nil, nil
		}
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: sd.Spec.Port.Value,
		}, nil
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Port, resolvePort)
}

func resolveServiceRefUsernameRef(synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceRefVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveUsername := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Username == nil {
			return nil, nil
		}
		if sd.Spec.Auth.Username.ValueFrom != nil {
			valueFrom := *sd.Spec.Auth.Username.ValueFrom
			return nil, &corev1.EnvVar{Name: defineKey, ValueFrom: &valueFrom}
		}
		// back-off to use .Value
		return nil, &corev1.EnvVar{Name: defineKey, Value: sd.Spec.Auth.Username.Value}
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveServiceRefPasswordRef(synthesizedComp *SynthesizedComponent, defineKey string, selector appsv1alpha1.ServiceRefVarSelector) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolvePassword := func(obj any) (*corev1.EnvVar, *corev1.EnvVar) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Password == nil {
			return nil, nil
		}
		if sd.Spec.Auth.Password.ValueFrom != nil {
			valueFrom := *sd.Spec.Auth.Password.ValueFrom
			return nil, &corev1.EnvVar{Name: defineKey, ValueFrom: &valueFrom}
		}
		// back-off to use .Value
		return nil, &corev1.EnvVar{Name: defineKey, Value: sd.Spec.Auth.Password.Value}
	}
	return resolveServiceRefVarRefLow(synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveServiceVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar)) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveObj := func() (any, error) {
		objName := func(compName string) string {
			svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName, selector.Name)
			if selector.Name == "headless" {
				svcName = constant.GenerateDefaultComponentHeadlessServiceName(synthesizedComp.ClusterName, compName)
			}
			return svcName
		}
		return resolveReferentObject(ctx, cli, synthesizedComp, selector.ClusterObjectReference, objName, &corev1.Service{})
	}
	return resolveClusterObjectVar("Service", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveCredentialVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar)) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveObj := func() (any, error) {
		objName := func(compName string) string {
			return constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName, selector.Name)
		}
		return resolveReferentObject(ctx, cli, synthesizedComp, selector.ClusterObjectReference, objName, &corev1.Secret{})
	}
	return resolveClusterObjectVar("Credential", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveServiceRefVarRefLow(synthesizedComp *SynthesizedComponent, selector appsv1alpha1.ServiceRefVarSelector,
	option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar)) (*corev1.EnvVar, *corev1.EnvVar, error) {
	resolveObj := func() (any, error) {
		if synthesizedComp.ServiceReferences == nil {
			return nil, nil
		}
		return synthesizedComp.ServiceReferences[selector.Name], nil
	}
	return resolveClusterObjectVar("ServiceRef", selector.ClusterObjectReference, option, resolveObj, resolveVar)
}

func resolveReferentObject(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	objRef appsv1alpha1.ClusterObjectReference, objName func(string) string, obj client.Object) (any, error) {
	compName, err := resolveReferentComponent(synthesizedComp, objRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	objKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      objName(compName),
	}
	if err = cli.Get(ctx, objKey, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func resolveReferentComponent(synthesizedComp *SynthesizedComponent, objRef appsv1alpha1.ClusterObjectReference) (string, error) {
	if len(objRef.CompDef) == 0 || objRef.CompDef == synthesizedComp.CompDefName {
		return synthesizedComp.Name, nil
	}
	compNames := make([]string, 0)
	for k, v := range synthesizedComp.Comp2CompDefs {
		if v == objRef.CompDef {
			compNames = append(compNames, k)
		}
	}
	switch len(compNames) {
	case 1:
		return compNames[0], nil
	case 0:
		return "", apierrors.NewNotFound(schema.GroupResource{}, "") // the error msg is trivial
	default:
		return "", fmt.Errorf("more than one referent component found: %s", strings.Join(compNames, ","))
	}
}

func resolveClusterObjectVar(kind string, objRef appsv1alpha1.ClusterObjectReference, option *appsv1alpha1.VarOption,
	resolveObj func() (any, error), resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar)) (*corev1.EnvVar, *corev1.EnvVar, error) {
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

func checkNBuildVars(var1, var2 *corev1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	vars1, vars2 := make([]corev1.EnvVar, 0), make([]corev1.EnvVar, 0)
	if var1 != nil {
		vars1 = append(vars1, *var1)
	}
	if var2 != nil {
		vars2 = append(vars2, *var2)
	}
	return vars1, vars2, nil
}
