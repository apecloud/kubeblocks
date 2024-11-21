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
	"slices"

	"github.com/fsnotify/fsnotify"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
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
	SecreteName   string               `json:"secreteName"` // TODO(v1.0): conn-credential

	// add volume to pod
	ScriptVolume           []corev1.Volume
	Cluster                *appsv1.Cluster
	ConfigSpecsBuildParams []ConfigSpecMeta

	// init tools container
	ToolsContainers    []corev1.Container
	DownwardAPIVolumes []corev1.VolumeMount
	CMConfigVolumes    []corev1.Volume
	// ConfigLazyRenderedVolumes map[string]corev1.VolumeMount

	// support custom config manager sidecar
	ConfigManagerReloadPath string `json:"configManagerReloadPath"`

	// support host network
	ContainerPort int32 `json:"containerPort"`
}

func NeedRestart(paramsDefs map[string]*parametersv1alpha1.ParametersDefinition, patch *core.ConfigPatchInfo) bool {
	if patch == nil {
		return false
	}
	for key := range patch.UpdateConfig {
		if paramsDef, ok := paramsDefs[key]; !ok || !IsSupportReload(paramsDef.Spec.ReloadAction) {
			return true
		}
	}
	return false
}

func IsSupportReload(reload *parametersv1alpha1.ReloadAction) bool {
	return reload != nil && isValidReloadPolicy(*reload)
}

func isValidReloadPolicy(reload parametersv1alpha1.ReloadAction) bool {
	return reload.AutoTrigger != nil ||
		reload.ShellTrigger != nil ||
		reload.TPLScriptTrigger != nil ||
		reload.UnixSignalTrigger != nil
}

func IsAutoReload(reload *parametersv1alpha1.ReloadAction) bool {
	return reload != nil && reload.AutoTrigger != nil
}

func FromReloadTypeConfig(reloadAction *parametersv1alpha1.ReloadAction) parametersv1alpha1.DynamicReloadType {
	switch {
	case reloadAction.UnixSignalTrigger != nil:
		return parametersv1alpha1.UnixSignalType
	case reloadAction.ShellTrigger != nil:
		return parametersv1alpha1.ShellType
	case reloadAction.TPLScriptTrigger != nil:
		return parametersv1alpha1.TPLScriptType
	case reloadAction.AutoTrigger != nil:
		return parametersv1alpha1.AutoType
	}
	return ""
}

