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

package configuration

import (
	"encoding/json"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func checkEnableCfgUpgrade(object client.Object) bool {
	// check user disable upgrade
	// config.kubeblocks.io/disable-reconfigure = "false"
	annotations := object.GetAnnotations()
	value, ok := annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]
	if !ok {
		return true
	}

	enable, err := strconv.ParseBool(value)
	if err == nil && enable {
		return false
	}

	return true
}

func setCfgUpgradeFlag(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, flag bool) error {
	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	config.ObjectMeta.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey] = strconv.FormatBool(flag)
	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return err
	}

	return nil
}

// checkAndApplyConfigsChanged check if configs changed
func checkAndApplyConfigsChanged(client client.Client, ctx intctrlutil.RequestCtx, cm *corev1.ConfigMap) (bool, error) {
	annotations := cm.GetAnnotations()

	configData, err := json.Marshal(cm.Data)
	if err != nil {
		return false, err
	}

	lastConfig, ok := annotations[constant.LastAppliedConfigAnnotation]
	if !ok {
		return updateAppliedConfigs(client, ctx, cm, configData, cfgcore.ReconfigureCreatedPhase)
	}

	return lastConfig == string(configData), nil
}

// updateAppliedConfigs update hash label and last applied config
func updateAppliedConfigs(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, configData []byte, reconfigurePhase string) (bool, error) {

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	config.ObjectMeta.Annotations[constant.LastAppliedConfigAnnotation] = string(configData)
	hash, err := cfgcore.ComputeHash(config.Data)
	if err != nil {
		return false, err
	}
	config.ObjectMeta.Labels[constant.CMInsConfigurationHashLabelKey] = hash

	newReconfigurePhase := config.ObjectMeta.Labels[constant.CMInsLastReconfigurePhaseKey]
	if newReconfigurePhase == "" {
		newReconfigurePhase = cfgcore.ReconfigureCreatedPhase
	}
	if cfgcore.ReconfigureNoChangeType != reconfigurePhase && !cfgcore.IsParametersUpdateFromManager(config) {
		newReconfigurePhase = reconfigurePhase
	}
	config.ObjectMeta.Labels[constant.CMInsLastReconfigurePhaseKey] = newReconfigurePhase

	// delete reconfigure-policy
	delete(config.ObjectMeta.Annotations, constant.UpgradePolicyAnnotationKey)
	delete(config.ObjectMeta.Annotations, constant.KBParameterUpdateSourceAnnotationKey)
	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return false, err
	}

	return true, nil
}

func getLastVersionConfig(cm *corev1.ConfigMap) (map[string]string, error) {
	data := make(map[string]string, 0)
	cfgContent, ok := cm.GetAnnotations()[constant.LastAppliedConfigAnnotation]
	if !ok {
		return data, nil
	}

	if err := json.Unmarshal([]byte(cfgContent), &data); err != nil {
		return nil, err
	}

	return data, nil
}

func getUpgradePolicy(cm *corev1.ConfigMap) appsv1alpha1.UpgradePolicy {
	const (
		DefaultUpgradePolicy = appsv1alpha1.NonePolicy
	)

	annotations := cm.GetAnnotations()
	value, ok := annotations[constant.UpgradePolicyAnnotationKey]
	if !ok {
		return DefaultUpgradePolicy
	}

	return appsv1alpha1.UpgradePolicy(value)
}
