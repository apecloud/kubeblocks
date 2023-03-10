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
	"github.com/go-logr/logr"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

const (
	ConfigReconcileInterval = time.Second * 5

	ReconfigureFirstConfigType = "created"
	ReconfigureNoChangeType    = "noChange"
	ReconfigureAutoReloadType  = string(appsv1alpha1.AutoReload)
	ReconfigureSimpleType      = string(appsv1alpha1.NormalPolicy)
	ReconfigureParallelType    = string(appsv1alpha1.RestartPolicy)
	ReconfigureRollingType     = string(appsv1alpha1.RollingPolicy)
)

type ValidateConfigMap func(configTpl, ns string) (*corev1.ConfigMap, error)
type ValidateConfigSchema func(tpl *appsv1alpha1.CustomParametersValidation) (bool, error)

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
	if ins, ok := labels[constant.CMConfigurationTypeLabelKey]; !ok || ins != constant.ConfigInstanceType {
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

func checkConfigConstraint(ctx intctrlutil.RequestCtx, tpl *appsv1alpha1.ConfigConstraint) (bool, error) {
	// validate configuration template
	validateConfigSchema := func(tpl *appsv1alpha1.CustomParametersValidation) (bool, error) {
		if tpl == nil || len(tpl.CUE) == 0 {
			return true, nil
		}

		err := cfgcore.CueValidate(tpl.CUE)
		return err == nil, err
	}

	// validate schema
	if ok, err := validateConfigSchema(tpl.Spec.ConfigurationSchema); !ok || err != nil {
		ctx.Log.Error(err, "failed to validate template schema!", "configMapName", fmt.Sprintf("%v", tpl.Spec.ConfigurationSchema))
		return ok, err
	}
	return true, nil
}

func ReconcileConfigurationForReferencedCR[T generics.Object, PT generics.PObject[T]](client client.Client, ctx intctrlutil.RequestCtx, obj PT) error {
	if ok, err := checkConfigTemplate(client, ctx, obj); !ok || err != nil {
		return fmt.Errorf("failed to check config template")
	}
	if ok, err := updateLabelsByConfiguration(client, ctx, obj); !ok || err != nil {
		return fmt.Errorf("failed to update using config template info")
	}
	if _, err := updateConfigMapFinalizer(client, ctx, obj); err != nil {
		return fmt.Errorf("failed to update config map finalizer")
	}
	return nil
}

func DeleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, obj client.Object) error {
	handler := func(tpls []appsv1alpha1.ComponentConfigSpec) (bool, error) {
		return true, batchDeleteConfigMapFinalizer(cli, ctx, tpls, obj)
	}
	_, err := handleConfigTemplate(obj, handler)
	return err
}

func validateConfigMapOwners(cli client.Client, ctx intctrlutil.RequestCtx, labels client.MatchingLabels, check func(obj client.Object) bool, objLists ...client.ObjectList) (bool, error) {
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
		if !val.CanInterface() || !check(val.Interface().(client.Object)) {
			return false, nil
		}
	}
	return true, nil
}

func batchDeleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpls []appsv1alpha1.ComponentConfigSpec, cr client.Object) error {
	validator := func(obj client.Object) bool {
		return obj.GetName() == cr.GetName() && obj.GetNamespace() == cr.GetNamespace()
	}
	for _, tpl := range tpls {
		labels := client.MatchingLabels{
			cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name): tpl.ConfigTemplateRef,
		}
		if ok, err := validateConfigMapOwners(cli, ctx, labels, validator, &appsv1alpha1.ClusterVersionList{}, &appsv1alpha1.ClusterDefinitionList{}); err != nil {
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

func updateConfigMapFinalizer(client client.Client, ctx intctrlutil.RequestCtx, obj client.Object) (bool, error) {
	handler := func(tpls []appsv1alpha1.ComponentConfigSpec) (bool, error) {
		return true, batchUpdateConfigMapFinalizer(client, ctx, tpls)
	}
	return handleConfigTemplate(obj, handler)
}

func batchUpdateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpls []appsv1alpha1.ComponentConfigSpec) error {
	for _, tpl := range tpls {
		if err := updateConfigMapFinalizerImpl(cli, ctx, tpl); err != nil {
			return err
		}
	}
	return nil
}

func updateConfigMapFinalizerImpl(cli client.Client, ctx intctrlutil.RequestCtx, tpl appsv1alpha1.ComponentConfigSpec) error {
	// step1: add finalizer
	// step2: add labels: CMConfigurationTypeLabelKey
	// step3: update immutable

	cmObj, err := getConfigMapByName(cli, ctx, tpl.ConfigTemplateRef, tpl.Namespace)
	if err != nil {
		ctx.Log.Error(err, "failed to get template cm object!", "configMapName", cmObj.Name)
		return err
	}

	if controllerutil.ContainsFinalizer(cmObj, constant.ConfigurationTemplateFinalizerName) {
		return nil
	}

	patch := client.MergeFrom(cmObj.DeepCopy())

	if cmObj.Labels == nil {
		cmObj.Labels = map[string]string{}
	}
	cmObj.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigTemplateType
	controllerutil.AddFinalizer(cmObj, constant.ConfigurationTemplateFinalizerName)

	// cmObj.Immutable = &tpl.Spec.Immutable
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

func deleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, tpl appsv1alpha1.ComponentConfigSpec) error {
	cmObj, err := getConfigMapByName(cli, ctx, tpl.ConfigTemplateRef, tpl.Namespace)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", tpl.ConfigTemplateRef)
		return err
	}

	if !controllerutil.ContainsFinalizer(cmObj, constant.ConfigurationTemplateFinalizerName) {
		return nil
	}

	patch := client.MergeFrom(cmObj.DeepCopy())
	controllerutil.RemoveFinalizer(cmObj, constant.ConfigurationTemplateFinalizerName)
	return cli.Patch(ctx.Ctx, cmObj, patch)
}

type ConfigTemplateHandler func([]appsv1alpha1.ComponentConfigSpec) (bool, error)
type ComponentValidateHandler func(component *appsv1alpha1.ClusterComponentDefinition) error

func handleConfigTemplate(object client.Object, handler ConfigTemplateHandler, handler2 ...ComponentValidateHandler) (bool, error) {
	var (
		err             error
		configTemplates []appsv1alpha1.ComponentConfigSpec
	)
	switch cr := object.(type) {
	case *appsv1alpha1.ClusterDefinition:
		configTemplates, err = getConfigTemplateFromCD(cr, handler2...)
	case *appsv1alpha1.ClusterVersion:
		configTemplates = getConfigTemplateFromCV(cr)
	default:
		return false, cfgcore.MakeError("not support CR type: %v", cr)
	}

	switch {
	case err != nil:
		return false, err
	case len(configTemplates) > 0:
		return handler(configTemplates)
	default:
		return true, nil
	}
}

func getConfigTemplateFromCV(appVer *appsv1alpha1.ClusterVersion) []appsv1alpha1.ComponentConfigSpec {
	configTemplates := make([]appsv1alpha1.ComponentConfigSpec, 0)
	for _, component := range appVer.Spec.ComponentVersions {
		if len(component.ComponentConfigSpec) > 0 {
			configTemplates = append(configTemplates, component.ComponentConfigSpec...)
		}
	}
	return configTemplates
}

func getConfigTemplateFromCD(clusterDef *appsv1alpha1.ClusterDefinition, validators ...ComponentValidateHandler) ([]appsv1alpha1.ComponentConfigSpec, error) {
	configTemplates := make([]appsv1alpha1.ComponentConfigSpec, 0)
	for _, component := range clusterDef.Spec.ComponentDefs {
		// For compatibility with the previous lifecycle management of configurationSpec.ConfigTemplateRef, it is necessary to convert ComponentScriptSpec to ComponentConfigSpec,
		// ensuring that the script-related configmap is not allowed to be deleted.
		for _, scriptSpec := range component.ComponentScriptSpec {
			configTemplates = append(configTemplates, appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: scriptSpec,
			})
		}
		if len(component.ComponentConfigSpec) == 0 {
			continue
		}
		configTemplates = append(configTemplates, component.ComponentConfigSpec...)
		// Check reload configure config template
		for _, validator := range validators {
			if err := validator(&component); err != nil {
				return nil, err
			}
		}
	}
	return configTemplates, nil
}

func checkConfigTemplate(client client.Client, ctx intctrlutil.RequestCtx, obj client.Object) (bool, error) {
	handler := func(tpls []appsv1alpha1.ComponentConfigSpec) (bool, error) {
		return validateConfigTemplate(client, ctx, tpls)
	}
	return handleConfigTemplate(obj, handler)
}

