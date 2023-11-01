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

package configuration

import (
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type options = func(*intctrlutil.Result)

func reconciled(status ReturnedStatus, policy string, phase appsv1alpha1.ConfigurationPhase, options ...options) intctrlutil.Result {
	result := intctrlutil.Result{
		Policy:        policy,
		Phase:         phase,
		ExecResult:    string(status.Status),
		SucceedCount:  status.SucceedCount,
		ExpectedCount: status.ExpectedCount,
		Retry:         true,
	}
	for _, option := range options {
		option(&result)
	}
	return result
}

func unReconciled(phase appsv1alpha1.ConfigurationPhase, revision string, message string) intctrlutil.Result {
	return intctrlutil.Result{
		Phase:         phase,
		Revision:      revision,
		Message:       message,
		SucceedCount:  core.NotStarted,
		ExpectedCount: core.Unconfirmed,
		Failed:        false,
		Retry:         false,
	}
}

func isReconciledResult(result intctrlutil.Result) bool {
	return result.ExecResult != "" && result.Policy != ""
}

func withFailed(err error, retry bool) options {
	return func(result *intctrlutil.Result) {
		result.Retry = retry
		if err != nil {
			result.Failed = true
			result.Message = err.Error()
		}
	}
}

func checkEnableCfgUpgrade(object client.Object) bool {
	// check user's upgrade switch
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

func updateConfigPhase(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, phase appsv1alpha1.ConfigurationPhase, message string) (ctrl.Result, error) {
	return updateConfigPhaseWithResult(cli, ctx, config, unReconciled(phase, "", message))
}

func updateConfigPhaseWithResult(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, result intctrlutil.Result) (ctrl.Result, error) {
	revision, ok := config.ObjectMeta.Annotations[constant.ConfigurationRevision]
	if !ok || revision == "" {
		return intctrlutil.Reconciled()
	}

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	if result.Failed && !result.Retry {
		ctx.Log.Info(fmt.Sprintf("failed to reconcile and disable retry for configmap[%+v]", client.ObjectKeyFromObject(config)))
		config.ObjectMeta.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey] = strconv.FormatBool(true)
	}

	GcConfigRevision(config)
	if _, ok := config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)]; !ok || isReconciledResult(result) {
		result.Revision = revision
		b, _ := json.Marshal(result)
		config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)] = string(b)
	}

	if err := cli.Patch(ctx.Ctx, config, patch); err != nil {
		return intctrlutil.RequeueWithError(err, ctx.Log, "")
	}
	if result.Retry {
		return intctrlutil.RequeueAfter(ConfigReconcileInterval, ctx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// checkAndApplyConfigsChanged check if configs changed
func checkAndApplyConfigsChanged(client client.Client, ctx intctrlutil.RequestCtx, cm *corev1.ConfigMap) (bool, error) {
	annotations := cm.GetAnnotations()

	configData, err := json.Marshal(cm.Data)
	if err != nil {
		return false, err
	}

	lastConfig, ok := annotations[constant.LastAppliedConfigAnnotationKey]
	if !ok {
		return updateAppliedConfigs(client, ctx, cm, configData, core.ReconfigureCreatedPhase, nil)
	}

	return lastConfig == string(configData), nil
}

// updateAppliedConfigs updates hash label and last applied config
func updateAppliedConfigs(cli client.Client, ctx intctrlutil.RequestCtx, config *corev1.ConfigMap, configData []byte, reconfigurePhase string, result *intctrlutil.Result) (bool, error) {

	patch := client.MergeFrom(config.DeepCopy())
	if config.ObjectMeta.Annotations == nil {
		config.ObjectMeta.Annotations = map[string]string{}
	}

	GcConfigRevision(config)
	if revision, ok := config.ObjectMeta.Annotations[constant.ConfigurationRevision]; ok && revision != "" {
		if result == nil {
			result = util.ToPointer(unReconciled(appsv1alpha1.CFinishedPhase, "", fmt.Sprintf("phase: %s", reconfigurePhase)))
		}
		result.Revision = revision
		b, _ := json.Marshal(result)
		config.ObjectMeta.Annotations[core.GenerateRevisionPhaseKey(revision)] = string(b)
	}
	config.ObjectMeta.Annotations[constant.LastAppliedConfigAnnotationKey] = string(configData)
	hash, err := util.ComputeHash(config.Data)
	if err != nil {
		return false, err
	}
	config.ObjectMeta.Labels[constant.CMInsConfigurationHashLabelKey] = hash

	newReconfigurePhase := config.ObjectMeta.Labels[constant.CMInsLastReconfigurePhaseKey]
	if newReconfigurePhase == "" {
		newReconfigurePhase = core.ReconfigureCreatedPhase
	}
	if core.ReconfigureNoChangeType != reconfigurePhase && !core.IsParametersUpdateFromManager(config) {
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
	cfgContent, ok := cm.GetAnnotations()[constant.LastAppliedConfigAnnotationKey]
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
