/*
Copyright 2022.

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

package configuration

import (
	"encoding/json"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	corev1 "k8s.io/api/core/v1"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LastAppliedConfigAnnotation          = "configuration.kubeblocks.io/last-applied-configuration"
	UpgradeInsConfigurationAnnotationKey = "configuration.kubeblocks.io/rolling-upgrade"
)

func EnableCfgUpgrade(object client.Object) bool {
	// check user disable upgrade
	// configuration.kubeblocks.io/rolling-upgrade = "false"
	annotations := object.GetAnnotations()
	value, ok := annotations[UpgradeInsConfigurationAnnotationKey]
	if !ok {
		return true
	}

	enable, err := strconv.ParseBool(value)
	if err == nil && !enable {
		return false
	}

	return true
}

// ApplyConfigurationChange is
func ApplyConfigurationChange(client client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap) (bool, error) {
	annotations := config.GetAnnotations()

	configData, err := json.Marshal(config.Data)
	if err != nil {
		return false, err
	}

	lastConfig, ok := annotations[LastAppliedConfigAnnotation]
	if !ok {
		return updateFirstApplyConfiguration(client, ctx, config, configData)
	}

	return lastConfig == string(configData), nil
}

func updateFirstApplyConfiguration(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, configData []byte) (bool, error) {

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	config.ObjectMeta.Annotations[LastAppliedConfigAnnotation] = string(configData)
	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return false, err
	}

	return true, nil
}

func GetLastVersionConfig(cfg *corev1.ConfigMap) (map[string]string, error) {
	cfgContent := cfg.GetAnnotations()[LastAppliedConfigAnnotation]

	data := make(map[string]string, 0)
	if err := json.Unmarshal([]byte(cfgContent), data); err != nil {
		return nil, err
	}

	return data, nil
}
