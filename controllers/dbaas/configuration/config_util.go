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

package configuration

import (
	"fmt"
	"time"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ConfigReconcileInterval = time.Second * 5

	ConfigurationTemplateFinalizerName = "configuration.kubeblocks.io/finalizer"

	// CMConfigurationTplLabelKey  configmap is config template
	CMConfigurationTplLabelKey     = "configuration.kubeblocks.io/configuration-template"
	CMConfigurationTplNameLabelKey = "app.kubernetes.io/configurationtpl-name"
	CMInsConfigurationHashLabelKey = "configuration.kubeblocks.io/configuration-hash"

	// CMInsConfigurationLabelKey configmap is configuration file for component
	CMInsConfigurationLabelKey = "app.kubernetes.io/ins-configure"

	CMInsLastReconfigureMethodLabelKey = "configuration.kubeblocks.io/last-applied-reconfigure-policy"

	ReconfigureFirstConfigType = "created"
	ReconfigureNoChangeType    = "noChange"
	ReconfigureAutoReloadType  = string(dbaasv1alpha1.AutoReload)
	ReconfigureSimpleType      = string(dbaasv1alpha1.NormalPolicy)
	ReconfigureParallelType    = string(dbaasv1alpha1.RestartPolicy)
	ReconfigureRollingType     = string(dbaasv1alpha1.RollingPolicy)
)

type ValidateConfigMap func(configTpl, ns string) (*corev1.ConfigMap, error)
type ValidateConfigSchema func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error)

func CheckConfigurationLabels(object client.Object, requiredLabs []string) bool {
	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for _, label := range requiredLabs {
		if _, ok := labels[label]; !ok {
			return false
		}
	}

	if _, ok := labels[CMInsConfigurationLabelKey]; !ok {
		return false
	}

	return CheckEnableCfgUpgrade(object)
}

func GetConfigMapByName(cli client.Client, ctx intctrlutil.RequestCtx, cmName, ns string) (*corev1.ConfigMap, error) {
	if len(cmName) == 0 {
		return nil, fmt.Errorf("required configmap reference name is empty! [%v]", cmName)
	}

	configKey := client.ObjectKey{
		Namespace: ns,
		Name:      cmName,
	}
	configObj := &corev1.ConfigMap{}
	if err := cli.Get(ctx.Ctx, configKey, configObj); err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", configKey)
		return nil, err
	}

	return configObj, nil
}

func CheckConfigurationTemplate(cli client.Client, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplate) (bool, error) {
	// check ConfigTemplate Validate
	configmapFn := func(configTpl, ns string) (*corev1.ConfigMap, error) {
		return GetConfigMapByName(cli, ctx, configTpl, ns)
	}

	// validate configuration template
	isConfigSchemaFn := func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error) {
		if tpl == nil || tpl.Cue == nil {
			return true, nil
		}

		err := cfgcore.CueValidate(*tpl.Cue)
		return err == nil, err
	}

	return checkConfigTpl(ctx, tpl, configmapFn, isConfigSchemaFn)
}

func UpdateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplate) error {
	// step1: add finalizer
	// step2: add labels: CMConfigurationTplLabelKey
	// step3: update immutable

	cmObj, err := GetConfigMapByName(cli, ctx, tpl.Spec.TplRef, tpl.Namespace)
	if err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", cmObj.Name)
	}

	patch := client.MergeFrom(cmObj.DeepCopy())
	if !controllerutil.ContainsFinalizer(cmObj, ConfigurationTemplateFinalizerName) {
		controllerutil.AddFinalizer(cmObj, ConfigurationTemplateFinalizerName)
	}

	if cmObj.ObjectMeta.Labels == nil {
		cmObj.ObjectMeta.Labels = map[string]string{}
	}
	cmObj.ObjectMeta.Labels[CMConfigurationTplLabelKey] = "true"

	// cmObj.Immutable = &tpl.Spec.Immutable
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

