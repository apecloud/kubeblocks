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

package operations

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type reconfiguringResult struct {
	failed             bool
	configPatch        *cfgcore.ConfigPatchInfo
	lastAppliedConfigs map[string]string
	err                error
}

// updateCfgParams merge parameters of the config into the configmap, and verify final configuration file.
func updateCfgParams(config appsv1alpha1.Configuration,
	tpl appsv1alpha1.ComponentConfigSpec,
	cmKey client.ObjectKey,
	ctx context.Context,
	cli client.Client,
	opsCrName string) reconfiguringResult {
	var (
		cm     = &corev1.ConfigMap{}
		cfgTpl = &appsv1alpha1.ConfigConstraint{}

		err    error
		newCfg map[string]string
	)

	if err := cli.Get(ctx, cmKey, cm); err != nil {
		return makeReconfiguringResult(err)
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tpl.Namespace,
		Name:      tpl.ConfigConstraintRef,
	}, cfgTpl); err != nil {
		return makeReconfiguringResult(err)
	}

	params := make([]cfgcore.ParamPairs, len(config.Keys))
	for i, key := range config.Keys {
		params[i] = cfgcore.ParamPairs{
			Key:           key.Key,
			UpdatedParams: fromKeyValuePair(key.Parameters),
		}
	}

	fc := cfgTpl.Spec.FormatterConfig
	newCfg, err = cfgcore.MergeAndValidateConfiguration(cfgTpl.Spec, cm.Data, tpl.Keys, params)
	if err != nil {
		return makeReconfiguringResult(err, withFailed(true))
	}

	configPatch, _, err := cfgcore.CreateConfigurePatch(cm.Data, newCfg, fc.Format, tpl.Keys, false)
	if err != nil {
		return makeReconfiguringResult(err)
	}
	if !configPatch.IsModify {
		return makeReconfiguringResult(nil, withReturned(newCfg, configPatch))
	}
	return makeReconfiguringResult(persistCfgCM(cm, newCfg, cli, ctx, opsCrName), withReturned(newCfg, configPatch))
}

func persistCfgCM(cmObj *corev1.ConfigMap, newCfg map[string]string, cli client.Client, ctx context.Context, opsCrName string) error {
	patch := client.MergeFrom(cmObj.DeepCopy())
	cmObj.Data = newCfg
	if cmObj.Annotations == nil {
		cmObj.Annotations = make(map[string]string)
	}
	cmObj.Annotations[constant.LastAppliedOpsCRAnnotation] = opsCrName
	return cli.Patch(ctx, cmObj, patch)
}

func fromKeyValuePair(parameters []appsv1alpha1.ParameterPair) map[string]interface{} {
	m := make(map[string]interface{}, len(parameters))
	for _, param := range parameters {
		if param.Value != nil {
			m[param.Key] = *param.Value
		} else {
			m[param.Key] = param.Value
		}
	}
	return m
}

func withFailed(failed bool) func(result *reconfiguringResult) {
	return func(result *reconfiguringResult) {
		result.failed = failed
	}
}

func withReturned(configs map[string]string, patch *cfgcore.ConfigPatchInfo) func(result *reconfiguringResult) {
	return func(result *reconfiguringResult) {
		result.lastAppliedConfigs = configs
		result.configPatch = patch
	}
}

func makeReconfiguringResult(err error, ops ...func(*reconfiguringResult)) reconfiguringResult {
	result := reconfiguringResult{
		failed: false,
		err:    err,
	}
	for _, o := range ops {
		o(&result)
	}
	return result
}

func constructReconfiguringConditions(result reconfiguringResult, resource *OpsResource, tpl *appsv1alpha1.ComponentConfigSpec) []*metav1.Condition {
	if result.configPatch.IsModify {
		return []*metav1.Condition{appsv1alpha1.NewReconfigureRunningCondition(
			resource.OpsRequest,
			appsv1alpha1.ReasonReconfigureMerged,
			tpl.Name,
			formatConfigPatchToMessage(result.configPatch, nil)),
		}
	}
	return []*metav1.Condition{
		appsv1alpha1.NewReconfigureRunningCondition(
			resource.OpsRequest,
			appsv1alpha1.ReasonReconfigureInvalidUpdated,
			tpl.Name,
			formatConfigPatchToMessage(result.configPatch, nil)),
		appsv1alpha1.NewSucceedCondition(resource.OpsRequest),
	}
}

func i2sMap(config map[string]interface{}) map[string]string {
	if len(config) == 0 {
		return nil
	}
	m := make(map[string]string, len(config))
	for key, value := range config {
		data, _ := json.Marshal(value)
		m[key] = string(data)
	}
	return m
}

func b2sMap(config map[string][]byte) map[string]string {
	if len(config) == 0 {
		return nil
	}
	m := make(map[string]string, len(config))
	for key, value := range config {
		m[key] = string(value)
	}
	return m
}

func processMergedFailed(resource *OpsResource, isInvalid bool, err error) error {
	if !isInvalid {
		return cfgcore.WrapError(err, "failed to update param!")
	}

	// if failed to validate configure, and retry
	if err := PatchOpsStatus(resource, appsv1alpha1.FailedPhase,
		appsv1alpha1.NewFailedCondition(resource.OpsRequest, err)); err != nil {
		return err
	}
	return nil
}

func formatConfigPatchToMessage(configPatch *cfgcore.ConfigPatchInfo, execStatus *cfgcore.PolicyExecStatus) string {
	policyName := ""
	if execStatus != nil {
		policyName = fmt.Sprintf("updated policy: <%s>, ", execStatus.PolicyName)
	}
	return fmt.Sprintf("%supdated: %s, added: %s, deleted:%s",
		policyName,
		configPatch.UpdateConfig,
		configPatch.AddConfig,
		configPatch.DeleteConfig)
}
