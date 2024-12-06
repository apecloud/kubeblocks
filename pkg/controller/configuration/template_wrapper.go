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
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/validate"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type renderWrapper struct {
	render.TemplateRender

	volumes             map[string]appsv1.ComponentTemplateSpec
	templateAnnotations map[string]string
	renderedObjs        []*corev1.ConfigMap

	ctx       context.Context
	cli       client.Client
	cluster   *appsv1.Cluster
	component *appsv1.Component
}

func newTemplateRenderWrapper(ctx context.Context, cli client.Client, templateBuilder render.TemplateRender,
	cluster *appsv1.Cluster, component *appsv1.Component) renderWrapper {

	return renderWrapper{
		ctx:       ctx,
		cli:       cli,
		cluster:   cluster,
		component: component,

		TemplateRender:      templateBuilder,
		templateAnnotations: make(map[string]string),
		volumes:             make(map[string]appsv1.ComponentTemplateSpec),
	}
}

func (wrapper *renderWrapper) checkRerenderTemplateSpec(cfgCMName string, localObjs []client.Object) (*corev1.ConfigMap, error) {
	cmKey := client.ObjectKey{
		Name:      cfgCMName,
		Namespace: wrapper.cluster.Namespace,
	}

	cmObj := &corev1.ConfigMap{}
	localObject := findMatchedLocalObject(localObjs, cmKey, generics.ToGVK(cmObj))
	if localObject != nil {
		if cm, ok := localObject.(*corev1.ConfigMap); ok {
			return cm, nil
		}
	}

	cmErr := wrapper.cli.Get(wrapper.ctx, cmKey, cmObj, inDataContext())
	if cmErr != nil && !apierrors.IsNotFound(cmErr) {
		// An unexpected error occurs
		return nil, cmErr
	}
	if cmErr != nil {
		// Config is not exists
		return nil, nil
	}

	return cmObj, nil
}

func (wrapper *renderWrapper) renderConfigTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	localObjs []client.Object,
	componentParameter *parametersv1alpha1.ComponentParameter,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
	defs []*parametersv1alpha1.ParametersDefinition, revision string) error {
	for _, configSpec := range component.ConfigTemplates {
		var item *parametersv1alpha1.ConfigTemplateItemDetail
		cmName := core.GetComponentCfgName(cluster.Name, component.Name, configSpec.Name)
		origCMObj, err := wrapper.checkRerenderTemplateSpec(cmName, localObjs)
		if err != nil {
			return err
		}
		// If ConfigMap already exists, skip the rendering process.
		// In this way, the Component controller only creates ConfigMap objects for the first time,
		// and does not update the ConfigMap objects in the subsequent reconfiguration process.
		// The subsequent reconfiguration process is handled by the Configuration controller.
		if origCMObj != nil {
			wrapper.addVolumeMountMeta(configSpec, origCMObj, false)
			continue
		}
		item = intctrlutil.GetConfigTemplateItem(&componentParameter.Spec, configSpec.Name)
		if item == nil {
			return fmt.Errorf("config template item not found: %s", configSpec.Name)
		}
		newCMObj, err := wrapper.rerenderConfigTemplate(cluster, component, configSpec, item, configRender, defs)
		if err != nil {
			return err
		}
		if newCMObj, err = applyUpdatedParameters(item, newCMObj, configRender, defs); err != nil {
			return err
		}
		if err := wrapper.addRenderedObject(configSpec, newCMObj, componentParameter); err != nil {
			return err
		}
		if err := updateConfigMetaForCM(newCMObj, item, revision); err != nil {
			return err
		}
	}
	return nil
}

func updateConfigMetaForCM(newCMObj *corev1.ConfigMap, item *parametersv1alpha1.ConfigTemplateItemDetail, revision string) (err error) {
	if item == nil {
		return
	}

	annotations := newCMObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	b, err := json.Marshal(item)
	if err != nil {
		return err
	}
	annotations[constant.ConfigAppliedVersionAnnotationKey] = string(b)
	hash, _ := cfgutil.ComputeHash(newCMObj.Data)
	annotations[constant.CMInsCurrentConfigurationHashLabelKey] = hash
	annotations[constant.ConfigurationRevision] = revision
	newCMObj.Annotations = annotations
	return
}