func ValidateReloadOptions(reloadAction *parametersv1alpha1.ReloadAction, cli client.Client, ctx context.Context) error {
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

func checkTPLScriptTrigger(options *parametersv1alpha1.TPLScriptTrigger, cli client.Client, ctx context.Context) error {
	cm := corev1.ConfigMap{}
	return cli.Get(ctx, client.ObjectKey{
		Namespace: options.Namespace,
		Name:      options.ScriptConfigMapRef,
	}, &cm)
}

func checkShellTrigger(options *parametersv1alpha1.ShellTrigger) error {
	if len(options.Command) == 0 {
		return core.MakeError("required shell trigger")
	}
	return nil
}

func checkSignalTrigger(options *parametersv1alpha1.UnixSignalTrigger) error {
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

func actionToolsImage(reloadAction *parametersv1alpha1.ReloadAction) *parametersv1alpha1.ToolsSetup {
	if reloadAction != nil && reloadAction.ShellTrigger != nil {
		return reloadAction.ShellTrigger.ToolsSetup
	}
	return nil
}

func actionToolsScripts(paramDef parametersv1alpha1.ParametersDefinitionSpec) []parametersv1alpha1.ScriptConfig {
	uniqueSlice := func(items []parametersv1alpha1.ScriptConfig) []parametersv1alpha1.ScriptConfig {
		var uniqItems []parametersv1alpha1.ScriptConfig
		for _, item := range items {
			if !slices.Contains(uniqItems, item) {
				uniqItems = append(uniqItems, item)
			}
		}
		return uniqItems
	}

	scriptConfigs := make([]parametersv1alpha1.ScriptConfig, 0)
	for _, action := range paramDef.DownwardAPIChangeTriggeredActions {
		if action.ScriptConfig != nil {
			scriptConfigs = append(scriptConfigs, *action.ScriptConfig)
		}
	}
	if paramDef.ReloadAction == nil {
		return uniqueSlice(scriptConfigs)
	}
	if paramDef.ReloadAction.ShellTrigger != nil && paramDef.ReloadAction.ShellTrigger.ScriptConfig != nil {
		scriptConfigs = append(scriptConfigs, *paramDef.ReloadAction.ShellTrigger.ScriptConfig)
	}
	return uniqueSlice(scriptConfigs)
}

func GetSupportReloadConfigSpecs(configSpecs []appsv1.ComponentTemplateSpec,
	configDescs []parametersv1alpha1.ComponentConfigDescription,
	paramsDefs []*parametersv1alpha1.ParametersDefinition) ([]ConfigSpecMeta, error) {
	resolveReloadAction := func(fileName string) *parametersv1alpha1.ParametersDefinition {
		for _, paramsDef := range paramsDefs {
			if paramsDef.Spec.FileName == fileName {
				return paramsDef
			}
		}
		return nil
	}
	resolveConfigTemplate := func(name string) *appsv1.ComponentTemplateSpec {
		for i, configSpec := range configSpecs {
			if configSpec.Name == name {
				return &configSpecs[i]
			}
		}
		return nil
	}

	var reloadConfigSpecMeta []ConfigSpecMeta
	for _, desc := range configDescs {
		paramsDef := resolveReloadAction(desc.Name)
		if paramsDef == nil || paramsDef.Spec.ReloadAction == nil || desc.FileFormatConfig == nil {
			continue
		}
		configSpec := resolveConfigTemplate(desc.TemplateName)
		if configSpec == nil {
			continue
		}
		action := paramsDef.Spec.ReloadAction
		if !IsSupportReload(action) || IsAutoReload(action) {
			continue
		}
		reloadConfigSpecMeta = append(reloadConfigSpecMeta, ConfigSpecMeta{
			ToolsImageSpec: actionToolsImage(action),
			ScriptConfig:   actionToolsScripts(paramsDef.Spec),
			ConfigSpecInfo: ConfigSpecInfo{
				ReloadAction:       action,
				FormatterConfig:    *desc.FileFormatConfig,
				ConfigSpec:         *configSpec,
				ReloadType:         FromReloadTypeConfig(action),
				ConfigFile:         desc.Name,
				DownwardAPIOptions: paramsDef.Spec.DownwardAPIChangeTriggeredActions,
			},
		})
	}
	return reloadConfigSpecMeta, nil
}

// FilterSupportReloadActionConfigSpecs filters the provided ConfigSpecMeta slices based on the reload action type and volume mount configuration.
// It handles two types of updates to ConfigMaps:
//
// 1. Async mode: KubeBlocks controller is responsible for updating the ConfigMap, while kubelet synchronizes the ConfigMap to volumes.
// The config-manager detects configuration changes using fsnotify and executes the reload action. This requires volume mounting the ConfigMap.
// However, in async mode, if the volume mount is a subpath, kubelet does not synchronize the ConfigMap content to the container (see kubernetes/kubernetes#50345).
// As a result, the config-manager cannot detect configuration changes and does not support dynamic parameter updates for such configurations.
// Therefore, async-type ConfigSpecs with subpath volume mounts need to be removed.
//
// 2. Sync mode: For sync mode (regardless of the reload action type - TPLScriptType trigger or ShellType trigger), the controller directly watches
// the ConfigMap changes and actively invokes the reload action.
//
// Both async and sync types need to pass the ConfigSpecs to the config-manager.
//
// The check logic is an OR condition: either it is the first type (sync mode) or the second type (async) with a non-subpath volume mount configuration.
func FilterSupportReloadActionConfigSpecs(metas []ConfigSpecMeta, volumes []corev1.VolumeMount) []ConfigSpecMeta {
	var filtered []ConfigSpecMeta
	for _, meta := range metas {
		v := FindVolumeMount(volumes, meta.ConfigSpec.VolumeName)
		if isSyncReloadAction(meta.ConfigSpecInfo) || !isSubPathMount(v) {
			filtered = append(filtered, meta)
		}
	}
	return filtered
}

func isSubPathMount(v *corev1.VolumeMount) bool {
	// Configmap uses subPath case: https://github.com/kubernetes/kubernetes/issues/50345
	// The files are being updated on the host VM, but can't be updated in the container.
	return v != nil && v.SubPath != ""
}

func isSyncReloadAction(meta ConfigSpecInfo) bool {
	// If synchronous reloadAction is supported, kubelet limitations can be ignored.
	return meta.ReloadType == parametersv1alpha1.TPLScriptType && !core.IsWatchModuleForTplTrigger(meta.TPLScriptTrigger) ||
		meta.ReloadType == parametersv1alpha1.ShellType && !core.IsWatchModuleForShellTrigger(meta.ShellTrigger)
}
