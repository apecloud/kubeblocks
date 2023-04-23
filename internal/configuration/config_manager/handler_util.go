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

package configmanager

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
)

// CfgManagerBuildParams is the params for building config manager sidecar
type CfgManagerBuildParams struct {
	ManagerName string          `json:"name"`
	Image       string          `json:"sidecarImage"`
	Args        []string        `json:"args"`
	Envs        []corev1.EnvVar `json:"envs"`

	Volumes       []corev1.VolumeMount `json:"volumes"`
	CharacterType string               `json:"characterType"`
	SecreteName   string               `json:"secreteName"`
	EnvConfigName string               `json:"envConfigName"`

	// add volume to pod
	ScriptVolume           []corev1.Volume
	Cluster                *appsv1alpha1.Cluster
	ConfigSpecsBuildParams []ConfigSpecMeta
	// init tools container
	ToolsContainers []corev1.Container
}

func IsSupportReload(reload *appsv1alpha1.ReloadOptions) bool {
	return reload != nil && (reload.ShellTrigger != nil || reload.UnixSignalTrigger != nil || reload.TPLScriptTrigger != nil)
}

func FromReloadTypeConfig(reloadOptions *appsv1alpha1.ReloadOptions) appsv1alpha1.CfgReloadType {
	switch {
	case reloadOptions.UnixSignalTrigger != nil:
		return appsv1alpha1.UnixSignalType
	case reloadOptions.ShellTrigger != nil:
		return appsv1alpha1.ShellType
	case reloadOptions.TPLScriptTrigger != nil:
		return appsv1alpha1.TPLScriptType
	}
	return ""
}

func ValidateReloadOptions(reloadOptions *appsv1alpha1.ReloadOptions, cli client.Client, ctx context.Context) error {
	switch {
	case reloadOptions.UnixSignalTrigger != nil:
		return checkSignalTrigger(reloadOptions.UnixSignalTrigger)
	case reloadOptions.ShellTrigger != nil:
		return checkShellTrigger(reloadOptions.ShellTrigger)
	case reloadOptions.TPLScriptTrigger != nil:
		return checkTPLScriptTrigger(reloadOptions.TPLScriptTrigger, cli, ctx)
	}
	return cfgutil.MakeError("require special reload type!")
}

func checkTPLScriptTrigger(options *appsv1alpha1.TPLScriptTrigger, cli client.Client, ctx context.Context) error {
	cm := corev1.ConfigMap{}
	return cli.Get(ctx, client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}, &cm)
}

func checkShellTrigger(options *appsv1alpha1.ShellTrigger) error {
	if options.Exec == "" {
		return cfgutil.MakeError("required shell trigger")
	}
	return nil
}

func checkSignalTrigger(options *appsv1alpha1.UnixSignalTrigger) error {
	signal := options.Signal
	if !IsValidUnixSignal(signal) {
		return cfgutil.MakeError("this special signal [%s] is not supported now.", signal)
	}
	return nil
}

func CreateCfgRegexFilter(regexString string) (NotifyEventFilter, error) {
	regxPattern, err := regexp.Compile(regexString)
	if err != nil {
		return nil, cfgutil.WrapError(err, "failed to create regexp [%s]", regexString)
	}

	return func(event fsnotify.Event) (bool, error) {
		return regxPattern.MatchString(event.Name), nil
	}, nil
}

// CreateValidConfigMapFilter processes configmap volume
// https://github.com/ossrs/srs/issues/1635
func CreateValidConfigMapFilter() NotifyEventFilter {
	return func(event fsnotify.Event) (bool, error) {
		if !event.Has(fsnotify.Create) {
			return false, nil
		}
		if filepath.Base(event.Name) != "..data" {
			return false, nil
		}
		return true, nil
	}
}

func GetSupportReloadConfigSpecs(configSpecs []appsv1alpha1.ComponentConfigSpec, cli client.Client, ctx context.Context) ([]ConfigSpecMeta, error) {
	var reloadConfigSpecMeta []ConfigSpecMeta
	for _, configSpec := range configSpecs {
		if !cfgutil.NeedReloadVolume(configSpec) {
			continue
		}
		ccKey := client.ObjectKey{
			Namespace: "",
			Name:      configSpec.ConfigConstraintRef,
		}
		cc := &appsv1alpha1.ConfigConstraint{}
		if err := cli.Get(ctx, ccKey, cc); err != nil {
			return nil, cfgutil.WrapError(err, "failed to get ConfigConstraint, key[%v]", ccKey)
		}
		reloadOptions := cc.Spec.ReloadOptions
		if !IsSupportReload(reloadOptions) || cc.Spec.ReloadOptions == nil {
			continue
		}
		reloadConfigSpecMeta = append(reloadConfigSpecMeta, ConfigSpecMeta{
			ReloadOptions:   cc.Spec.ReloadOptions,
			ConfigSpec:      configSpec,
			ReloadType:      FromReloadTypeConfig(reloadOptions),
			FormatterConfig: *cc.Spec.FormatterConfig,
		})
	}
	return reloadConfigSpecMeta, nil
}
