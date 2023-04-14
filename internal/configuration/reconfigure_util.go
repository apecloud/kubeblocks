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
	"encoding/json"
	"reflect"

	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func getUpdateParameterList(cfg *ConfigPatchInfo, trimField string) ([]string, error) {
	params := make([]string, 0)
	walkFn := func(parent, cur string, v reflect.Value, fn util.UpdateFn) error {
		if cur != "" {
			if parent != "" {
				cur = parent + "." + cur
			}
			params = append(params, cur)
		}
		return nil
	}

	for _, diff := range cfg.UpdateConfig {
		var err error
		var updatedParams any
		if err = json.Unmarshal(diff, &updatedParams); err != nil {
			return nil, err
		}
		if updatedParams, err = trimNestedField(updatedParams, trimField); err != nil {
			return nil, err
		}
		if err := util.UnstructuredObjectWalk(updatedParams, walkFn, true); err != nil {
			return nil, WrapError(err, "failed to walk params: [%s]", diff)
		}
	}
	return params, nil
}

func trimNestedField(updatedParams any, trimField string) (any, error) {
	if trimField == "" {
		return updatedParams, nil
	}
	if m, ok := updatedParams.(map[string]interface{}); ok {
		trimParams, found, err := unstructured.NestedFieldNoCopy(m, trimField)
		if err != nil {
			return nil, err
		}
		if found {
			return trimParams, nil
		}
	}
	return updatedParams, nil
}

// IsUpdateDynamicParameters is used to check whether the changed parameters require a restart
func IsUpdateDynamicParameters(cc *appsv1alpha1.ConfigConstraintSpec, cfg *ConfigPatchInfo) (bool, error) {
	if len(cfg.DeleteConfig) > 0 || len(cfg.AddConfig) > 0 {
		return false, nil
	}

	params, err := getUpdateParameterList(cfg, NestedPrefixField(cc.FormatterConfig))
	if err != nil {
		return false, err
	}
	updateParams := set.NewLinkedHashSetString(params...)

	// if ConfigConstraint has StaticParameters, check updated parameter
	if len(cc.StaticParameters) > 0 {
		staticParams := set.NewLinkedHashSetString(cc.StaticParameters...)
		union := util.Union(staticParams, updateParams)
		if union.Length() > 0 {
			return false, nil
		}
		// if no dynamicParameters is configured, reload is the default behavior
		if len(cc.DynamicParameters) == 0 {
			return true, nil
		}
	}

	// if ConfigConstraint has DynamicParameter, all updated param in dynamic params
	if len(cc.DynamicParameters) > 0 {
		dynamicParams := set.NewLinkedHashSetString(cc.DynamicParameters...)
		union := util.Difference(updateParams, dynamicParams)
		return union.Length() == 0, nil
	}

	// if the updated parameter is not in list of DynamicParameter and in list of StaticParameter,
	// restart is the default behavior.
	return false, nil
}

// IsParametersUpdateFromManager is used to check whether the parameters are updated from manager
func IsParametersUpdateFromManager(cm *corev1.ConfigMap) bool {
	annotation := cm.ObjectMeta.Annotations
	if annotation == nil {
		return false
	}
	v := annotation[constant.KBParameterUpdateSourceAnnotationKey]
	return v == constant.ReconfigureManagerSource
}

// IsNotUserReconfigureOperation is used to check whether the parameters are updated from operation
func IsNotUserReconfigureOperation(cm *corev1.ConfigMap) bool {
	labels := cm.GetLabels()
	annotations := cm.GetAnnotations()
	if labels == nil || annotations == nil {
		return false
	}
	if _, ok := annotations[constant.CMInsEnableRerenderTemplateKey]; !ok {
		return false
	}
	lastReconfigurePhase := labels[constant.CMInsLastReconfigurePhaseKey]
	if annotations[constant.KBParameterUpdateSourceAnnotationKey] != constant.ReconfigureManagerSource {
		return false
	}
	return lastReconfigurePhase == "" || ReconfigureCreatedPhase == lastReconfigurePhase
}

// SetParametersUpdateSource is used to set the parameters update source
// manager: parameter only updated from manager
// external-template: parameter only updated from template
// ops: parameter has updated from operation
func SetParametersUpdateSource(cm *corev1.ConfigMap, source string) {
	annotation := cm.GetAnnotations()
	if annotation == nil {
		annotation = make(map[string]string)
	}
	annotation[constant.KBParameterUpdateSourceAnnotationKey] = source
	cm.SetAnnotations(annotation)
}

func IsSchedulableConfigResource(object client.Object) bool {
	var requiredLabels = []string{
		constant.AppNameLabelKey,
		constant.AppInstanceLabelKey,
		constant.KBAppComponentLabelKey,
		constant.CMConfigurationTemplateNameLabelKey,
		constant.CMConfigurationTypeLabelKey,
		constant.CMConfigurationSpecProviderLabelKey,
	}

	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}
	for _, label := range requiredLabels {
		if _, ok := labels[label]; !ok {
			return false
		}
	}
	return true
}

func hasImportTemplate(cm *corev1.ConfigMap) bool {
	annotations := cm.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[constant.CMImportedConfigTemplateLabelKey]
	return ok
}

func isKnownImportTemplate(templateKey client.ObjectKey, cm *corev1.ConfigMap) bool {
	annotations := cm.GetAnnotations()
	if annotations == nil {
		return false
	}
	v, ok := annotations[constant.CMImportedConfigTemplateLabelKey]
	if !ok {
		return false
	}
	return v == templateKey.String()
}

// IsChangedImportTemplate is used to check whether the imported template is changed
// case1: cancel imported template
// case2: add imported template or change imported template
func IsChangedImportTemplate(template *appsv1alpha1.ImportConfigTemplate, cm *corev1.ConfigMap) bool {
	if template == nil {
		return hasImportTemplate(cm)
	}

	templateKey := client.ObjectKey{
		Namespace: template.Namespace,
		Name:      template.TemplateRef,
	}
	return !isKnownImportTemplate(templateKey, cm)
}
