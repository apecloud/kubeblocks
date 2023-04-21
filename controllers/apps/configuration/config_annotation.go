/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"encoding/json"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
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
	hash, err := util.ComputeHash(config.Data)
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
