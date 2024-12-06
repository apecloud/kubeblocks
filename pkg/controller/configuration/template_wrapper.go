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
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
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
	renderedObjs        []client.Object

	renderedSecretObjs []client.Object

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
	component *component.SynthesizedComponent, localObjs []client.Object, configuration *appsv1alpha1.Configuration) error {
	revision := fromConfiguration(configuration)
	for _, configSpec := range component.ConfigTemplates {
		var item *appsv1alpha1.ConfigurationItemDetail
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
			wrapper.addVolumeMountMeta(configSpec.ComponentTemplateSpec, origCMObj, false, !toSecret(configSpec))
			continue
		}
		if configuration != nil {
			item = configuration.Spec.GetConfigurationItem(configSpec.Name)
		}
		newCMObj, err := wrapper.rerenderConfigTemplate(cluster, component, configSpec, item)
		if err != nil {
			return err
		}
		if err := applyUpdatedParameters(item, newCMObj, configSpec, wrapper.cli, wrapper.ctx); err != nil {
			return err
		}
		if err := wrapper.addRenderedObject(configSpec.ComponentTemplateSpec, newCMObj, configuration, !toSecret(configSpec)); err != nil {
			return err
		}
		if err := updateConfigMetaForCM(newCMObj, item, revision); err != nil {
			return err
		}
	}
	return nil
}

func fromConfiguration(configuration *appsv1alpha1.Configuration) string {
	if configuration == nil {
		return ""
	}
	return strconv.FormatInt(configuration.GetGeneration(), 10)
}

func updateConfigMetaForCM(newCMObj *corev1.ConfigMap, item *appsv1alpha1.ConfigurationItemDetail, revision string) (err error) {
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
	annotations[constant.CMConfigurationTemplateVersion] = item.Version
	newCMObj.Annotations = annotations
	return
}

func applyUpdatedParameters(item *appsv1alpha1.ConfigurationItemDetail, cm *corev1.ConfigMap, configSpec appsv1.ComponentConfigSpec, cli client.Client, ctx context.Context) (err error) {
	var newData map[string]string
	var configConstraint *appsv1beta1.ConfigConstraint

	if item == nil || len(item.ConfigFileParams) == 0 {
		return
	}
	if configSpec.ConfigConstraintRef != "" {
		configConstraint, err = fetchConfigConstraint(configSpec.ConfigConstraintRef, ctx, cli)
	}
	if err != nil {
		return
	}
	newData, err = DoMerge(cm.Data, item.ConfigFileParams, configConstraint, configSpec)
	if err != nil {
		return
	}
	cm.Data = newData
	return
}

func (wrapper *renderWrapper) rerenderConfigTemplate(cluster *appsv1.Cluster,
	component *component.SynthesizedComponent,
	configSpec appsv1.ComponentConfigSpec,
	item *appsv1alpha1.ConfigurationItemDetail,
) (*corev1.ConfigMap, error) {
	cmName := core.GetComponentCfgName(cluster.Name, component.Name, configSpec.Name)
	newCMObj, err := wrapper.RenderComponentTemplate(configSpec.ComponentTemplateSpec, cmName, func(m map[string]string) error {
		return validateRenderedData(m, configSpec, wrapper.ctx, wrapper.cli)
	})
	if err != nil {
		return nil, err
	}
	// render user specified template
	if item != nil && item.ImportTemplateRef != nil {
		newData, err := mergerConfigTemplate(
			appsv1.ConfigTemplateExtension{
				TemplateRef: item.ImportTemplateRef.TemplateRef,
				Namespace:   item.ImportTemplateRef.Namespace,
				Policy:      appsv1.MergedPolicy(item.ImportTemplateRef.Policy),
			},
			wrapper.TemplateRender,
			configSpec,
			newCMObj.Data,
			wrapper.ctx,
			wrapper.cli)
		if err != nil {
			return nil, err
		}
		newCMObj.Data = newData
	}
	UpdateCMConfigSpecLabels(newCMObj, configSpec)
	if InjectEnvEnabled(configSpec) && toSecret(configSpec) {
		wrapper.renderedSecretObjs = append(wrapper.renderedSecretObjs, newCMObj)
	}
	return newCMObj, nil
}

func (wrapper *renderWrapper) renderScriptTemplate(cluster *appsv1.Cluster, component *component.SynthesizedComponent,
	localObjs []client.Object) error {
	for _, templateSpec := range component.ScriptTemplates {
		cmName := core.GetComponentCfgName(cluster.Name, component.Name, templateSpec.Name)
		object := findMatchedLocalObject(localObjs, client.ObjectKey{
			Name:      cmName,
			Namespace: wrapper.cluster.Namespace}, generics.ToGVK(&corev1.ConfigMap{}))
		if object != nil {
			wrapper.addVolumeMountMeta(templateSpec, object, false, true)
			continue
		}

		// Generate ConfigMap objects for config files
		cm, err := wrapper.RenderComponentTemplate(templateSpec, cmName, nil)
		if err != nil {
			return err
		}
		if err := wrapper.addRenderedObject(templateSpec, cm, nil, true); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) addRenderedObject(templateSpec appsv1.ComponentTemplateSpec, cm *corev1.ConfigMap, configuration *appsv1alpha1.Configuration, asVolume bool) (err error) {
	// The owner of the configmap object is a cluster,
	// in order to manage the life cycle of configmap
	if configuration != nil {
		err = intctrlutil.SetControllerReference(configuration, cm)
	} else {
		err = intctrlutil.SetControllerReference(wrapper.component, cm)
	}
	if err != nil {
		return err
	}

	core.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
	wrapper.addVolumeMountMeta(templateSpec, cm, true, asVolume)
	return nil
}

func (wrapper *renderWrapper) addVolumeMountMeta(templateSpec appsv1.ComponentTemplateSpec, object client.Object, rendered bool, asVolume bool) {
	if asVolume {
		wrapper.volumes[object.GetName()] = templateSpec
	}
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

func UpdateCMConfigSpecLabels(cm *corev1.ConfigMap, configSpec appsv1.ComponentConfigSpec) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}

	cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpec.Name
	cm.Labels[constant.CMConfigurationTemplateNameLabelKey] = configSpec.TemplateRef
	if configSpec.ConfigConstraintRef != "" {
		cm.Labels[constant.CMConfigurationConstraintsNameLabelKey] = configSpec.ConfigConstraintRef
	}

	if len(configSpec.Keys) != 0 {
		cm.Labels[constant.CMConfigurationCMKeysLabelKey] = strings.Join(configSpec.Keys, ",")
	}
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
func validateRenderedData(
	renderedData map[string]string,
	configSpec appsv1.ComponentConfigSpec,
	ctx context.Context,
	cli client.Client) error {
	if configSpec.ConfigConstraintRef == "" {
		return nil
	}
	configConstraint, err := fetchConfigConstraint(configSpec.ConfigConstraintRef, ctx, cli)
	if err != nil {
		return err
	}
	return validateRawData(renderedData, configSpec, &configConstraint.Spec)
}

func validateRawData(renderedData map[string]string, configSpec appsv1.ComponentConfigSpec, cc *appsv1beta1.ConfigConstraintSpec) error {
	configChecker := validate.NewConfigValidator(cc, validate.WithKeySelector(configSpec.Keys))
	// NOTE: It is necessary to verify the correctness of the data
	if err := configChecker.Validate(renderedData); err != nil {
		return core.WrapError(err, "failed to validate configmap")
	}
	return nil
}
