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

type ComponentsType interface {
	dbaasv1alpha1.ClusterVersionComponent | dbaasv1alpha1.ClusterDefinitionComponent | dbaasv1alpha1.ClusterComponent
}

type filterFn[T ComponentsType] func(o T) bool

func filter[T ComponentsType](components []T, f filterFn[T]) *T {
	for _, c := range components {
		if f(c) {
			return &c
		}
	}
	return nil
}

func getConfigTemplatesFromComponent(
	cComponents []dbaasv1alpha1.ClusterComponent,
	dComponents []dbaasv1alpha1.ClusterDefinitionComponent,
	aComponents []dbaasv1alpha1.ClusterVersionComponent,
	componentName string) ([]dbaasv1alpha1.ConfigTemplate, error) {
	findCompTypeByName := func(comName string) *dbaasv1alpha1.ClusterComponent {
		return filter(cComponents, func(o dbaasv1alpha1.ClusterComponent) bool {
			return o.Name == comName
		})
	}

	cCom := findCompTypeByName(componentName)
	if cCom == nil {
		return nil, cfgcore.MakeError("failed to find component[%s]", componentName)
	}
	aCom := filter(aComponents, func(o dbaasv1alpha1.ClusterVersionComponent) bool {
		return o.Type == cCom.Type
	})
	dCom := filter(dComponents, func(o dbaasv1alpha1.ClusterDefinitionComponent) bool {
		return o.TypeName == cCom.Type
	})

	var (
		avTpls []dbaasv1alpha1.ConfigTemplate
		cdTpls []dbaasv1alpha1.ConfigTemplate
	)

	if aCom != nil {
		avTpls = aCom.ConfigTemplateRefs
	}
	if dCom != nil && dCom.ConfigSpec != nil {
		cdTpls = dCom.ConfigSpec.ConfigTemplateRefs
	}

	return MergeConfigTemplates(avTpls, cdTpls), nil
}

// updateCfgParams merge parameters of the config into the configmap, and verify final configuration file.
func updateCfgParams(
	config dbaasv1alpha1.Configuration,
	tpl dbaasv1alpha1.ConfigTemplate,
	cmKey client.ObjectKey,
	ctx context.Context,
	cli client.Client,
	opsCrName string) (bool, error) {
	var (
		cm     = &corev1.ConfigMap{}
		cfgTpl = &dbaasv1alpha1.ConfigurationTemplate{}

		err      error
		operator cfgcore.ConfigOperator
	)

	if err := cli.Get(ctx, cmKey, cm); err != nil {
		return false, err
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tpl.Namespace,
		Name:      tpl.ConfigConstraintRef,
	}, cfgTpl); err != nil {
		return false, err
	}
	if operator, err = cfgcore.NewConfigLoader(cfgcore.CfgOption{
		Type:    cfgcore.CfgCmType,
		Log:     log.FromContext(ctx),
		CfgType: cfgTpl.Spec.FormatterConfig.Formatter,
		K8sKey: &cfgcore.K8sConfig{
			CfgKey: client.ObjectKeyFromObject(cm),
			ResourceFn: func(key client.ObjectKey) (map[string]string, error) {
				return cm.Data, nil
			},
		},
	}); err != nil {
		return false, err
	}

	// process special formatter options
	fc := cfgTpl.Spec.FormatterConfig
	options := func(ctx *cfgcore.CfgOpOption) {
		// process special formatter
		if fc.Formatter == dbaasv1alpha1.INI && fc.IniConfig != nil {
			ctx.IniContext = &cfgcore.IniContext{
				SectionName: fc.IniConfig.SectionName,
			}
		}
	}

	// merge param to config file
	for _, key := range config.Keys {
		if err := operator.MergeFrom(fromKeyValuePair(key.Parameters),
			cfgcore.NewCfgOptions(key.Key, options)); err != nil {
			return false, err
		}
	}

	newCfg, err := operator.ToCfgContent()
	if err != nil {
		return false, cfgcore.WrapError(err, "failed to generate config file")
	}

	configChecker := cfgcore.NewConfigValidator(&cfgTpl.Spec)
	if err := configChecker.Validate(newCfg); err != nil {
		return true, cfgcore.WrapError(err, "failed to validate updated config")
	}

	return false, persistCfgCM(cm, newCfg, cli, ctx, opsCrName)
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

// MergeConfigTemplates merge AppVersion.Components[*].ConfigTemplateRefs and ClusterDefinition.Components[*].ConfigTemplateRefs
func MergeConfigTemplates(appVersionTpl []dbaasv1alpha1.ConfigTemplate,
	cdTpl []dbaasv1alpha1.ConfigTemplate) []dbaasv1alpha1.ConfigTemplate {
	if len(appVersionTpl) == 0 {
		return cdTpl
	}

	if len(cdTpl) == 0 {
		return appVersionTpl
	}

	mergedCfgTpl := make([]dbaasv1alpha1.ConfigTemplate, 0, len(appVersionTpl)+len(cdTpl))
	mergedTplMap := make(map[string]struct{}, cap(mergedCfgTpl))

	for i := range appVersionTpl {
		volumeName := appVersionTpl[i].VolumeName
		if _, ok := (mergedTplMap)[volumeName]; ok {
			// It's been checked in validation webhook
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, appVersionTpl[i])
		mergedTplMap[volumeName] = struct{}{}
	}

	for i := range cdTpl {
		// AppVersion replace clusterDefinition
		volumeName := cdTpl[i].VolumeName
		if _, ok := (mergedTplMap)[volumeName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, cdTpl[i])
		mergedTplMap[volumeName] = struct{}{}
	}

	return mergedCfgTpl
}
