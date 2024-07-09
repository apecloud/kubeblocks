/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/generics"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var (
	varReferenceRegExp = regexp.MustCompile(`\$\(([^)]+)\)`)
	varTemplate        = template.New("vars").Option("missingkey=error").Funcs(sprig.TxtFuncMap())
)

const builtinClusterDomain = "ClusterDomain"

func VarReferenceRegExp() *regexp.Regexp {
	return varReferenceRegExp
}

// ResolveTemplateNEnvVars resolves all built-in and user-defined vars for config template and Env usage.
func ResolveTemplateNEnvVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, definedVars []appsv1alpha1.EnvVar) (map[string]any, []corev1.EnvVar, error) {
	return resolveTemplateNEnvVars(ctx, cli, synthesizedComp, definedVars, false)
}

func ResolveEnvVars4LegacyCluster(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, definedVars []appsv1alpha1.EnvVar) (map[string]any, []corev1.EnvVar, error) {
	return resolveTemplateNEnvVars(ctx, cli, synthesizedComp, definedVars, true)
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
	definedVars []appsv1alpha1.EnvVar, legacy bool) (map[string]any, []corev1.EnvVar, error) {
	templateVars, envVars, err := resolveNewTemplateNEnvVars(ctx, cli, synthesizedComp, definedVars)
	if err != nil {
		return nil, nil, err
	}

	implicitEnvVars, err := buildLegacyImplicitEnvVars(synthesizedComp, legacy)
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

func buildLegacyImplicitEnvVars(synthesizedComp *SynthesizedComponent, legacy bool) ([]corev1.EnvVar, error) {
	envVars := make([]corev1.EnvVar, 0)
	envVars = append(envVars, buildDefaultEnvVars(synthesizedComp, legacy)...)
	envVars = append(envVars, buildEnv4TLS(synthesizedComp)...)
	userDefinedVars, err := buildEnv4UserDefined(synthesizedComp.Annotations)
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
	if err = evaluateObjectVarsExpression(definedVars, vars2, &vars); err != nil {
		return nil, nil, err
	}
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
	matches := varReferenceRegExp.FindAllStringSubmatchIndex(v.Value, -1)
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
	clusterCompName := func() string {
		return constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name)
	}()
	if legacy {
		vars = append(vars, []corev1.EnvVar{
			{Name: constant.KBEnvClusterName, Value: synthesizedComp.ClusterName},
			{Name: constant.KBEnvCompName, Value: synthesizedComp.Name},
			{Name: constant.KBEnvClusterCompName, Value: clusterCompName},
			{Name: constant.KBEnvClusterUIDPostfix8Deprecated, Value: clusterUIDPostfix(synthesizedComp)},
			{Name: constant.KBEnvPodFQDN, Value: fmt.Sprintf("%s.%s-headless.%s.svc", constant.EnvPlaceHolder(constant.KBEnvPodName), constant.EnvPlaceHolder(constant.KBEnvClusterCompName), constant.EnvPlaceHolder(constant.KBEnvNamespace))}}...)
	} else {
		vars = append(vars, corev1.EnvVar{
			Name:  constant.KBEnvPodFQDN,
			Value: fmt.Sprintf("%s.%s-headless.%s.svc", constant.EnvPlaceHolder(constant.KBEnvPodName), clusterCompName, constant.EnvPlaceHolder(constant.KBEnvNamespace)),
		})
	}
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

func evaluateObjectVarsExpression(definedVars []appsv1alpha1.EnvVar, credentialVars []corev1.EnvVar, vars *[]corev1.EnvVar) error {
	var (
		isValues = make(map[string]bool)
		values   = make(map[string]string)
	)
	normalize := func(name string) string {
		return strings.ReplaceAll(name, "-", "_")
	}
	for _, v := range [][]corev1.EnvVar{*vars, credentialVars} {
		values[builtinClusterDomain] = viper.GetString(constant.KubernetesClusterDomainEnv)
		for _, vv := range v {
			if vv.ValueFrom == nil {
				isValues[vv.Name] = true
				values[normalize(vv.Name)] = vv.Value
			} else {
				isValues[vv.Name] = false
			}
		}
	}

	evaluable := func(v appsv1alpha1.EnvVar) bool {
		if v.Expression == nil || len(*v.Expression) == 0 {
			return false
		}
		isValue, ok := isValues[v.Name]
		// !ok is for vars that defined and resolved successfully, but have nil value.
		return !ok || isValue
	}

	update := func(name, value string) {
		if val, exist := values[normalize(name)]; exist {
			if val != value {
				for i := range *vars {
					if (*vars)[i].Name == name {
						(*vars)[i].Value = value
						break
					}
				}
			}
		} else {
			// TODO: insert the var to keep orders?
			*vars = append(*vars, corev1.EnvVar{Name: name, Value: value})
		}
		values[normalize(name)] = value
	}

	eval := func(v appsv1alpha1.EnvVar) error {
		if !evaluable(v) {
			return nil
		}
		tpl, err := varTemplate.Parse(*v.Expression)
		if err != nil {
			return err
		}
		var buf strings.Builder
		if err = tpl.Execute(&buf, values); err != nil {
			return err
		}
		update(v.Name, buf.String())
		return nil
	}

	for _, v := range definedVars {
		if err := eval(v); err != nil {
			return err
		}
	}
	return nil
}

func resolveClusterObjectRefVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedVars []appsv1alpha1.EnvVar) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	if synthesizedComp == nil {
		return nil, nil, nil
	}
	vars1, vars2 := make([]corev1.EnvVar, 0), make([]corev1.EnvVar, 0)
	for _, v := range definedVars {
		switch {
		case v.ValueFrom != nil:
			var1, var2, err := resolveClusterObjectVarRef(ctx, cli, synthesizedComp, v.Name, *v.ValueFrom, v)
			if err != nil {
				return nil, nil, err
			}
			vars1 = append(vars1, var1...)
			vars2 = append(vars2, var2...)
		case len(v.Value) > 0:
			vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: v.Value})
		default:
			vars1 = append(vars1, corev1.EnvVar{Name: v.Name, Value: ""})
		}
	}
	return vars1, vars2, nil
}

