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

package core

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ComponentsType interface {
	appsv1alpha1.ClusterComponentVersion | appsv1alpha1.ClusterComponentDefinition | appsv1alpha1.ClusterComponentSpec
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

// GetConfigTemplatesFromComponent returns ConfigTemplate list used by the component
func GetConfigTemplatesFromComponent(
	cComponents []appsv1alpha1.ClusterComponentSpec,
	dComponents []appsv1alpha1.ClusterComponentDefinition,
	aComponents []appsv1alpha1.ClusterComponentVersion,
	componentName string) ([]appsv1alpha1.ComponentConfigSpec, error) {
	findCompTypeByName := func(comName string) *appsv1alpha1.ClusterComponentSpec {
		return filter(cComponents, func(o appsv1alpha1.ClusterComponentSpec) bool {
			return o.Name == comName
		})
	}

	cCom := findCompTypeByName(componentName)
	if cCom == nil {
		return nil, MakeError("failed to find component[%s]", componentName)
	}
	aCom := filter(aComponents, func(o appsv1alpha1.ClusterComponentVersion) bool {
		return o.ComponentDefRef == cCom.ComponentDefRef
	})
	dCom := filter(dComponents, func(o appsv1alpha1.ClusterComponentDefinition) bool {
		return o.Name == cCom.ComponentDefRef
	})

	var (
		cvConfigSpecs []appsv1alpha1.ComponentConfigSpec
		cdConfigSpecs []appsv1alpha1.ComponentConfigSpec
	)

	if aCom != nil {
		cvConfigSpecs = aCom.ConfigSpecs
	}
	if dCom != nil {
		cdConfigSpecs = dCom.ConfigSpecs
	}

	return MergeConfigTemplates(cvConfigSpecs, cdConfigSpecs), nil
}

// MergeConfigTemplates merges ClusterVersion.ComponentDefs[*].ConfigTemplateRefs and ClusterDefinition.ComponentDefs[*].ConfigTemplateRefs
func MergeConfigTemplates(cvConfigSpecs []appsv1alpha1.ComponentConfigSpec,
	cdConfigSpecs []appsv1alpha1.ComponentConfigSpec) []appsv1alpha1.ComponentConfigSpec {
	if len(cvConfigSpecs) == 0 {
		return cdConfigSpecs
	}

	if len(cdConfigSpecs) == 0 {
		return cvConfigSpecs
	}

	mergedCfgTpl := make([]appsv1alpha1.ComponentConfigSpec, 0, len(cvConfigSpecs)+len(cdConfigSpecs))
	mergedTplMap := make(map[string]struct{}, cap(mergedCfgTpl))

	for _, configSpec := range cvConfigSpecs {
		tplName := configSpec.Name
		if _, ok := (mergedTplMap)[tplName]; ok {
			// It's been checked in validation webhook
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, configSpec)
		mergedTplMap[tplName] = struct{}{}
	}

	for _, configSpec := range cdConfigSpecs {
		// ClusterVersion replaces clusterDefinition
		tplName := configSpec.Name
		if _, ok := (mergedTplMap)[tplName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, configSpec)
		mergedTplMap[tplName] = struct{}{}
	}

	return mergedCfgTpl
}

func IsSupportConfigFileReconfigure(configTemplateSpec appsv1alpha1.ComponentConfigSpec, configFileKey string) bool {
	if len(configTemplateSpec.Keys) == 0 {
		return true
	}
	for _, keySelector := range configTemplateSpec.Keys {
		if keySelector == configFileKey {
			return true
		}
	}
	return false
}
