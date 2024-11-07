/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type templateRenderValidator = func(map[string]string) error

// type renderWrapper struct {
// 	templateBuilder *configTemplateBuilder
//
// 	volumes             map[string]appsv1.ComponentTemplateSpec
// 	templateAnnotations map[string]string
// 	renderedObjs        []client.Object
//
// 	renderedSecretObjs []client.Object
//
// 	ctx       context.Context
// 	cli       client.Client
// 	cluster   *appsv1.Cluster
// 	component *appsv1.Component
// }
//
// func newTemplateRenderWrapper(ctx context.Context, cli client.Client, templateBuilder *configTemplateBuilder,
// 	cluster *appsv1.Cluster, component *appsv1.Component) renderWrapper {
// 	return renderWrapper{
// 		ctx:       ctx,
// 		cli:       cli,
// 		cluster:   cluster,
// 		component: component,
//
// 		templateBuilder:     templateBuilder,
// 		templateAnnotations: make(map[string]string),
// 		volumes:             make(map[string]appsv1.ComponentTemplateSpec),
// 	}
// }

// func (wrapper *renderWrapper) checkRerenderTemplateSpec(cfgCMName string, localObjs []client.Object) (*corev1.ConfigMap, error) {
// 	cmKey := client.ObjectKey{
// 		Name:      cfgCMName,
// 		Namespace: wrapper.cluster.Namespace,
// 	}
//
// 	cmObj := &corev1.ConfigMap{}
// 	localObject := findMatchedLocalObject(localObjs, cmKey, generics.ToGVK(cmObj))
// 	if localObject != nil {
// 		if cm, ok := localObject.(*corev1.ConfigMap); ok {
// 			return cm, nil
// 		}
// 	}
//
// 	cmErr := wrapper.cli.Get(wrapper.ctx, cmKey, cmObj, inDataContext())
// 	if cmErr != nil && !apierrors.IsNotFound(cmErr) {
// 		// An unexpected error occurs
// 		return nil, cmErr
// 	}
// 	if cmErr != nil {
// 		// Config is not exists
// 		return nil, nil
// 	}
//
// 	return cmObj, nil
// }
//
// func (wrapper *renderWrapper) renderConfigTemplate(cluster *appsv1.Cluster,
// 	component *component.SynthesizedComponent,
// 	localObjs []client.Object,
// 	componentParameter *parametersv1alpha1.ComponentParameter,
// 	paramsDefs []*parametersv1alpha1.ParametersDefinition) error {
// 	revision := fromConfiguration(componentParameter)
// 	for _, configSpec := range component.ConfigTemplates {
// 		cmName := core.GetComponentCfgName(cluster.Name, component.Name, configSpec.Name)
// 		origCMObj, err := wrapper.checkRerenderTemplateSpec(cmName, localObjs)
// 		if err != nil {
// 			return err
// 		}
// 		// If ConfigMap already exists, skip the rendering process.
// 		// In this way, the Component controller only creates ConfigMap objects for the first time,
// 		// and does not update the ConfigMap objects in the subsequent reconfiguration process.
// 		// The subsequent reconfiguration process is handled by the Configuration controller.
// 		if origCMObj != nil {
// 			wrapper.addVolumeMountMeta(configSpec, origCMObj, false, true)
// 			continue
// 		}
// 		item := intctrlutil.GetConfigTemplateItem(&componentParameter.Spec, configSpec.Name)
// 		if item == nil {
// 			return fmt.Errorf("not fount ComponentTemplateSpec: [%s]", configSpec.Name)
// 		}
// 		newCMObj, err := wrapper.rerenderConfigTemplate(cluster, component, configSpec, item, paramsDefs)
// 		if err != nil {
// 			return err
// 		}
// 		if err := applyUpdatedParameters(item, newCMObj, configSpec, wrapper.cli, wrapper.ctx); err != nil {
// 			return err
// 		}
// 		if err := wrapper.addRenderedObject(configSpec.ComponentTemplateSpec, newCMObj, configuration, !toSecret(configSpec)); err != nil {
// 			return err
// 		}
// 		if err := updateConfigMetaForCM(newCMObj, item, revision); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func fromConfiguration(componentParameter *parametersv1alpha1.ComponentParameter) string {
// 	if componentParameter == nil {
// 		return ""
// 	}
// 	return strconv.FormatInt(componentParameter.GetGeneration(), 10)
// }
//
// func updateConfigMetaForCM(newCMObj *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail, revision string) (err error) {
// 	annotations := newCMObj.GetAnnotations()
// 	if annotations == nil {
// 		annotations = make(map[string]string)
// 	}
// 	b, err := json.Marshal(item)
// 	if err != nil {
// 		return err
// 	}
// 	annotations[constant.ConfigAppliedVersionAnnotationKey] = string(b)
// 	hash, _ := cfgutil.ComputeHash(newCMObj.Data)
// 	annotations[constant.CMInsCurrentConfigurationHashLabelKey] = hash
// 	annotations[constant.ConfigurationRevision] = revision
// 	newCMObj.Annotations = annotations
// 	return
// }
//
// func applyUpdatedParameters(item *appsv1alpha1.ConfigurationItemDetail, cm *corev1.ConfigMap, configSpec appsv1.ComponentConfigSpec, cli client.Client, ctx context.Context) (err error) {
// 	var newData map[string]string
// 	var configConstraint *appsv1beta1.ConfigConstraint
//
// 	if item == nil || len(item.ConfigFileParams) == 0 {
// 		return
// 	}
// 	if configSpec.ConfigConstraintRef != "" {
// 		configConstraint, err = fetchConfigConstraint(configSpec.ConfigConstraintRef, ctx, cli)
// 	}
// 	if err != nil {
// 		return
// 	}
// 	newData, err = DoMerge(cm.Data, item.ConfigFileParams, configConstraint, configSpec)
// 	if err != nil {
// 		return
// 	}
// 	cm.Data = newData
// 	return
// }
//
// func (wrapper *renderWrapper) rerenderConfigTemplate(cluster *appsv1.Cluster,
// 	component *component.SynthesizedComponent,
// 	configSpec appsv1.ComponentTemplateSpec,
// 	item *parametersv1alpha1.ConfigTemplateItemDetail,
// 	paramsDefs []*parametersv1alpha1.ParametersDefinition,
// 	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
// ) (*corev1.ConfigMap, error) {
// 	cmName := core.GetComponentCfgName(cluster.Name, component.Name, configSpec.Name)
// 	newCMObj, err := generateConfigMapFromTemplate(cluster,
// 		component,
// 		wrapper.templateBuilder,
// 		cmName,
// 		configSpec,
// 		wrapper.ctx,
// 		wrapper.cli,
// 		func(m map[string]string) error {
// 			return validateRenderedData(m, paramsDefs, configRender)
// 		})
// 	if err != nil {
// 		return nil, err
// 	}
// 	// render user specified template
// 	if item != nil && item.CustomTemplates != nil {
// 		newData, err := mergerConfigTemplate(
// 			&appsv1.LegacyRenderedTemplateSpec{
// 				ConfigTemplateExtension: appsv1.ConfigTemplateExtension{
// 					TemplateRef: item.CustomTemplates.TemplateRef,
// 					Namespace:   item.CustomTemplates.Namespace,
// 					Policy:      item.CustomTemplates.Policy,
// 				}},
// 			wrapper.templateBuilder,
// 			configSpec,
// 			newCMObj.Data,
// 			paramsDefs,
// 			configRender,
// 			wrapper.ctx,
// 			wrapper.cli)
// 		if err != nil {
// 			return nil, err
// 		}
// 		newCMObj.Data = newData
// 	}
// 	// UpdateCMConfigSpecLabels(newCMObj, configSpec)
// 	// if InjectEnvEnabled(configSpec) && toSecret(configSpec) {
// 	// 	wrapper.renderedSecretObjs = append(wrapper.renderedSecretObjs, newCMObj)
// 	// }
// 	return newCMObj, nil
// }
//
// func (wrapper *renderWrapper) renderScriptTemplate(cluster *appsv1.Cluster, component *component.SynthesizedComponent,
// 	localObjs []client.Object) error {
// 	for _, templateSpec := range component.ScriptTemplates {
// 		cmName := core.GetComponentCfgName(cluster.Name, component.Name, templateSpec.Name)
// 		object := findMatchedLocalObject(localObjs, client.ObjectKey{
// 			Name:      cmName,
// 			Namespace: wrapper.cluster.Namespace}, generics.ToGVK(&corev1.ConfigMap{}))
// 		if object != nil {
// 			wrapper.addVolumeMountMeta(templateSpec, object, false, true)
// 			continue
// 		}
//
// 		// Generate ConfigMap objects for config files
// 		cm, err := generateConfigMapFromTemplate(cluster, component, wrapper.templateBuilder, cmName, "", templateSpec, wrapper.ctx, wrapper.cli, nil)
// 		if err != nil {
// 			return err
// 		}
// 		if err := wrapper.addRenderedObject(templateSpec, cm, nil, true); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
//
// func (wrapper *renderWrapper) addRenderedObject(templateSpec appsv1.ComponentTemplateSpec, cm *corev1.ConfigMap, configuration *appsv1alpha1.Configuration, asVolume bool) (err error) {
// 	// The owner of the configmap object is a cluster,
// 	// in order to manage the life cycle of configmap
// 	if configuration != nil {
// 		err = intctrlutil.SetControllerReference(configuration, cm)
// 	} else {
// 		err = intctrlutil.SetControllerReference(wrapper.component, cm)
// 	}
// 	if err != nil {
// 		return err
// 	}
//
// 	core.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
// 	wrapper.addVolumeMountMeta(templateSpec, cm, true, asVolume)
// 	return nil
// }
//
// func (wrapper *renderWrapper) addVolumeMountMeta(templateSpec appsv1.ComponentTemplateSpec, object client.Object, rendered bool, asVolume bool) {
// 	if asVolume {
// 		wrapper.volumes[object.GetName()] = templateSpec
// 	}
// 	if rendered {
// 		wrapper.renderedObjs = append(wrapper.renderedObjs, object)
// 	}
// 	wrapper.templateAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(templateSpec.Name)] = object.GetName()
// }
//
// func (wrapper *renderWrapper) CheckAndPatchConfigResource(origCMObj *corev1.ConfigMap, newData map[string]string) error {
// 	if origCMObj == nil {
// 		return nil
// 	}
// 	if reflect.DeepEqual(origCMObj.Data, newData) {
// 		return nil
// 	}
//
// 	patch := client.MergeFrom(origCMObj.DeepCopy())
// 	origCMObj.Data = newData
// 	if origCMObj.Annotations == nil {
// 		origCMObj.Annotations = make(map[string]string)
// 	}
// 	core.SetParametersUpdateSource(origCMObj, constant.ReconfigureManagerSource)
// 	rawData, err := json.Marshal(origCMObj.Data)
// 	if err != nil {
// 		return err
// 	}
//
// 	origCMObj.Annotations[corev1.LastAppliedConfigAnnotation] = string(rawData)
// 	return wrapper.cli.Patch(wrapper.ctx, origCMObj, patch)
// }