// resolveClusterObjectVarRef resolves vars referred from cluster objects, returns the resolved non-credential and credential vars respectively.
func resolveClusterObjectVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, source appsv1alpha1.VarSource, ext ...any) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	switch {
	case source.ConfigMapKeyRef != nil:
		return resolveConfigMapKeyRef(ctx, cli, synthesizedComp, defineKey, *source.ConfigMapKeyRef)
	case source.SecretKeyRef != nil:
		return resolveSecretKeyRef(ctx, cli, synthesizedComp, defineKey, *source.SecretKeyRef)
	case source.HostNetworkVarRef != nil:
		return resolveHostNetworkVarRef(ctx, cli, synthesizedComp, defineKey, *source.HostNetworkVarRef, ext...)
	case source.ServiceVarRef != nil:
		return resolveServiceVarRef(ctx, cli, synthesizedComp, defineKey, *source.ServiceVarRef)
	case source.CredentialVarRef != nil:
		return resolveCredentialVarRef(ctx, cli, synthesizedComp, defineKey, *source.CredentialVarRef)
	case source.ServiceRefVarRef != nil:
		return resolveServiceRefVarRef(ctx, cli, synthesizedComp, defineKey, *source.ServiceRefVarRef)
	case source.ComponentVarRef != nil:
		return resolveComponentVarRef(ctx, cli, synthesizedComp, defineKey, *source.ComponentVarRef)
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
	return checkNBuildVars([]*corev1.EnvVar{var1}, []*corev1.EnvVar{var2}, err)
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
	return checkNBuildVars([]*corev1.EnvVar{var1}, []*corev1.EnvVar{var2}, err)
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
	if err := cli.Get(ctx, objKey, obj, inDataContext()); err != nil {
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

func resolveHostNetworkVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.HostNetworkVarSelector, ext ...any) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.HostNetworkVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error)
	switch {
	case selector.Container != nil && selector.Container.Port != nil:
		resolveFunc = resolveHostNetworkPortRef
	default:
		return nil, nil, nil
	}
	v1, _, err := resolveFunc(ctx, cli, synthesizedComp, defineKey, selector)
	// HACK: back-off to use v.Value if specified
	if v1 == nil && err == nil && len(ext) > 0 {
		v := ext[0].(appsv1alpha1.EnvVar)
		if len(v.Value) > 0 {
			v1 = []*corev1.EnvVar{{Name: v.Name, Value: v.Value}}
		}
	}
	return checkNBuildVars(v1, nil, err)
}

func resolveHostNetworkPortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.HostNetworkVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolvePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		compName := obj.(string)
		port, _ := getHostNetworkPort(ctx, cli, synthesizedComp.ClusterName, compName, selector.Container.Name, selector.Container.Port.Name)
		if port > 0 {
			return &corev1.EnvVar{
				Name:  defineKey,
				Value: strconv.Itoa(int(port)),
			}, nil, nil
		}
		return nil, nil, nil
	}
	return resolveHostNetworkVarRefLow(ctx, cli, synthesizedComp, selector, selector.Container.Port.Option, resolvePort)
}

func resolveServiceVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.ServiceVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error)
	switch {
	case selector.Host != nil && selector.LoadBalancer != nil:
		resolveFunc = resolveServiceHostOrLoadBalancerRefAdaptive
	case selector.Host != nil:
		resolveFunc = resolveServiceHostRef
	case selector.Port != nil:
		resolveFunc = resolveServicePortRef
	case selector.LoadBalancer != nil:
		resolveFunc = resolveServiceLoadBalancerRef
	default:
		return nil, nil, nil
	}
	return checkNBuildVars(resolveFunc(ctx, cli, synthesizedComp, defineKey, selector))
}

type resolvedServiceObj struct {
	service     *corev1.Service
	podServices []*corev1.Service
}

func resolveServiceHostRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveHost := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		return &corev1.EnvVar{Name: defineKey, Value: composeHostValueFromServices(obj)}, nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, resolveHost)
}

func composeHostValueFromServices(obj any) string {
	robj := obj.(*resolvedServiceObj)
	services := []*corev1.Service{robj.service}
	if robj.podServices != nil {
		services = robj.podServices
	}

	svcNames := make([]string, 0)
	for _, svc := range services {
		svcNames = append(svcNames, svc.Name)
	}
	slices.Sort(svcNames)

	return strings.Join(svcNames, ",")
}

func resolveServicePortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolvePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		port := composePortValueFromServices(obj, selector.Port.Name)
		if port == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{Name: defineKey, Value: *port}, nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Port.Option, resolvePort)
}

func composePortValueFromServices(obj any, targetPortName string) *string {
	hasNodePort := func(svc *corev1.Service, svcPort corev1.ServicePort) bool {
		return svc.Spec.Type == corev1.ServiceTypeNodePort ||
			svc.Spec.Type == corev1.ServiceTypeLoadBalancer && svc.Spec.AllocateLoadBalancerNodePorts != nil && *svc.Spec.AllocateLoadBalancerNodePorts
	}

	selector := func(services []*corev1.Service) map[string]string {
		ports := make(map[string]string)
		insert := func(svc *corev1.Service, svcPort corev1.ServicePort) {
			port := svcPort.Port
			if hasNodePort(svc, svcPort) {
				port = svcPort.NodePort
			}
			if port > 0 {
				ports[svc.Name] = strconv.Itoa(int(port))
			}
		}
		for _, svc := range services {
			for _, svcPort := range svc.Spec.Ports {
				if svcPort.Name == targetPortName {
					insert(svc, svcPort)
					break
				}
			}
			if len(svc.Spec.Ports) == 1 && (len(svc.Spec.Ports[0].Name) == 0 || len(targetPortName) == 0) {
				insert(svc, svc.Spec.Ports[0])
			}
		}
		return ports
	}
	return composeNamedValueFromServices(obj, selector)
}

func resolveServiceLoadBalancerRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveLoadBalancer := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		points := composeLoadBalancerValueFromServices(obj)
		if points == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{Name: defineKey, Value: *points}, nil, nil
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, resolveLoadBalancer)
}

func composeLoadBalancerValueFromServices(obj any) *string {
	selector := func(services []*corev1.Service) map[string]string {
		points := make(map[string]string)
		for _, svc := range services {
			if svc.Spec.Type != corev1.ServiceTypeLoadBalancer || len(svc.Status.LoadBalancer.Ingress) == 0 {
				break
			}
			ingress := svc.Status.LoadBalancer.Ingress[0]
			if len(ingress.IP) == 0 && len(ingress.Hostname) == 0 {
				break
			}
			if len(ingress.IP) > 0 {
				points[svc.Name] = ingress.IP
			} else {
				points[svc.Name] = ingress.Hostname
			}
		}
		return points
	}
	return composeNamedValueFromServices(obj, selector)
}

func composeNamedValueFromServices(obj any, selector func([]*corev1.Service) map[string]string) *string {
	robj := obj.(*resolvedServiceObj)
	services := []*corev1.Service{robj.service}
	if robj.podServices != nil {
		services = robj.podServices
	}

	values := selector(services)
	if len(values) == 0 || len(values) != len(services) {
		return nil
	}

	svcNames := maps.Keys(values)
	slices.Sort(svcNames)

	nameless := func() []string {
		var vals []string
		for _, svcName := range svcNames {
			vals = append(vals, values[svcName])
		}
		return vals
	}
	named := func() []string {
		var vals []string
		for _, svcName := range svcNames {
			vals = append(vals, fmt.Sprintf("%s:%s", svcName, values[svcName]))
		}
		return vals
	}

	value := ""
	if robj.podServices == nil {
		value = nameless()[0]
	} else {
		value = strings.Join(named(), ",")
	}
	return &value
}

