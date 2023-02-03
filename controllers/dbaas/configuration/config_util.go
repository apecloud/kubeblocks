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
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/StudioSol/set"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ConfigReconcileInterval = time.Second * 5

	ReconfigureFirstConfigType = "created"
	ReconfigureNoChangeType    = "noChange"
	ReconfigureAutoReloadType  = string(dbaasv1alpha1.AutoReload)
	ReconfigureSimpleType      = string(dbaasv1alpha1.NormalPolicy)
	ReconfigureParallelType    = string(dbaasv1alpha1.RestartPolicy)
	ReconfigureRollingType     = string(dbaasv1alpha1.RollingPolicy)
)

type ValidateConfigMap func(configTpl, ns string) (*corev1.ConfigMap, error)
type ValidateConfigSchema func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error)

func checkConfigurationLabels(object client.Object, requiredLabs []string) bool {
	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for _, label := range requiredLabs {
		if _, ok := labels[label]; !ok {
			return false
		}
	}

	// reconfigure ConfigMap for db instance
	if ins, ok := labels[cfgcore.CMConfigurationTypeLabelKey]; !ok || ins != cfgcore.ConfigInstanceType {
		return false
	}

	return checkEnableCfgUpgrade(object)
}

func getConfigMapByName(cli client.Client, ctx intctrlutil.RequestCtx, cmName, ns string) (*corev1.ConfigMap, error) {
	if len(cmName) == 0 {
		return nil, fmt.Errorf("required configmap reference name is empty! [%v]", cmName)
	}

	configObj := &corev1.ConfigMap{}
	if err := cli.Get(ctx.Ctx, client.ObjectKey{
		Namespace: ns,
		Name:      cmName,
	}, configObj); err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", cmName)
		return nil, err
	}

	return configObj, nil
}

func checkConfigurationTemplate(ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigConstraint) (bool, error) {
	// validate configuration template
	isConfigSchemaFn := func(tpl *dbaasv1alpha1.CustomParametersValidation) (bool, error) {
		if tpl == nil || len(tpl.CUE) == 0 {
			return true, nil
		}

		err := cfgcore.CueValidate(tpl.CUE)
		return err == nil, err
	}

	return checkConfigTPL(ctx, tpl, isConfigSchemaFn)
}

func DeleteCDConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	_, err := handleConfigTemplate(clusterDef, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		return true, batchDeleteConfigMapFinalizer[*dbaasv1alpha1.ClusterDefinition](cli, ctx, tpls, clusterDef)
	})
	return err
}

func DeleteCVConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, clusterVersion *dbaasv1alpha1.ClusterVersion) error {
	_, err := handleConfigTemplate(clusterVersion, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		return true, batchDeleteConfigMapFinalizer[*dbaasv1alpha1.ClusterVersion](cli, ctx, tpls, clusterVersion)
	})
	return err
}

func validateConfigMapOwners(cli client.Client, ctx intctrlutil.RequestCtx, labels client.MatchingLabels, check func(obj any) bool, objLists ...client.ObjectList) (bool, error) {
	for _, objList := range objLists {
		if err := cli.List(ctx.Ctx, objList, labels, client.Limit(2)); err != nil {
			return false, err
		}
		v, err := conversion.EnforcePtr(objList)
		if err != nil {
			return false, err
		}
		items := v.FieldByName("Items")
		if !items.IsValid() || items.Kind() != reflect.Slice || items.Len() > 1 {
			return false, nil
		}
		if items.Len() == 0 {
			continue
		}

		val := items.Index(0)
		// fetch object pointer
		if val.CanAddr() {
			val = val.Addr()
		}
		if !val.CanInterface() || !check(val.Interface()) {
			return false, nil
		}
	}
	return true, nil
}

func batchDeleteConfigMapFinalizer[T client.Object](cli client.Client, ctx intctrlutil.RequestCtx, tpls []dbaasv1alpha1.ConfigTemplate, cr T) error {
	validator := func(obj any) bool {
		if expected, ok := obj.(T); !ok {
			return false
		} else {
			return expected.GetName() == cr.GetName() && expected.GetNamespace() == cr.GetNamespace()
		}
	}
	for _, tpl := range tpls {
		labelKey := cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)
		if ok, err := validateConfigMapOwners(cli, ctx, client.MatchingLabels{
			labelKey: tpl.ConfigTplRef,
		}, validator, &dbaasv1alpha1.ClusterVersionList{}, &dbaasv1alpha1.ClusterDefinitionList{}); err != nil {
			return err
		} else if !ok {
			continue
		}
		if err := deleteConfigMapFinalizer(cli, ctx, tpl); err != nil {
			return err
		}
	}
	return nil
}

func UpdateCDConfigMapFinalizer(client client.Client, ctx intctrlutil.RequestCtx, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	_, err := handleConfigTemplate(clusterDef,
		func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return true, batchUpdateConfigMapFinalizer(client, ctx, tpls)
		})
	return err
}