func applyUpdatedParameters(item *parametersv1alpha1.ConfigTemplateItemDetail, orig *corev1.ConfigMap, configRender *parametersv1alpha1.ParameterDrivenConfigRender, paramsDefs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, fmt.Errorf("not support parameter reconfigure")
	}

	newData, err := DoMerge(orig.Data, item.ConfigFileParams, paramsDefs, configRender.Spec.Configs)
	if err != nil {
		return nil, err
	}

	expected := orig.DeepCopy()
	expected.Data = newData
	return expected, nil
}

func (wrapper *renderWrapper) rerenderConfigTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	configSpec appsv1.ComponentTemplateSpec,
	item *parametersv1alpha1.ConfigTemplateItemDetail,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
	defs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
	cmName := core.GetComponentCfgName(cluster.Name, component.Name, configSpec.Name)
	newCMObj, err := wrapper.RenderComponentTemplate(configSpec, cmName, func(m map[string]string) error {
		return validateRenderedData(m, defs, configRender)
	})
	if err != nil {
		return nil, err
	}
	// render user specified template
	if item != nil && item.CustomTemplates != nil {
		newData, err := mergerConfigTemplate(
			appsv1.ConfigTemplateExtension{
				TemplateRef: item.CustomTemplates.TemplateRef,
				Namespace:   item.CustomTemplates.Namespace,
				Policy:      item.CustomTemplates.Policy,
			},
			wrapper.TemplateRender,
			configSpec,
			newCMObj.Data,
			defs,
			configRender)
		if err != nil {
			return nil, err
		}
		newCMObj.Data = newData
	}
	UpdateCMConfigSpecLabels(newCMObj, configSpec)
	return newCMObj, nil
}

func (wrapper *renderWrapper) renderScriptTemplate(cluster *appsv1.Cluster, component *component.SynthesizedComponent,
	localObjs []client.Object, owner client.Object) error {
	for _, templateSpec := range component.ScriptTemplates {
		cmName := core.GetComponentCfgName(cluster.Name, component.Name, templateSpec.Name)
		object := findMatchedLocalObject(localObjs, client.ObjectKey{
			Name:      cmName,
			Namespace: wrapper.cluster.Namespace}, generics.ToGVK(&corev1.ConfigMap{}))
		if object != nil {
			wrapper.addVolumeMountMeta(templateSpec, object.(*corev1.ConfigMap), false)
			continue
		}

		// Generate ConfigMap objects for config files
		cm, err := wrapper.RenderComponentTemplate(templateSpec, cmName, nil)
		if err != nil {
			return err
		}
		if err := wrapper.addRenderedObject(templateSpec, cm, owner); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) addRenderedObject(templateSpec appsv1.ComponentTemplateSpec, cm *corev1.ConfigMap, owner client.Object) (err error) {
	// The owner of the configmap object is a cluster,
	// in order to manage the life cycle of configmap
	if err = intctrlutil.SetControllerReference(owner, cm); err != nil {
		return err
	}

	core.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
	wrapper.addVolumeMountMeta(templateSpec, cm, true)
	return nil
}

func (wrapper *renderWrapper) addVolumeMountMeta(templateSpec appsv1.ComponentTemplateSpec, object *corev1.ConfigMap, rendered bool) {
	wrapper.volumes[object.GetName()] = templateSpec
	if rendered {
		wrapper.renderedObjs = append(wrapper.renderedObjs, object)
	}
	wrapper.templateAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(templateSpec.Name)] = object.GetName()
}

func (wrapper *renderWrapper) CheckAndPatchConfigResource(origCMObj *corev1.ConfigMap, newData map[string]string) error {
	if origCMObj == nil {
		return nil
	}
	if reflect.DeepEqual(origCMObj.Data, newData) {
		return nil
	}

	patch := client.MergeFrom(origCMObj.DeepCopy())
	origCMObj.Data = newData
	if origCMObj.Annotations == nil {
		origCMObj.Annotations = make(map[string]string)
	}
	core.SetParametersUpdateSource(origCMObj, constant.ReconfigureManagerSource)
	rawData, err := json.Marshal(origCMObj.Data)
	if err != nil {
		return err
	}

	origCMObj.Annotations[corev1.LastAppliedConfigAnnotation] = string(rawData)
	return wrapper.cli.Patch(wrapper.ctx, origCMObj, patch)
}

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

func UpdateCMConfigSpecLabels(cm *corev1.ConfigMap, configSpec appsv1.ComponentTemplateSpec) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpec.Name
	cm.Labels[constant.CMConfigurationTemplateNameLabelKey] = configSpec.TemplateRef
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