func resolveServiceHostOrLoadBalancerRefAdaptive(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	host := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		return &corev1.EnvVar{Name: defineKey, Value: composeHostValueFromServices(obj)}, nil, nil
	}
	loadBalancer := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		points := composeLoadBalancerValueFromServices(obj)
		if points == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{Name: defineKey, Value: *points}, nil, nil
	}
	adaptive := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		hasLBService := func() bool {
			robj := obj.(*resolvedServiceObj)
			services := []*corev1.Service{robj.service}
			if robj.podServices != nil {
				services = robj.podServices
			}
			for _, svc := range services {
				if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
					return true
				}
			}
			return false
		}
		if hasLBService() {
			return loadBalancer(obj)
		}
		return host(obj)
	}
	return resolveServiceVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, adaptive)
}

func resolveCredentialVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.CredentialVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error)
	switch {
	case selector.Username != nil:
		resolveFunc = resolveCredentialUsernameRef
	case selector.Password != nil:
		resolveFunc = resolveCredentialPasswordRef
	default:
		return nil, nil, nil
	}
	return checkNBuildVars(resolveFunc(ctx, cli, synthesizedComp, defineKey, selector))
}

func resolveCredentialUsernameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveUsername := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
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
				}, nil
			}
		}
		return nil, nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveCredentialPasswordRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.CredentialVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolvePassword := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
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
				}, nil
			}
		}
		return nil, nil, nil
	}
	return resolveCredentialVarRefLow(ctx, cli, synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveServiceRefVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error)
	switch {
	case selector.Endpoint != nil:
		resolveFunc = resolveServiceRefEndpointRef
	case selector.Host != nil:
		resolveFunc = resolveServiceRefHostRef
	case selector.Port != nil:
		resolveFunc = resolveServiceRefPortRef
	case selector.Username != nil:
		resolveFunc = resolveServiceRefUsernameRef
	case selector.Password != nil:
		resolveFunc = resolveServiceRefPasswordRef
	default:
		return nil, nil, nil
	}
	return checkNBuildVars(resolveFunc(ctx, cli, synthesizedComp, defineKey, selector))
}

func resolveServiceRefEndpointRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveEndpoint := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Endpoint == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: sd.Spec.Endpoint.Value,
		}, nil, nil
	}
	return resolveServiceRefVarRefLow(ctx, cli, synthesizedComp, selector, selector.Endpoint, resolveEndpoint)
}

func resolveServiceRefHostRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveHost := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Host == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: sd.Spec.Host.Value,
		}, nil, nil
	}
	return resolveServiceRefVarRefLow(ctx, cli, synthesizedComp, selector, selector.Host, resolveHost)
}

func resolveServiceRefPortRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolvePort := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Port == nil {
			return nil, nil, nil
		}
		return &corev1.EnvVar{
			Name:  defineKey,
			Value: sd.Spec.Port.Value,
		}, nil, nil
	}
	return resolveServiceRefVarRefLow(ctx, cli, synthesizedComp, selector, selector.Port, resolvePort)
}

func resolveServiceRefUsernameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveUsername := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Username == nil {
			return nil, nil, nil
		}
		if sd.Spec.Auth.Username.ValueFrom != nil {
			valueFrom := *sd.Spec.Auth.Username.ValueFrom
			return nil, &corev1.EnvVar{Name: defineKey, ValueFrom: &valueFrom}, nil
		}
		// back-off to use .Value
		return nil, &corev1.EnvVar{Name: defineKey, Value: sd.Spec.Auth.Username.Value}, nil
	}
	return resolveServiceRefVarRefLow(ctx, cli, synthesizedComp, selector, selector.Username, resolveUsername)
}

func resolveServiceRefPasswordRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ServiceRefVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolvePassword := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		sd := obj.(*appsv1alpha1.ServiceDescriptor)
		if sd.Spec.Auth == nil || sd.Spec.Auth.Password == nil {
			return nil, nil, nil
		}
		if sd.Spec.Auth.Password.ValueFrom != nil {
			valueFrom := *sd.Spec.Auth.Password.ValueFrom
			return nil, &corev1.EnvVar{Name: defineKey, ValueFrom: &valueFrom}, nil
		}
		// back-off to use .Value
		return nil, &corev1.EnvVar{Name: defineKey, Value: sd.Spec.Auth.Password.Value}, nil
	}
	return resolveServiceRefVarRefLow(ctx, cli, synthesizedComp, selector, selector.Password, resolvePassword)
}

func resolveHostNetworkVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.HostNetworkVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveObjs := func() (map[string]any, error) {
		getter := func(compName string) (any, error) {
			enabled, err := isHostNetworkEnabled(ctx, cli, synthesizedComp, compName)
			if err != nil {
				return nil, err
			}
			if enabled {
				return compName, nil
			}
			return nil, nil
		}
		return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, getter)
	}
	return resolveClusterObjectVars("HostNetwork", selector.ClusterObjectReference, option, resolveObjs, resolveVar)
}

func resolveServiceVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveObjs := func() (map[string]any, error) {
		headlessGetter := func(compName string) (any, error) {
			return headlessCompServiceGetter(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, compName)
		}
		getter := func(compName string) (any, error) {
			return compServiceGetter(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, compName, selector.Name)
		}
		if selector.Name == "headless" {
			return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, headlessGetter)
		}
		return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, getter)
	}
	return resolveClusterObjectVars("Service", selector.ClusterObjectReference, option, resolveObjs, resolveVar)
}

func clusterServiceGetter(ctx context.Context, cli client.Reader, namespace, clusterName, name string) (any, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      constant.GenerateClusterServiceName(clusterName, name),
	}
	obj := &corev1.Service{}
	err := cli.Get(ctx, key, obj, inDataContext())
	return &resolvedServiceObj{service: obj}, err
}

func compServiceGetter(ctx context.Context, cli client.Reader, namespace, clusterName, compName, name string) (any, error) {
	svcName, err := func() (string, error) {
		if len(name) == 0 {
			return constant.GenerateDefaultComponentServiceName(clusterName, compName), nil
		}

		// resolve service name from referenced component definition
		_, compDef, err := GetCompNCompDefByName(ctx, cli, namespace, FullName(clusterName, compName))
		if err != nil {
			return "", err
		}
		for _, svc := range compDef.Spec.Services {
			if svc.Name == name {
				return constant.GenerateComponentServiceName(clusterName, compName, svc.ServiceName), nil
			}
		}
		return "", fmt.Errorf("service %s not defined in the component definition that component %s used", name, compName)
	}()
	if err != nil {
		return nil, err
	}

	key := types.NamespacedName{
		Namespace: namespace,
		Name:      svcName,
	}
	obj := &corev1.Service{}
	err = cli.Get(ctx, key, obj, inDataContext())
	if err == nil {
		return &resolvedServiceObj{service: obj}, nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	// fall-back to list services and find the matched prefix
	svcList := &corev1.ServiceList{}
	matchingLabels := client.MatchingLabels(constant.GetComponentWellKnownLabels(clusterName, compName))
	err = cli.List(ctx, svcList, matchingLabels, inDataContext())
	if err != nil {
		return nil, err
	}
	objs := make([]*corev1.Service, 0)
	podServiceNamePrefix := fmt.Sprintf("%s-", svcName)
	for i, svc := range svcList.Items {
		if strings.HasPrefix(svc.Name, podServiceNamePrefix) {
			objs = append(objs, &svcList.Items[i])
		}
	}
	if len(objs) == 0 {
		return nil, apierrors.NewNotFound(corev1.Resource("service"), name)
	}
	return &resolvedServiceObj{podServices: objs}, nil
}

func headlessCompServiceGetter(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) (any, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      constant.GenerateDefaultComponentHeadlessServiceName(clusterName, compName),
	}
	obj := &corev1.Service{}
	err := cli.Get(ctx, key, obj, inDataContext())
	return &resolvedServiceObj{service: obj}, err
}

func resolveCredentialVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.CredentialVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveObjs := func() (map[string]any, error) {
		getter := func(compName string) (any, error) {
			key := types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName, selector.Name),
			}
			obj := &corev1.Secret{}
			err := cli.Get(ctx, key, obj, inDataContext())
			return obj, err
		}
		return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, getter)
	}
	return resolveClusterObjectVars("Credential", selector.ClusterObjectReference, option, resolveObjs, resolveVar)
}

func resolveServiceRefVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ServiceRefVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveObjs := func() (map[string]any, error) {
		getter := func(compName string) (any, error) {
			if compName == synthesizedComp.Name {
				if synthesizedComp.ServiceReferences == nil {
					return nil, nil
				}
				return synthesizedComp.ServiceReferences[selector.Name], nil
			}
			// TODO: service ref about other components?
			return nil, nil
		}
		return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, getter)
	}
	return resolveClusterObjectVars("ServiceRef", selector.ClusterObjectReference, option, resolveObjs, resolveVar)
}

func resolveComponentVarRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ComponentVarSelector) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	var resolveFunc func(context.Context, client.Reader, *SynthesizedComponent, string, appsv1alpha1.ComponentVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error)
	switch {
	case selector.ComponentName != nil:
		resolveFunc = resolveComponentNameRef
	case selector.Replicas != nil:
		resolveFunc = resolveComponentReplicasRef
	case selector.InstanceNames != nil:
		resolveFunc = resolveComponentInstanceNamesRef
	case selector.PodFQDNs != nil:
		resolveFunc = resolveComponentPodFQDNsRef
	default:
		return nil, nil, nil
	}
	return checkNBuildVars(resolveFunc(ctx, cli, synthesizedComp, defineKey, selector))
}

func resolveComponentNameRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ComponentVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveComponentName := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		comp := obj.(*appsv1alpha1.Component)
		return &corev1.EnvVar{Name: defineKey, Value: comp.Name}, nil, nil
	}
	return resolveComponentVarRefLow(ctx, cli, synthesizedComp, selector, selector.ComponentName, resolveComponentName)
}

func resolveComponentReplicasRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ComponentVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveReplicas := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		comp := obj.(*appsv1alpha1.Component)
		return &corev1.EnvVar{Name: defineKey, Value: strconv.Itoa(int(comp.Spec.Replicas))}, nil, nil
	}
	return resolveComponentVarRefLow(ctx, cli, synthesizedComp, selector, selector.Replicas, resolveReplicas)
}

func resolveComponentInstanceNamesRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ComponentVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveInstanceNames := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		comp := obj.(*appsv1alpha1.Component)
		var templates []instanceset.InstanceTemplate
		for i := range comp.Spec.Instances {
			templates = append(templates, toV1InstanceTemplate(comp.Spec.Instances[i]))
		}
		instanceNameList, err := instanceset.GenerateAllInstanceNames(comp.Name, comp.Spec.Replicas, templates, comp.Spec.OfflineInstances, workloads.Ordinals{})
		if err != nil {
			return nil, nil, err
		}
		return &corev1.EnvVar{Name: defineKey, Value: strings.Join(instanceNameList, ",")}, nil, nil
	}
	return resolveComponentVarRefLow(ctx, cli, synthesizedComp, selector, selector.InstanceNames, resolveInstanceNames)
}

func resolveComponentPodFQDNsRef(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	defineKey string, selector appsv1alpha1.ComponentVarSelector) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveFQDNList := func(obj any) (*corev1.EnvVar, *corev1.EnvVar, error) {
		comp := obj.(*appsv1alpha1.Component)
		var templates []instanceset.InstanceTemplate
		for i := range comp.Spec.Instances {
			templates = append(templates, toV1InstanceTemplate(comp.Spec.Instances[i]))
		}
		clusterDomainFn := func(name string) string {
			return fmt.Sprintf("%s.%s", name, viper.GetString(constant.KubernetesClusterDomainEnv))
		}
		names, err := instanceset.GenerateAllInstanceNames(comp.Name, comp.Spec.Replicas, templates, comp.Spec.OfflineInstances, workloads.Ordinals{})
		if err != nil {
			return nil, nil, err
		}
		fqdn := func(name string) string {
			return clusterDomainFn(fmt.Sprintf("%s.%s-headless.%s.svc", name, comp.Name, synthesizedComp.Namespace))
		}
		for i := range names {
			names[i] = fqdn(names[i])
		}
		return &corev1.EnvVar{Name: defineKey, Value: strings.Join(names, ",")}, nil, nil
	}
	return resolveComponentVarRefLow(ctx, cli, synthesizedComp, selector, selector.PodFQDNs, resolveFQDNList)
}

func resolveComponentVarRefLow(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	selector appsv1alpha1.ComponentVarSelector, option *appsv1alpha1.VarOption, resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	resolveObjs := func() (map[string]any, error) {
		getter := func(compName string) (any, error) {
			key := types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, compName),
			}
			obj := &appsv1alpha1.Component{}
			err := cli.Get(ctx, key, obj, inDataContext())
			return obj, err
		}
		return resolveReferentObjects(synthesizedComp, selector.ClusterObjectReference, getter)
	}
	return resolveClusterObjectVars("Component", selector.ClusterObjectReference, option, resolveObjs, resolveVar)
}