func checkConfigTpl(ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplate, confTplFn ValidateConfigMap, schemaFn ValidateConfigSchema) (bool, error) {
	// check cm isExist
	cmObj, err := confTplFn(tpl.Spec.TplRef, tpl.Namespace)
	if err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", cmObj.Name)
		return false, err
	}

	// validate schema
	if ok, err := schemaFn(tpl.Spec.ConfigurationSchema); !ok || err != nil {
		ctx.Log.Error(err, "failed to validate template schema!", "configMapName", fmt.Sprintf("%v", tpl.Spec.ConfigurationSchema))
		return ok, err
	}

	return true, nil
}

type ConfigTemplateHandler func([]dbaasv1alpha1.ConfigTemplate) (bool, error)
type ComponentValidateHandler func(component *dbaasv1alpha1.ClusterDefinitionComponent) error

func CheckCDConfigTemplate(client client.Client, ctx intctrlutil.RequestCtx, clusterDef *dbaasv1alpha1.ClusterDefinition) (bool, error) {
	return HandleConfigTemplate(clusterDef,
		func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return validateConfTpls(client, ctx, tpls)
		},
		func(component *dbaasv1alpha1.ClusterDefinitionComponent) error {
			cfgSpec := component.ConfigSpec
			_, err := cfgcm.NeedBuildConfigSidecar(cfgSpec.ConfigReload, cfgSpec.ConfigReloadType, cfgSpec.ConfigReloadTrigger)
			return err
		})
}

func HandleConfigTemplate(object client.Object, handler ConfigTemplateHandler, handler2 ...ComponentValidateHandler) (bool, error) {
	var (
		err  error
		tpls []dbaasv1alpha1.ConfigTemplate
	)
	switch cr := object.(type) {
	case *dbaasv1alpha1.ClusterDefinition:
		tpls, err = getCfgTplFromCD(cr, handler2...)
	case *dbaasv1alpha1.AppVersion:
		tpls = getCfgTplFromAV(cr)
	default:
		return false, cfgcore.MakeError("not support CR type: %v", cr)
	}

	switch {
	case err != nil:
		return false, err
	case len(tpls) > 0:
		return handler(tpls)
	default:
		return true, nil

	}
}

func getCfgTplFromAV(appVer *dbaasv1alpha1.AppVersion) []dbaasv1alpha1.ConfigTemplate {
	tpls := make([]dbaasv1alpha1.ConfigTemplate, 0)
	for _, component := range appVer.Spec.Components {
		if len(component.ConfigTemplateRefs) > 0 {
			tpls = append(tpls, component.ConfigTemplateRefs...)
		}
	}
	return tpls
}

func getCfgTplFromCD(clusterDef *dbaasv1alpha1.ClusterDefinition, validators ...ComponentValidateHandler) ([]dbaasv1alpha1.ConfigTemplate, error) {
	tpls := make([]dbaasv1alpha1.ConfigTemplate, 0)
	for _, component := range clusterDef.Spec.Components {
		if component.ConfigSpec != nil && len(component.ConfigSpec.ConfigTemplateRefs) > 0 {
			tpls = append(tpls, component.ConfigSpec.ConfigTemplateRefs...)
			// Check reload configure config template
			for _, validator := range validators {
				if err := validator(&component); err != nil {
					return nil, err
				}
			}
		}
	}
	return tpls, nil
}

func UpdateCDLabelsWithUsingConfiguration(cli client.Client, ctx intctrlutil.RequestCtx, cd *dbaasv1alpha1.ClusterDefinition) (bool, error) {
	return HandleConfigTemplate(cd, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		patch := client.MergeFrom(cd.DeepCopy())
		for _, tpl := range tpls {
			cd.Labels[cfgcore.GenerateUniqLabelKeyWithConfig(tpl.Name)] = tpl.Name
		}
		return true, cli.Patch(ctx.Ctx, cd, patch)
	})
}

func CheckAVConfigTemplate(client client.Client, ctx intctrlutil.RequestCtx, appVersion *dbaasv1alpha1.AppVersion) (bool, error) {
	return HandleConfigTemplate(appVersion, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		return validateConfTpls(client, ctx, tpls)
	})
}

