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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type ComponentsType interface {
	dbaasv1alpha1.ClusterVersionComponent | dbaasv1alpha1.ClusterDefinitionComponent | dbaasv1alpha1.ClusterComponent
}

type filterFn[T ComponentsType] func(o T) bool

func filter[T ComponentsType](components []T, f filterFn[T]) *T {
	for _, c := range components {
		if f(c) {
			return &c
		}
	}
	return nil
}

// GetConfigTemplatesFromComponent returns ConfigTemplate list used by the component.
func GetConfigTemplatesFromComponent(
	cComponents []dbaasv1alpha1.ClusterComponent,
	dComponents []dbaasv1alpha1.ClusterDefinitionComponent,
	aComponents []dbaasv1alpha1.ClusterVersionComponent,
	componentName string) ([]dbaasv1alpha1.ConfigTemplate, error) {
	findCompTypeByName := func(comName string) *dbaasv1alpha1.ClusterComponent {
		return filter(cComponents, func(o dbaasv1alpha1.ClusterComponent) bool {
			return o.Name == comName
		})
	}

	cCom := findCompTypeByName(componentName)
	if cCom == nil {
		return nil, MakeError("failed to find component[%s]", componentName)
	}
	aCom := filter(aComponents, func(o dbaasv1alpha1.ClusterVersionComponent) bool {
		return o.Type == cCom.Type
	})
	dCom := filter(dComponents, func(o dbaasv1alpha1.ClusterDefinitionComponent) bool {
		return o.Name == cCom.Type
	})

	var (
		avTpls []dbaasv1alpha1.ConfigTemplate
		cdTpls []dbaasv1alpha1.ConfigTemplate
	)

	if aCom != nil {
		avTpls = aCom.ConfigTemplateRefs
	}
	if dCom != nil && dCom.ConfigSpec != nil {
		cdTpls = dCom.ConfigSpec.ConfigTemplateRefs
	}

	return MergeConfigTemplates(avTpls, cdTpls), nil
}

// MergeConfigTemplates merge ClusterVersion.ComponentDefs[*].ConfigTemplateRefs and ClusterDefinition.ComponentDefs[*].ConfigTemplateRefs
func MergeConfigTemplates(clusterVersionTpl []dbaasv1alpha1.ConfigTemplate,
	cdTpl []dbaasv1alpha1.ConfigTemplate) []dbaasv1alpha1.ConfigTemplate {
	if len(clusterVersionTpl) == 0 {
		return cdTpl
	}

	if len(cdTpl) == 0 {
		return clusterVersionTpl
	}

	mergedCfgTpl := make([]dbaasv1alpha1.ConfigTemplate, 0, len(clusterVersionTpl)+len(cdTpl))
	mergedTplMap := make(map[string]struct{}, cap(mergedCfgTpl))

	for _, tpl := range clusterVersionTpl {
		volumeName := tpl.VolumeName
		if _, ok := (mergedTplMap)[volumeName]; ok {
			// It's been checked in validation webhook
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, tpl)
		mergedTplMap[volumeName] = struct{}{}
	}

	for _, tpl := range cdTpl {
		// ClusterVersion replace clusterDefinition
		volumeName := tpl.VolumeName
		if _, ok := (mergedTplMap)[volumeName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, tpl)
		mergedTplMap[volumeName] = struct{}{}
	}

	return mergedCfgTpl
}