func resolveReferentObjects(synthesizedComp *SynthesizedComponent,
	objRef appsv1alpha1.ClusterObjectReference, getter func(string) (any, error)) (map[string]any, error) {
	compNames, err := resolveReferentComponents(synthesizedComp, objRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	objs := make(map[string]any)
	for _, compName := range compNames {
		obj, err := getter(compName)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if apierrors.IsNotFound(err) {
			objs[compName] = nil
		} else {
			objs[compName] = obj
		}
	}
	return objs, nil
}

func resolveReferentComponents(synthesizedComp *SynthesizedComponent, objRef appsv1alpha1.ClusterObjectReference) ([]string, error) {
	// nolint:gocritic
	compDefMatched := func(def, defRef string) bool {
		return strings.HasPrefix(def, defRef) // prefix match
	}

	// match the current component when the multiple cluster object option not set
	if len(objRef.CompDef) == 0 || (compDefMatched(synthesizedComp.CompDefName, objRef.CompDef) && objRef.MultipleClusterObjectOption == nil) {
		return []string{synthesizedComp.Name}, nil
	}

	compNames := make([]string, 0)
	for k, v := range synthesizedComp.Comp2CompDefs {
		if compDefMatched(v, objRef.CompDef) {
			compNames = append(compNames, k)
		}
	}
	switch len(compNames) {
	case 1:
		return compNames, nil
	case 0:
		return nil, apierrors.NewNotFound(schema.GroupResource{}, "") // the error msg is trivial
	default:
		if objRef.MultipleClusterObjectOption == nil {
			return nil, fmt.Errorf("more than one referent component found: %s", strings.Join(compNames, ","))
		} else {
			return compNames, nil
		}
	}
}

func resolveClusterObjectVars(kind string, objRef appsv1alpha1.ClusterObjectReference, option *appsv1alpha1.VarOption,
	resolveObjs func() (map[string]any, error), resolveVar func(any) (*corev1.EnvVar, *corev1.EnvVar, error)) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	objOptional := func() bool {
		return objRef.Optional != nil && *objRef.Optional
	}
	varOptional := func() bool {
		return option != nil && *option == appsv1alpha1.VarOptional
	}

	objs, err := resolveObjs()
	if err != nil {
		return nil, nil, fmt.Errorf("resolving vars from %s object %s error: %s", kind, objRef.Name, err.Error())
	}
	switch {
	case objOptional() && isAllNil(objs):
		return nil, nil, nil
	case !objOptional() && (len(objs) == 0 || isHasNil(objs)):
		return nil, nil, fmt.Errorf("has no %s object %s found when resolving vars", kind, objRef.Name)
	}

	vars1, vars2 := make(map[string]*corev1.EnvVar), make(map[string]*corev1.EnvVar)
	for compName, obj := range objs {
		if obj == nil {
			vars1[compName], vars2[compName] = nil, nil
		} else {
			var1, var2, err := resolveVar(obj)
			if err != nil {
				return nil, nil, err
			}
			if var1 == nil && var2 == nil {
				if !varOptional() {
					return nil, nil, fmt.Errorf("the required var is not found in %s object %s", kind, objRef.Name)
				}
			}
			vars1[compName], vars2[compName] = var1, var2
		}
	}
	if len(objs) <= 1 {
		return maps.Values(vars1), maps.Values(vars2), nil
	}
	return handleMultipleClusterObjectVars(objRef, vars1, vars2)
}

func handleMultipleClusterObjectVars(objRef appsv1alpha1.ClusterObjectReference,
	vars1 map[string]*corev1.EnvVar, vars2 map[string]*corev1.EnvVar) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	strategy := objRef.MultipleClusterObjectOption.Strategy
	switch strategy {
	case appsv1alpha1.MultipleClusterObjectStrategyIndividual:
		return handleMultipleClusterObjectVarsIndividual(vars1, vars2)
	case appsv1alpha1.MultipleClusterObjectStrategyCombined:
		return handleMultipleClusterObjectVarsCombined(objRef, vars1, vars2)
	default:
		return nil, nil, fmt.Errorf("unknown multiple cluster objects strategy: %s", strategy)
	}
}

func handleMultipleClusterObjectVarsIndividual(vars1, vars2 map[string]*corev1.EnvVar) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	buildIndividualVars := func(vars map[string]*corev1.EnvVar) []*corev1.EnvVar {
		if isAllVarsNil(vars) {
			return nil
		}
		definedKey := definedKeyFromVars(vars)
		newVarName := func(compName string) string {
			return fmt.Sprintf("%s_%s", definedKey, strings.ToUpper(strings.ReplaceAll(compName, "-", "_")))
		}
		updateVarName := func(compName string, v *corev1.EnvVar) *corev1.EnvVar {
			v.Name = newVarName(compName)
			return v
		}
		allVars := []*corev1.EnvVar{newDummyVar(definedKey)}
		for _, compName := range orderedComps(vars) {
			v := vars[compName]
			if v == nil {
				allVars = append(allVars, newDummyVar(newVarName(compName)))
			} else {
				allVars = append(allVars, updateVarName(compName, v))
			}
		}
		return allVars
	}
	return buildIndividualVars(vars1), buildIndividualVars(vars2), nil
}

