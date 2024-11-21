/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

// BuildConfigTemplateAnnotations builds config template annotations for object
func BuildConfigTemplateAnnotations(object client.Object, synthesizedComp *component.SynthesizedComponent) {
	asMapAnnotations := make(map[string]string)
	for _, configTplSpec := range synthesizedComp.ConfigTemplates {
		asMapAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(configTplSpec.Name)] = core.GetComponentCfgName(synthesizedComp.ClusterName, synthesizedComp.Name, configTplSpec.Name)
	}
	for _, scriptTplSpec := range synthesizedComp.ScriptTemplates {
		asMapAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(scriptTplSpec.Name)] = core.GetComponentCfgName(synthesizedComp.ClusterName, synthesizedComp.Name, scriptTplSpec.Name)
	}
	updateLabelsOrAnnotations(asMapAnnotations, object.GetAnnotations, object.SetAnnotations, constant.ConfigurationTplLabelPrefixKey)
}

func updateLabelsOrAnnotations(newLabels map[string]string, getter func() map[string]string, setter func(map[string]string), labelPrefix ...string) {
	existLabels := make(map[string]string)
	updatedLabels := getter()
	if updatedLabels == nil {
		updatedLabels = make(map[string]string)
	}

	for key, val := range updatedLabels {
	matchLabel:
		for _, prefix := range labelPrefix {
			if strings.HasPrefix(key, prefix) {
				existLabels[key] = val
				break matchLabel
			}
		}
	}

	// delete not exist configmap label
	deletedLabels := cfgutil.MapKeyDifference(existLabels, newLabels)
	for l := range deletedLabels.Iter() {
		delete(updatedLabels, l)
	}

	for key, val := range newLabels {
		updatedLabels[key] = val
	}
	setter(updatedLabels)
}