func UpdateCVConfigMapFinalizer(client client.Client, ctx intctrlutil.RequestCtx, clusterVersion *dbaasv1alpha1.ClusterVersion) error {
	_, err := handleConfigTemplate(clusterVersion,
		func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return true, batchUpdateConfigMapFinalizer(client, ctx, tpls)
		})
	return err
}

func batchUpdateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpls []dbaasv1alpha1.ConfigTemplate) error {
	for _, tpl := range tpls {
		if err := updateConfigMapFinalizer(cli, ctx, tpl); err != nil {
			return err
		}
	}
	return nil
}

func updateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpl dbaasv1alpha1.ConfigTemplate) error {
	// step1: add finalizer
	// step2: add labels: CMConfigurationTypeLabelKey
	// step3: update immutable

	cmObj, err := getConfigMapByName(cli, ctx, tpl.ConfigTplRef, tpl.Namespace)
	if err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", cmObj.Name)
		return err
	}

	if controllerutil.ContainsFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName) {
		return nil
	}

	patch := client.MergeFrom(cmObj.DeepCopy())

	if cmObj.ObjectMeta.Labels == nil {
		cmObj.ObjectMeta.Labels = map[string]string{}
	}
	cmObj.ObjectMeta.Labels[cfgcore.CMConfigurationTypeLabelKey] = cfgcore.ConfigTemplateType
	controllerutil.AddFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName)

	// cmObj.Immutable = &tpl.Spec.Immutable
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

func deleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpl dbaasv1alpha1.ConfigTemplate) error {
	cmObj, err := getConfigMapByName(cli, ctx, tpl.ConfigTplRef, tpl.Namespace)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", tpl.ConfigTplRef)
		return err
	}

	if !controllerutil.ContainsFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName) {
		return nil
	}

	patch := client.MergeFrom(cmObj.DeepCopy())
	controllerutil.RemoveFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName)
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

func checkConfigTPL(ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigConstraint, schemaFn ValidateConfigSchema) (bool, error) {
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
	return handleConfigTemplate(clusterDef,
		func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			return validateConfigTPLs(client, ctx, tpls)
		})
}

