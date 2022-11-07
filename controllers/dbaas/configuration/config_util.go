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
	"github.com/spf13/viper"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ConfigNamespaceKey = "CM_NAMESPACE"

	ConfigReconcileInterval = time.Second * 5

	ConfigurationTemplateFinalizerName = "configuration.kubeblocks.io/finalizer"
	ConfigurationTplLabelKey           = "configuration.kubeblocks.io/name"
	CMConfigurationTplLabelKey         = "configuration.kubeblocks.io/configuration-template"
	CMInsConfigurationHashLabelKey     = "configuration.kubeblocks.io/configuration-hash"

	CMInsConfigurationLabelKey = "app.kubernetes.io/ins-configure"
)

type ValidateConfigMap func(configTpl string) (*corev1.ConfigMap, error)
type ValidateConfigSchema func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error)

func init() {
	viper.SetDefault(ConfigNamespaceKey, "default")
}

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

	return EnableCfgUpgrade(object)
}

func GetConfigMapByName(cli client.Client, ctx intctrlutil.RequestCtx, cmName string) (*corev1.ConfigMap, error) {
	if len(cmName) == 0 {
		return nil, fmt.Errorf("required configmap reference name is empty! [%v]", cmName)
	}

	configKey := client.ObjectKey{
		Namespace: viper.GetString(ConfigNamespaceKey),
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
	configmapFn := func(configTpl string) (*corev1.ConfigMap, error) {
		return GetConfigMapByName(cli, ctx, configTpl)
	}

	// TODO(zt) validate configuration template
	isConfigSchemaFn := func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error) {
		// TODO(zt)

		return false, nil
	}

	return checkConfigTpl(ctx, tpl, configmapFn, isConfigSchemaFn)
}

func UpdateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplate) error {
	// TODO(zt)
	// step1: add finalizer
	// step2: add labels: CMConfigurationTplLabelKey
	// step3: update immutable

	cmObj, err := GetConfigMapByName(cli, ctx, tpl.Spec.TplRef)
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

	cmObj.Immutable = &tpl.Spec.Immutable
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

func checkConfigTpl(ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplate, confTplFn ValidateConfigMap, schemaFn ValidateConfigSchema) (bool, error) {
	// check cm isExist
	cmObj, err := confTplFn(tpl.Spec.TplRef)
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

func CheckClusterDefinitionTemplate(client client.Client, ctx intctrlutil.RequestCtx, clusterDef *dbaasv1alpha1.ClusterDefinition) (bool, error) {
	for _, component := range clusterDef.Spec.Components {
		if len(component.ConfigTemplateRefs) == 0 {
			continue
		}

		if ok, err := validateConfTpls(client, ctx, component.ConfigTemplateRefs); !ok || err != nil {
			return ok, err
		}
	}
	return true, nil
}

func CheckAppVersionTemplate(client client.Client, ctx intctrlutil.RequestCtx, appVersion *dbaasv1alpha1.AppVersion) (bool, error) {
	for _, component := range appVersion.Spec.Components {
		if len(component.ConfigTemplateRefs) == 0 {
			continue
		}

		if ok, err := validateConfTpls(client, ctx, component.ConfigTemplateRefs); !ok || err != nil {
			return ok, err
		}
	}
	return true, nil
}

func validateConfTpls(cli client.Client, ctx intctrlutil.RequestCtx, configTpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {

	// check ConfigTemplate Validate
	foundConfTplFn := func(configTpl dbaasv1alpha1.ConfigTemplate) (*dbaasv1alpha1.ConfigurationTemplate, error) {
		if len(configTpl.Name) == 0 || len(configTpl.VolumeName) == 0 {
			return nil, fmt.Errorf("required configmap reference name or volume name is empty! [%v]", configTpl)
		}

		configKey := client.ObjectKey{
			Namespace: viper.GetString(ConfigNamespaceKey),
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

func GetConfigurationVersion(config *corev1.ConfigMap, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigurationTemplateSpec) (*cfgcore.ConfigDiffInformation, error) {
	lastConfig, err := GetLastVersionConfig(config)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(config))
	}

	option := cfgcore.CfgOption{
		Type:    cfgcore.CFG_TPL,
		CfgType: tpl.Formatter,
		Log:     ctx.Log,
	}

	return cfgcore.CreateMergePatch(&cfgcore.K8sConfig{
		CfgKey:         client.ObjectKeyFromObject(config),
		Configurations: lastConfig,
	}, &cfgcore.K8sConfig{
		CfgKey:         client.ObjectKeyFromObject(config),
		Configurations: config.Data,
	}, option)
}
