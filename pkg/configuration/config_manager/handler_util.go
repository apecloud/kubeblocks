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

package configmanager

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

// CfgManagerBuildParams is the params for building config manager sidecar
type CfgManagerBuildParams struct {
	ManagerName string          `json:"name"`
	Image       string          `json:"sidecarImage"`
	Args        []string        `json:"args"`
	Envs        []corev1.EnvVar `json:"envs"`

	ShareProcessNamespace bool `json:"shareProcessNamespace"`

	Volumes       []corev1.VolumeMount `json:"volumes"`
	ComponentName string               `json:"componentName"`
	CharacterType string               `json:"characterType"`
	SecreteName   string               `json:"secreteName"`

	// add volume to pod
	ScriptVolume           []corev1.Volume
	Cluster                *appsv1alpha1.Cluster
	ConfigSpecsBuildParams []ConfigSpecMeta

	// init tools container
	ToolsContainers           []corev1.Container
	DownwardAPIVolumes        []corev1.VolumeMount
	CMConfigVolumes           []corev1.Volume
	ConfigLazyRenderedVolumes map[string]corev1.VolumeMount

	// support custom config manager sidecar
	ConfigManagerReloadPath string `json:"configManagerReloadPath"`

	// support host network
	ContainerPort int32 `json:"containerPort"`
}

func IsSupportReload(reload *appsv1beta1.ReloadAction) bool {
	return reload != nil && isValidReloadPolicy(*reload)
}

func isValidReloadPolicy(reload appsv1beta1.ReloadAction) bool {
	return reload.AutoTrigger != nil ||
		reload.ShellTrigger != nil ||
		reload.TPLScriptTrigger != nil ||
		reload.UnixSignalTrigger != nil
}

func IsAutoReload(reload *appsv1beta1.ReloadAction) bool {
	return reload != nil && reload.AutoTrigger != nil
}

func FromReloadTypeConfig(reloadAction *appsv1beta1.ReloadAction) appsv1beta1.DynamicReloadType {
	switch {
	case reloadAction.UnixSignalTrigger != nil:
		return appsv1beta1.UnixSignalType
	case reloadAction.ShellTrigger != nil:
		return appsv1beta1.ShellType
	case reloadAction.TPLScriptTrigger != nil:
		return appsv1beta1.TPLScriptType
	case reloadAction.AutoTrigger != nil:
		return appsv1beta1.AutoType
	}
	return ""
}

func ValidateReloadOptions(reloadAction *appsv1beta1.ReloadAction, cli client.Client, ctx context.Context) error {
	switch {
	case reloadAction.UnixSignalTrigger != nil:
		return checkSignalTrigger(reloadAction.UnixSignalTrigger)
	case reloadAction.ShellTrigger != nil:
		return checkShellTrigger(reloadAction.ShellTrigger)
	case reloadAction.TPLScriptTrigger != nil:
		return checkTPLScriptTrigger(reloadAction.TPLScriptTrigger, cli, ctx)
	case reloadAction.AutoTrigger != nil:
		return nil
	}
	return core.MakeError("require special reload type!")
}

func checkTPLScriptTrigger(options *appsv1beta1.TPLScriptTrigger, cli client.Client, ctx context.Context) error {
	cm := corev1.ConfigMap{}
	return cli.Get(ctx, client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}, &cm)
}

func checkShellTrigger(options *appsv1beta1.ShellTrigger) error {
	if len(options.Command) == 0 {
		return core.MakeError("required shell trigger")
	}
	return nil
}

func checkSignalTrigger(options *appsv1beta1.UnixSignalTrigger) error {
	signal := options.Signal
	if !IsValidUnixSignal(signal) {
		return core.MakeError("this special signal [%s] is not supported now.", signal)
	}
	return nil
}

func CreateCfgRegexFilter(regexString string) (NotifyEventFilter, error) {
	regxPattern, err := regexp.Compile(regexString)
	if err != nil {
		return nil, core.WrapError(err, "failed to create regexp [%s]", regexString)
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
		if !core.NeedReloadVolume(configSpec) {
			continue
		}
		ccKey := client.ObjectKey{
			Namespace: "",
			Name:      configSpec.ConfigConstraintRef,
		}
		cc := &appsv1beta1.ConfigConstraint{}
		if err := cli.Get(ctx, ccKey, cc); err != nil {
			return nil, core.WrapError(err, "failed to get ConfigConstraint, key[%v]", ccKey)
		}
		reloadOptions := cc.Spec.ReloadAction
		if !IsSupportReload(reloadOptions) || IsAutoReload(reloadOptions) {
			continue
		}
		reloadConfigSpecMeta = append(reloadConfigSpecMeta, ConfigSpecMeta{
			ToolsImageSpec: cc.Spec.ToolsSetup,
			ScriptConfig:   cc.Spec.ScriptConfigs,
			ConfigSpecInfo: ConfigSpecInfo{
				ReloadAction:       cc.Spec.ReloadAction,
				ConfigSpec:         configSpec,
				ReloadType:         FromReloadTypeConfig(reloadOptions),
				DownwardAPIOptions: cc.Spec.DownwardAPITriggeredActions,
				FormatterConfig:    *cc.Spec.FileFormatConfig,
			},
		})
	}
	return reloadConfigSpecMeta, nil
}

func FilterSubPathVolumeMount(metas []ConfigSpecMeta, volumes []corev1.VolumeMount) []ConfigSpecMeta {
	var filtered []ConfigSpecMeta
	for _, meta := range metas {
		v := FindVolumeMount(volumes, meta.ConfigSpec.VolumeName)
		if v == nil || v.SubPath == "" || meta.ReloadType == appsv1beta1.TPLScriptType {
			filtered = append(filtered, meta)
		}
	}
	return filtered
}
