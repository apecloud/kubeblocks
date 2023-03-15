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

	"sigs.k8s.io/controller-runtime/pkg/client"

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

// GetConfigTemplatesFromComponent returns ConfigTemplate list used by the component.
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
		avTpls []appsv1alpha1.ComponentConfigSpec
		cdTpls []appsv1alpha1.ComponentConfigSpec
	)

	if aCom != nil {
		avTpls = aCom.ConfigSpecs
	}
	if dCom != nil {
		cdTpls = dCom.ConfigSpecs
	}

	return MergeConfigTemplates(avTpls, cdTpls), nil
}

// MergeConfigTemplates merge ClusterVersion.ComponentDefs[*].ConfigTemplateRefs and ClusterDefinition.ComponentDefs[*].ConfigTemplateRefs
func MergeConfigTemplates(clusterVersionTpl []appsv1alpha1.ComponentConfigSpec,
	cdTpl []appsv1alpha1.ComponentConfigSpec) []appsv1alpha1.ComponentConfigSpec {
	if len(clusterVersionTpl) == 0 {
		return cdTpl
	}

	if len(cdTpl) == 0 {
		return clusterVersionTpl
	}

	mergedCfgTpl := make([]appsv1alpha1.ComponentConfigSpec, 0, len(clusterVersionTpl)+len(cdTpl))
	mergedTplMap := make(map[string]struct{}, cap(mergedCfgTpl))

	for _, tpl := range clusterVersionTpl {
		tplName := tpl.Name
		if _, ok := (mergedTplMap)[tplName]; ok {
			// It's been checked in validation webhook
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, tpl)
		mergedTplMap[tplName] = struct{}{}
	}

	for _, tpl := range cdTpl {
		// ClusterVersion replace clusterDefinition
		tplName := tpl.Name
		if _, ok := (mergedTplMap)[tplName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, tpl)
		mergedTplMap[tplName] = struct{}{}
	}

	return mergedCfgTpl
}

func GetClusterVersionResource(cvName string, cv *appsv1alpha1.ClusterVersion, cli client.Client, ctx context.Context) error {
	if cvName == "" {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: "",
		Name:      cvName,
	}, cv); err != nil {
		return WrapError(err, "failed to get clusterversion[%s]", cvName)
	}
	return nil
}

func CheckConfigTemplateReconfigureKey(tpl appsv1alpha1.ComponentConfigSpec, key string) bool {
	if len(tpl.Keys) == 0 {
		return true
	}
	for _, keySelector := range tpl.Keys {
		if keySelector == key {
			return true
		}
	}
	return false
}