func handleConfigTemplate(object client.Object, handler ConfigTemplateHandler, handler2 ...ComponentValidateHandler) (bool, error) {
	var (
		err  error
		tpls []dbaasv1alpha1.ConfigTemplate
	)
	switch cr := object.(type) {
	case *dbaasv1alpha1.ClusterDefinition:
		tpls, err = getCfgTplFromCD(cr, handler2...)
	case *dbaasv1alpha1.ClusterVersion:
		tpls = getCfgTplFromCV(cr)
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

func getCfgTplFromCV(appVer *dbaasv1alpha1.ClusterVersion) []dbaasv1alpha1.ConfigTemplate {
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

func UpdateCDLabelsByConfiguration(cli client.Client, ctx intctrlutil.RequestCtx, cd *dbaasv1alpha1.ClusterDefinition) (bool, error) {
	return handleConfigTemplate(cd, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		patch := client.MergeFrom(cd.DeepCopy())
		for _, tpl := range tpls {
			cd.Labels[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = tpl.ConfigTplRef
			if len(tpl.ConfigConstraintRef) != 0 {
				cd.Labels[cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(tpl.ConfigConstraintRef)] = tpl.ConfigConstraintRef
			}
		}
		return true, cli.Patch(ctx.Ctx, cd, patch)
	})
}

func CheckCVConfigTemplate(client client.Client, ctx intctrlutil.RequestCtx, clusterVersion *dbaasv1alpha1.ClusterVersion) (bool, error) {
	return handleConfigTemplate(clusterVersion, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		return validateConfigTPLs(client, ctx, tpls)
	})
}

func UpdateCVLabelsByConfiguration(cli client.Client, ctx intctrlutil.RequestCtx, appVer *dbaasv1alpha1.ClusterVersion) (bool, error) {
	return handleConfigTemplate(appVer, func(tpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		patch := client.MergeFrom(appVer.DeepCopy())
		for _, tpl := range tpls {
			appVer.Labels[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = tpl.ConfigTplRef
			appVer.Labels[cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(tpl.ConfigConstraintRef)] = tpl.ConfigConstraintRef
		}
		return true, cli.Patch(ctx.Ctx, appVer, patch)
	})
}

func validateConfigTPLs(cli client.Client, ctx intctrlutil.RequestCtx, configTpls []dbaasv1alpha1.ConfigTemplate) (bool, error) {
	// check ConfigTemplate Validate
	foundConfTplFn := func(configTpl dbaasv1alpha1.ConfigTemplate) (*dbaasv1alpha1.ConfigConstraint, error) {
		if _, err := getConfigMapByName(cli, ctx, configTpl.ConfigTplRef, configTpl.Namespace); err != nil {
			ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", configTpl.ConfigTplRef)
			return nil, err
		}

		if len(configTpl.ConfigConstraintRef) == 0 {
			return nil, nil
		}

		configObj := &dbaasv1alpha1.ConfigConstraint{}
		if err := cli.Get(ctx.Ctx, client.ObjectKey{
			Namespace: "",
			Name:      configTpl.ConfigConstraintRef,
		}, configObj); err != nil {
			ctx.Log.Error(err, "failed to get config template cm object!", "configTplName", configTpl)
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
		if tpl == nil {
			continue
		}
		if err := cfgcm.NeedBuildConfigSidecar(tpl.Spec.ReloadOptions); err != nil {
			return false, err
		}
		if !validateConfTplStatus(tpl.Status) {
			errMsg := fmt.Sprintf("Configuration template CR[%s] status not ready! current status: %s", tpl.Name, tpl.Status.Phase)
			ctx.Log.V(4).Info(errMsg)
			return false, fmt.Errorf(errMsg)
		}
	}
	return true, nil
}

func validateConfTplStatus(configStatus dbaasv1alpha1.ConfigConstraintStatus) bool {
	return configStatus.Phase == dbaasv1alpha1.AvailablePhase
}

func getRelatedComponentsByConfigmap(stsList *appv1.StatefulSetList, cfg client.ObjectKey) ([]appv1.StatefulSet, []string) {
	managerContainerName := cfgcore.ConfigSidecarName
	stsLen := len(stsList.Items)
	if stsLen == 0 {
		return nil, nil
	}

	sts := make([]appv1.StatefulSet, 0, stsLen)
	containers := set.NewLinkedHashSetString()
	for _, s := range stsList.Items {
		volumeMounted := intctrlutil.GetVolumeMountName(s.Spec.Template.Spec.Volumes, cfg.Name)
		if volumeMounted == nil {
			continue
		}
		// filter config manager sidecar container
		contains := intctrlutil.GetContainersByConfigmap(s.Spec.Template.Spec.Containers,
			volumeMounted.Name, func(containerName string) bool {
				return managerContainerName == containerName
			})
		if len(contains) > 0 {
			sts = append(sts, s)
			containers.Add(contains...)
		}
	}
	return sts, containers.AsSlice()
}

func getClusterComponentsByName(components []dbaasv1alpha1.ClusterComponent, componentName string) *dbaasv1alpha1.ClusterComponent {
	for i := range components {
		component := &components[i]
		if component.Name == componentName {
			return component
		}
	}
	return nil
}

func createConfigurePatch(cfg *corev1.ConfigMap, ctx intctrlutil.RequestCtx, tpl *dbaasv1alpha1.ConfigConstraintSpec) (*cfgcore.ConfigPatchInfo, error) {
	lastConfig, err := getLastVersionConfig(cfg)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	option := cfgcore.CfgOption{
		Type:    cfgcore.CfgTplType,
		CfgType: tpl.FormatterConfig.Formatter,
		Log:     ctx.Log,
	}

	return cfgcore.CreateMergePatch(
		&cfgcore.K8sConfig{
			CfgKey:         client.ObjectKeyFromObject(cfg),
			Configurations: lastConfig,
		}, &cfgcore.K8sConfig{
			CfgKey:         client.ObjectKeyFromObject(cfg),
			Configurations: cfg.Data,
		}, option)
}

func updateConfigurationSchema(tpl *dbaasv1alpha1.ConfigConstraintSpec) error {
	schema := tpl.ConfigurationSchema
	if schema != nil && len(schema.CUE) > 0 && schema.Schema == nil {
		customSchema, err := cfgcore.GenerateOpenAPISchema(schema.CUE, tpl.CfgSchemaTopLevelName)
		if err != nil {
			return err
		}
		tpl.ConfigurationSchema.Schema = customSchema
	}
	return nil
}

func NeedReloadVolume(tpl dbaasv1alpha1.ConfigTemplate) bool {
	// TODO distinguish between scripts and configuration
	return len(tpl.ConfigConstraintRef) != 0
}

func GetReloadOptions(cli client.Client, ctx context.Context, tpls []dbaasv1alpha1.ConfigTemplate) (*dbaasv1alpha1.ReloadOptions, error) {
	for _, tpl := range tpls {
		if !NeedReloadVolume(tpl) {
			continue
		}
		cfgConst := &dbaasv1alpha1.ConfigConstraint{}
		if err := cli.Get(ctx, client.ObjectKey{
			Namespace: "",
			Name:      tpl.ConfigConstraintRef,
		}, cfgConst); err != nil {
			return nil, cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tpl)
		}
		if cfgConst.Spec.ReloadOptions != nil {
			return cfgConst.Spec.ReloadOptions, nil
		}
	}
	return nil, nil
}

func getComponentFromClusterDefinition(
	ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	typeName string) (*dbaasv1alpha1.ClusterDefinitionComponent, error) {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return nil, err
	}
	for i := range clusterDef.Spec.Components {
		component := &clusterDef.Spec.Components[i]
		if component.TypeName == typeName {
			return component, nil
		}
	}
	return nil, nil
}
