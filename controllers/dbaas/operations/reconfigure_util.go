/*
Copyright ApeCloud Inc.

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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

// updateCfgParams merge parameters of the config into the configmap, and verify final configuration file.
func updateCfgParams(config dbaasv1alpha1.Configuration,
	tpl dbaasv1alpha1.ConfigTemplate,
	cmKey client.ObjectKey,
	ctx context.Context,
	cli client.Client,
	opsCrName string) (bool, *cfgcore.ConfigDiffInformation, error) {
	var (
		cm     = &corev1.ConfigMap{}
		cfgTpl = &dbaasv1alpha1.ConfigConstraint{}

		err    error
		newCfg map[string]string
	)

	if err := cli.Get(ctx, cmKey, cm); err != nil {
		return false, nil, err
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tpl.Namespace,
		Name:      tpl.ConfigConstraintRef,
	}, cfgTpl); err != nil {
		return false, nil, err
	}

	params := make([]cfgcore.ParamPairs, len(config.Keys))
	for i, key := range config.Keys {
		params[i] = cfgcore.ParamPairs{
			Key:           key.Key,
			UpdatedParams: fromKeyValuePair(key.Parameters),
		}
	}

	fc := cfgTpl.Spec.FormatterConfig
	newCfg, err = cfgcore.MergeAndValidateConfiguration(cfgTpl.Spec, cm.Data, params)
	if err != nil {
		return false, nil, err
	}

	difference, err := generateVersionDifference(client.ObjectKeyFromObject(cm), cm.Data, newCfg, fc.Formatter)
	if err != nil {
		return false, nil, err
	}
	if !difference.IsModify {
		return false, difference, nil
	}
	return false, difference, persistCfgCM(cm, newCfg, cli, ctx, opsCrName)
}

func persistCfgCM(cmObj *corev1.ConfigMap, newCfg map[string]string, cli client.Client, ctx context.Context, opsCrName string) error {
	patch := client.MergeFrom(cmObj.DeepCopy())
	cmObj.Data = newCfg
	if cmObj.Annotations == nil {
		cmObj.Annotations = make(map[string]string)
	}
	cmObj.Annotations[cfgcore.LastAppliedOpsCRAnnotation] = opsCrName
	return cli.Patch(ctx, cmObj, patch)
}

func fromKeyValuePair(parameters []dbaasv1alpha1.ParameterPair) map[string]interface{} {
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

func generateVersionDifference(cfgKey client.ObjectKey,
	old, updated map[string]string,
	formatter dbaasv1alpha1.ConfigurationFormatter) (*cfgcore.ConfigDiffInformation, error) {
	option := cfgcore.CfgOption{
		Type:    cfgcore.CfgTplType,
		CfgType: formatter,
		Log:     log.Log,
	}

	return cfgcore.CreateMergePatch(
		&cfgcore.K8sConfig{
			CfgKey:         cfgKey,
			Configurations: old,
		}, &cfgcore.K8sConfig{
			CfgKey:         cfgKey,
			Configurations: updated,
		}, option)
}
