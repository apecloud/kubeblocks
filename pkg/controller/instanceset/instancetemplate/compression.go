/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instancetemplate

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// extract compressed instance templates from the configmap
func getInstanceTemplates(instances []workloads.InstanceTemplate, template *corev1.ConfigMap) []workloads.InstanceTemplate {
	if template == nil {
		return instances
	}

	// if template is found with incorrect format, try it as the whole templates is corrupted.
	if template.BinaryData == nil {
		return nil
	}
	templateData, ok := template.BinaryData[TemplateRefDataKey]
	if !ok {
		return nil
	}
	templateByte, err := reader.DecodeAll(templateData, nil)
	if err != nil {
		return nil
	}
	extraTemplates := make([]workloads.InstanceTemplate, 0)
	err = json.Unmarshal(templateByte, &extraTemplates)
	if err != nil {
		return nil
	}

	return append(instances, extraTemplates...)
}

func findTemplateObject(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*corev1.ConfigMap, error) {
	templateMap, err := getInstanceTemplateMap(its.Annotations)
	// error has been checked in prepare stage, there should be no error occurs
	if err != nil {
		return nil, nil
	}
	for name, templateName := range templateMap {
		if name != its.Name {
			continue
		}
		// find the compressed instance templates, parse them
		template := builder.NewConfigMapBuilder(its.Namespace, templateName).GetObject()
		templateObj, err := tree.Get(template)
		if err != nil {
			return nil, err
		}
		template, _ = templateObj.(*corev1.ConfigMap)
		return template, nil
	}
	return nil, nil
}

func getInstanceTemplateMap(annotations map[string]string) (map[string]string, error) {
	if annotations == nil {
		return nil, nil
	}
	templateRef, ok := annotations[TemplateRefAnnotationKey]
	if !ok {
		return nil, nil
	}
	templateMap := make(map[string]string)
	if err := json.Unmarshal([]byte(templateRef), &templateMap); err != nil {
		return nil, err
	}
	return templateMap, nil
}
