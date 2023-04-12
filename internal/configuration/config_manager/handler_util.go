/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	// add volume to pod
	ScriptVolume *corev1.Volume
	Cluster      *appsv1alpha1.Cluster
}

func IsSupportReload(reload *appsv1alpha1.ReloadOptions) bool {
	return reload != nil && (reload.ShellTrigger != nil || reload.UnixSignalTrigger != nil || reload.TPLScriptTrigger != nil)
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
		return cfgutil.MakeError("shell trigger require exec not empty!")
	}
	return nil
}

func checkSignalTrigger(options *appsv1alpha1.UnixSignalTrigger) error {
	signal := options.Signal
	if !IsValidUnixSignal(signal) {
		return cfgutil.MakeError("this special signal [%s] is not supported for now!", signal)
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

// CreateValidConfigMapFilter process configmap volume
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
