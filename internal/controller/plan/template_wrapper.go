/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package plan

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type templateRenderValidator = func(map[string]string) error

type renderWrapper struct {
	templateBuilder *configTemplateBuilder

	volumes             map[string]appsv1alpha1.ComponentTemplateSpec
	templateAnnotations map[string]string
	renderedObjs        []client.Object

	ctx     context.Context
	cli     client.Client
	cluster *appsv1alpha1.Cluster
	params  builder.BuilderParams
}

func newTemplateRenderWrapper(templateBuilder *configTemplateBuilder, cluster *appsv1alpha1.Cluster, params builder.BuilderParams, ctx context.Context, cli client.Client) renderWrapper {
	return renderWrapper{
		ctx:     ctx,
		cli:     cli,
		cluster: cluster,
		params:  params,

		templateBuilder:     templateBuilder,
		templateAnnotations: make(map[string]string),
		volumes:             make(map[string]appsv1alpha1.ComponentTemplateSpec),
	}
}

func (wrapper *renderWrapper) checkRerenderTemplateSpec(cfgCMName string, task *intctrltypes.ReconcileTask) (bool, *corev1.ConfigMap, error) {
	cmKey := client.ObjectKey{
		Name:      cfgCMName,
		Namespace: wrapper.cluster.Namespace,
	}

	cmObj := &corev1.ConfigMap{}
	localObject := task.GetLocalResourceWithObjectKey(cmKey, generics.ToGVK(cmObj))
	if localObject != nil {
		if cm, ok := localObject.(*corev1.ConfigMap); ok {
			return false, cm, nil
		}
	}

	cmErr := wrapper.cli.Get(wrapper.ctx, cmKey, cmObj)
	if cmErr != nil && !apierrors.IsNotFound(cmErr) {
		// An unexpected error occurs
		return false, nil, cmErr
	}
	if cmErr != nil {
		// Config is not exists
		return true, nil, nil
	}

	// Config is exists
	return cfgcore.IsNotUserReconfigureOperation(cmObj), cmObj, nil
}

func (wrapper *renderWrapper) renderConfigTemplate(task *intctrltypes.ReconcileTask) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, configSpec := range task.Component.ConfigTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, configSpec.Name)
		enableRerender, origCMObj, err := wrapper.checkRerenderTemplateSpec(cmName, task)
		if err != nil {
			return err
		}
		if !enableRerender {
			wrapper.addVolumeMountMeta(configSpec.ComponentTemplateSpec, origCMObj)
			continue
		}
		// Generate ConfigMap objects for config files
		newCMObj, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, configSpec.ConfigConstraintRef, configSpec.ComponentTemplateSpec,
			wrapper.params, wrapper.ctx, wrapper.cli, func(m map[string]string) error {
				return validateRenderedData(m, configSpec, wrapper.ctx, wrapper.cli)
			})
		if err != nil {
			return err
		}
		if err := wrapper.checkAndPatchConfigResource(origCMObj, newCMObj.Data); err != nil {
			return err
		}
		updateCMConfigSpecLabels(newCMObj, configSpec)
		if err := wrapper.addRenderedObject(configSpec.ComponentTemplateSpec, newCMObj, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) renderScriptTemplate(task *intctrltypes.ReconcileTask) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, templateSpec := range task.Component.ScriptTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, templateSpec.Name)
		object := task.GetLocalResourceWithObjectKey(client.ObjectKey{
			Name:      cmName,
			Namespace: wrapper.cluster.Namespace,
		}, generics.ToGVK(&corev1.ConfigMap{}))
		if object != nil {
			wrapper.addVolumeMountMeta(templateSpec, object)
			continue
		}

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, "", templateSpec, wrapper.params, wrapper.ctx, wrapper.cli, nil)
		if err != nil {
			return err
		}
		if err := wrapper.addRenderedObject(templateSpec, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) addRenderedObject(templateSpec appsv1alpha1.ComponentTemplateSpec, cm *corev1.ConfigMap, scheme *runtime.Scheme) error {
	// The owner of the configmap object is a cluster of users,
	// in order to manage the life cycle of configmap
	if err := controllerutil.SetOwnerReference(wrapper.cluster, cm, scheme); err != nil {
		return err
	}

	cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
	wrapper.addVolumeMountMeta(templateSpec, cm)
	return nil
}

func (wrapper *renderWrapper) addVolumeMountMeta(templateSpec appsv1alpha1.ComponentTemplateSpec, object client.Object) {
	wrapper.volumes[object.GetName()] = templateSpec
	wrapper.renderedObjs = append(wrapper.renderedObjs, object)
	wrapper.templateAnnotations[cfgcore.GenerateTPLUniqLabelKeyWithConfig(templateSpec.Name)] = object.GetName()
}

func (wrapper *renderWrapper) checkAndPatchConfigResource(origCMObj *corev1.ConfigMap, newData map[string]string) error {
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
	cfgcore.SetParametersUpdateSource(origCMObj, constant.ReconfigureManagerSource)
	rawData, err := json.Marshal(origCMObj.Data)
	if err != nil {
		return err
	}

	origCMObj.Annotations[corev1.LastAppliedConfigAnnotation] = string(rawData)
	return wrapper.cli.Patch(wrapper.ctx, origCMObj, patch)
}

func updateCMConfigSpecLabels(cm *corev1.ConfigMap, configSpec appsv1alpha1.ComponentConfigSpec) {
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

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	configConstraintName string,
	templateSpec appsv1alpha1.ComponentTemplateSpec,
	params builder.BuilderParams,
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
	return builder.BuildConfigMapWithTemplate(configs, params, cmName, configConstraintName, templateSpec)
}

// renderConfigMapTemplate render config file using template engine
func renderConfigMapTemplate(
	templateBuilder *configTemplateBuilder,
	templateSpec appsv1alpha1.ComponentTemplateSpec,
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
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}

// validateRenderedData validate config file against constraint
func validateRenderedData(
	renderedData map[string]string,
	configSpec appsv1alpha1.ComponentConfigSpec,
	ctx context.Context,
	cli client.Client) error {
	configConstraint := &appsv1alpha1.ConfigConstraint{}
	if configSpec.ConfigConstraintRef == "" {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}, configConstraint); err != nil {
		return cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", configSpec)
	}

	// NOTE: not require checker configuration template status
	configChecker := cfgcore.NewConfigValidator(&configConstraint.Spec, cfgcore.WithKeySelector(configSpec.Keys))

	// NOTE: It is necessary to verify the correctness of the data
	if err := configChecker.Validate(renderedData); err != nil {
		return cfgcore.WrapError(err, "failed to validate configmap")
	}

	return nil
}