func UpdateAVLabelsWithUsingConfiguration(cli client.Client, ctx intctrlutil.RequestCtx, appVer *dbaasv1alpha1.AppVersion) (bool, error) {
	return HandleConfigTemplate(appVer, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		patch := client.MergeFrom(appVer.DeepCopy())
		for _, tpl := range tpls {
			appVer.Labels[cfgcore.GenerateUniqLabelKeyWithConfig(tpl.Name)] = tpl.Name
		}
		return true, cli.Patch(ctx.Ctx, appVer, patch)
	})
}

func validateConfTpls(cli client.Client, ctx intctrlutil.RequestCtx, configTpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
	// check ConfigTemplate Validate
	foundConfTplFn := func(configTpl dbaasv1alpha1.ConfigTemplate) (*dbaasv1alpha1.ConfigurationTemplate, error) {
		if len(configTpl.Name) == 0 || len(configTpl.VolumeName) == 0 {
			return nil, fmt.Errorf("required configmap reference name or volume name is empty! [%v]", configTpl)
		}

		configKey := client.ObjectKey{
			Namespace: configTpl.Namespace,
			Name:      configTpl.Name,
		}
		configObj := &dbaasv1alpha1.ConfigurationTemplate{}
		if err := cli.Get(ctx.Ctx, configKey, configObj); err != nil {
			ctx.Log.Error(err, "failed to get config template cm object!", "configTplName", configKey)
			return nil, err
		}

		return configObj, nil
	}

	for _, tplRef := range configTpls {
		tpl, err := foundConfTplFn(tplRef)
		if err != nil {
			ctx.Log.Error(err, "failed to validate configuration template!", "configTpl", tplRef)
			return false, err
		}
		if !ValidateConfTplStatus(tpl.Status) {
			errMsg := fmt.Sprintf("Configuration template CR[%s] status not ready! current status: %s", tpl.Name, tpl.Status.Phase)
			ctx.Log.V(4).Info(errMsg)
			return false, fmt.Errorf(errMsg)
		}
	}

	return true, nil
}

func ValidateConfTplStatus(configStatus dbaasv1alpha1.ConfigurationTemplateStatus) bool {
	return configStatus.Phase == dbaasv1alpha1.AvailablePhase
}

func GetComponentByUsingCM(stsList *appv1.StatefulSetList, cfg client.ObjectKey) []appv1.StatefulSet {
	stsLen := len(stsList.Items)
	if stsLen == 0 {
		return nil
	}

	sts := make([]appv1.StatefulSet, 0, stsLen)
	for _, s := range stsList.Items {
		volumeMounted := intctrlutil.GetVolumeMountName(s.Spec.Template.Spec.Volumes, cfg.Name)
		if volumeMounted != nil {
			sts = append(sts, s)
		}
	}
	return sts
}

func GetClusterComponentsByName(components []dbaasv1alpha1.ClusterComponent, componentName string) *dbaasv1alpha1.ClusterComponent {
	for _, component := range components {
		if component.Name == componentName {
			return &component
		}
	}
	return nil
}

func GetConfigurationVersion(cfg *corev1.ConfigMap, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplateSpec) (*cfgcore.ConfigDiffInformation, error) {
	lastConfig, err := GetLastVersionConfig(cfg)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	option := cfgcore.CfgOption{
		Type:    cfgcore.CfgTplType,
		CfgType: tpl.Formatter,
		Log:     ctx.Log,
	}

	return cfgcore.CreateMergePatch(&cfgcore.K8sConfig{
		CfgKey:         client.ObjectKeyFromObject(cfg),
		Configurations: lastConfig,
	}, &cfgcore.K8sConfig{
		CfgKey:         client.ObjectKeyFromObject(cfg),
		Configurations: cfg.Data,
	}, option)
}

func UpdateConfigurationSchema(tpl *dbaasv1alpha1.ConfigurationTemplateSpec) error {
	schema := tpl.ConfigurationSchema
	if schema != nil && schema.Cue != nil && len(*schema.Cue) > 0 && schema.Schema == nil {
		customSchema, err := cfgcore.GenerateOpenAPISchema(*schema.Cue, tpl.CfgSchemaTopLevelName)
		if err != nil {
			return err
		}
		tpl.ConfigurationSchema.Schema = customSchema
	}
	return nil
}