func updateLabelsByConfiguration[T generics.Object, PT generics.PObject[T]](cli client.Client, ctx intctrlutil.RequestCtx, obj PT) (bool, error) {
	handler := func(tpls []appsv1alpha1.ComponentConfigSpec) (bool, error) {
		patch := client.MergeFrom(PT(obj.DeepCopy()))
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for _, tpl := range tpls {
			labels[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = tpl.ConfigTemplateRef
			if len(tpl.ConfigConstraintRef) != 0 {
				labels[cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(tpl.ConfigConstraintRef)] = tpl.ConfigConstraintRef
			}
		}
		obj.SetLabels(labels)
		return true, cli.Patch(ctx.Ctx, obj, patch)
	}
	return handleConfigTemplate(obj, handler)
}

func validateConfigTemplate(cli client.Client, ctx intctrlutil.RequestCtx, configTemplates []appsv1alpha1.ComponentConfigSpec) (bool, error) {
	// check ConfigTemplate Validate
	foundTemplateFn := func(configTpl appsv1alpha1.ComponentConfigSpec, logger logr.Logger) (*appsv1alpha1.ConfigConstraint, error) {
		if _, err := getConfigMapByName(cli, ctx, configTpl.ConfigTemplateRef, configTpl.Namespace); err != nil {
			logger.Error(err, "failed to get config template cm object!")
			return nil, err
		}

		if len(configTpl.ConfigConstraintRef) == 0 {
			return nil, nil
		}

		configObj := &appsv1alpha1.ConfigConstraint{}
		if err := cli.Get(ctx.Ctx, client.ObjectKey{
			Namespace: "",
			Name:      configTpl.ConfigConstraintRef,
		}, configObj); err != nil {
			logger.Error(err, "failed to get template cm object!")
			return nil, err
		}
		return configObj, nil
	}

	for _, templateRef := range configTemplates {
		logger := ctx.Log.WithValues("templateName", templateRef.Name).WithValues("configMapName", templateRef.ConfigTemplateRef)
		tpl, err := foundTemplateFn(templateRef, logger)
		if err != nil {
			logger.Error(err, "failed to validate config template!")
			return false, err
		}
		if tpl == nil || tpl.Spec.ReloadOptions == nil {
			continue
		}
		if err := cfgcm.ValidateReloadOptions(tpl.Spec.ReloadOptions, cli, ctx.Ctx); err != nil {
			return false, err
		}
		if !validateConfigConstraintStatus(tpl.Status) {
			errMsg := fmt.Sprintf("Configuration template CR[%s] status not ready! current status: %s", tpl.Name, tpl.Status.Phase)
			logger.V(1).Info(errMsg)
			return false, fmt.Errorf(errMsg)
		}
	}
	return true, nil
}

func validateConfigConstraintStatus(configStatus appsv1alpha1.ConfigConstraintStatus) bool {
	return configStatus.Phase == appsv1alpha1.AvailablePhase
}

func getRelatedComponentsByConfigmap(stsList *appv1.StatefulSetList, cfg client.ObjectKey) ([]appv1.StatefulSet, []string) {
	managerContainerName := constant.ConfigSidecarName
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

func createConfigurePatch(cfg *corev1.ConfigMap, format appsv1alpha1.CfgFileFormat, cmKeys []string) (*cfgcore.ConfigPatchInfo, bool, error) {
	lastConfig, err := getLastVersionConfig(cfg)
	if err != nil {
		return nil, false, cfgcore.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	return cfgcore.CreateConfigurePatch(lastConfig, cfg.Data, format, cmKeys, true)
}

func updateConfigurationSchema(tpl *appsv1alpha1.ConfigConstraintSpec) error {
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

func NeedReloadVolume(config appsv1alpha1.ComponentConfigSpec) bool {
	// TODO distinguish between scripts and configuration
	return config.ConfigConstraintRef != ""
}

func GetReloadOptions(cli client.Client, ctx context.Context, tpls []appsv1alpha1.ComponentConfigSpec) (*appsv1alpha1.ReloadOptions, error) {
	for _, tpl := range tpls {
		if !NeedReloadVolume(tpl) {
			continue
		}
		cfgConst := &appsv1alpha1.ConfigConstraint{}
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
	cluster *appsv1alpha1.Cluster,
	compDefName string) (*appsv1alpha1.ClusterComponentDefinition, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return nil, err
	}
	for i := range clusterDef.Spec.ComponentDefs {
		component := &clusterDef.Spec.ComponentDefs[i]
		if component.Name == compDefName {
			return component, nil
		}
	}
	return nil, nil
}