func findMatchedLocalObject(localObjs []client.Object, objKey client.ObjectKey, gvk schema.GroupVersionKind) client.Object {
	for _, obj := range localObjs {
		if obj.GetName() == objKey.Name && obj.GetNamespace() == objKey.Namespace {
			if generics.ToGVK(obj) == gvk {
				return obj
			}
		}
	}
	return nil
}

// func UpdateCMConfigSpecLabels(cm *corev1.ConfigMap, configSpec appsv1.ComponentTemplateSpec) {
// 	if cm.Labels == nil {
// 		cm.Labels = make(map[string]string)
// 	}
//
// 	cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpec.Name
// 	cm.Labels[constant.CMConfigurationTemplateNameLabelKey] = configSpec.TemplateRef
// }

// generateConfigMapFromTemplate renders config file by config template provided by provider.
func generateConfigMapFromTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	tplBuilder *configTemplateBuilder,
	cmName string,
	templateSpec appsv1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client, dataValidator templateRenderValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := renderConfigMapTemplate(tplBuilder, templateSpec, ctx, cli)
	if err != nil {
		return nil, err
	}

	if dataValidator != nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return factory.BuildConfigMapWithTemplate(cluster, component, configs, cmName, templateSpec), nil
}

// renderConfigMapTemplate renders config file using template engine
func renderConfigMapTemplate(
	templateBuilder *configTemplateBuilder,
	templateSpec appsv1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: templateSpec.Namespace,
		Name:      templateSpec.TemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	templateBuilder.setTemplateName(templateSpec.TemplateRef)
	renderedData, err := templateBuilder.render(cmObj.Data)
	if err != nil {
		return nil, core.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}

func fetchConfigConstraint(ccName string, ctx context.Context, cli client.Client) (*appsv1beta1.ConfigConstraint, error) {
	ccKey := client.ObjectKey{
		Name: ccName,
	}
	configConstraint := &appsv1beta1.ConfigConstraint{}
	if err := cli.Get(ctx, ccKey, configConstraint); err != nil {
		return nil, core.WrapError(err, "failed to get ConfigConstraint, key[%s]", ccName)
	}
	return configConstraint, nil
}

// validateRenderedData validates config file against constraint
func validateRenderedData(renderedData map[string]string, paramsDefs []*parametersv1alpha1.ParametersDefinition, configRender *parametersv1alpha1.ParameterDrivenConfigRender) error {
	if len(paramsDefs) == 0 || configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil
	}
	for _, paramsDef := range paramsDefs {
		fileName := paramsDef.Spec.FileName
		if paramsDef.Spec.ParametersSchema == nil {
			continue
		}
		if _, ok := renderedData[fileName]; !ok {
			continue
		}
		if fileConfig := resolveFileFormatConfig(configRender.Spec.Configs, fileName); fileConfig != nil {
			if err := validateConfigContent(renderedData[fileName], &paramsDef.Spec, fileConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveFileFormatConfig(configDescs []parametersv1alpha1.ComponentConfigDescription, fileName string) *parametersv1alpha1.FileFormatConfig {
	for i, configDesc := range configDescs {
		if fileName == configDesc.Name {
			return configDescs[i].FileFormatConfig
		}
	}
	return nil
}

func validateConfigContent(renderedData string, paramsDef *parametersv1alpha1.ParametersDefinitionSpec, fileFormat *parametersv1alpha1.FileFormatConfig) error {
	configChecker := validate.NewConfigValidator(paramsDef.ParametersSchema, fileFormat)
	// NOTE: It is necessary to verify the correctness of the data
	if err := configChecker.Validate(renderedData); err != nil {
		return core.WrapError(err, "failed to validate configmap")
	}
	return nil
}