func handleMultipleClusterObjectVarsCombined(objRef appsv1alpha1.ClusterObjectReference,
	vars1, vars2 map[string]*corev1.EnvVar) ([]*corev1.EnvVar, []*corev1.EnvVar, error) {
	value1, value2, err := multipleClusterObjectVarsCombinedValue(objRef, vars1, vars2)
	if err != nil {
		return nil, nil, err
	}

	opt := objRef.MultipleClusterObjectOption.CombinedOption
	combinedVars := func(vars map[string]*corev1.EnvVar, value *string) []*corev1.EnvVar {
		if isAllVarsNil(vars) {
			return nil
		}
		definedKey := definedKeyFromVars(vars)
		reuseVar := func() *corev1.EnvVar {
			return &corev1.EnvVar{
				Name:  definedKey,
				Value: *value,
			}
		}
		newVar := func() *corev1.EnvVar {
			return &corev1.EnvVar{
				Name:  fmt.Sprintf("%s_%s", definedKey, *opt.NewVarSuffix),
				Value: *value,
			}
		}
		if opt == nil || opt.NewVarSuffix == nil {
			return []*corev1.EnvVar{reuseVar()}
		}
		return []*corev1.EnvVar{newDummyVar(definedKey), newVar()}
	}
	return combinedVars(vars1, value1), combinedVars(vars2, value2), nil
}

func multipleClusterObjectVarsCombinedValue(objRef appsv1alpha1.ClusterObjectReference,
	vars1, vars2 map[string]*corev1.EnvVar) (*string, *string, error) {
	var (
		pairDelimiter   = ","
		keyValDelimiter = ":"
	)
	opt := objRef.MultipleClusterObjectOption.CombinedOption
	if opt != nil && opt.FlattenFormat != nil {
		pairDelimiter = opt.FlattenFormat.Delimiter
		keyValDelimiter = opt.FlattenFormat.KeyValueDelimiter
	}

	composeVars := func(vars map[string]*corev1.EnvVar) (*string, error) {
		if isAllVarsNil(vars) {
			return nil, nil
		}
		values := make([]string, 0)
		for _, compName := range orderedComps(vars) {
			v := vars[compName]
			if v != nil && v.ValueFrom != nil {
				return nil, fmt.Errorf("combined strategy doesn't support vars with valueFrom values, var: %s, component: %s", v.Name, compName)
			}
			if v == nil {
				values = append(values, compName+keyValDelimiter)
			} else {
				values = append(values, compName+keyValDelimiter+v.Value)
			}
		}
		value := strings.Join(values, pairDelimiter)
		return &value, nil
	}

	value1, err1 := composeVars(vars1)
	if err1 != nil {
		return nil, nil, err1
	}
	value2, err2 := composeVars(vars2)
	if err2 != nil {
		return nil, nil, err2
	}
	return value1, value2, nil
}

func checkNBuildVars(pvars1, pvars2 []*corev1.EnvVar, err error) ([]corev1.EnvVar, []corev1.EnvVar, error) {
	if err != nil {
		return nil, nil, err
	}
	vars1, vars2 := make([]corev1.EnvVar, 0), make([]corev1.EnvVar, 0)
	for i := range pvars1 {
		if pvars1[i] != nil {
			vars1 = append(vars1, *pvars1[i])
		}
	}
	for i := range pvars2 {
		if pvars2[i] != nil {
			vars2 = append(vars2, *pvars2[i])
		}
	}
	return vars1, vars2, nil
}

func isAllNil(objs map[string]any) bool {
	isNil := func(o any) bool {
		return o == nil
	}
	return generics.CountFunc(maps.Values(objs), isNil) == len(objs)
}

func isHasNil(objs map[string]any) bool {
	isNil := func(o any) bool {
		return o == nil
	}
	return generics.CountFunc(maps.Values(objs), isNil) > 0
}

func isAllVarsNil(vars map[string]*corev1.EnvVar) bool {
	isNil := func(v *corev1.EnvVar) bool {
		return v == nil
	}
	return len(vars) == 0 || generics.CountFunc(maps.Values(vars), isNil) == len(vars)
}

func orderedComps(vars map[string]*corev1.EnvVar) []string {
	compNames := maps.Keys(vars)
	slices.Sort(compNames)
	return compNames
}

func definedKeyFromVars(vars map[string]*corev1.EnvVar) string {
	for _, v := range maps.Values(vars) {
		if v != nil {
			return v.Name
		}
	}
	panic("runtime error: all vars are nil")
}

func newDummyVar(name string) *corev1.EnvVar {
	return &corev1.EnvVar{
		Name:      name,
		Value:     "",
		ValueFrom: nil,
	}
}
