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

// MergeConfigTemplates merge ClusterVersion.ComponentDefs[*].ConfigTemplateRefs and ClusterDefinition.ComponentDefs[*].ConfigTemplateRefs
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
		// ClusterVersion replace clusterDefinition
		tplName := configSpec.Name
		if _, ok := (mergedTplMap)[tplName]; ok {
			continue
		}
		mergedCfgTpl = append(mergedCfgTpl, configSpec)
		mergedTplMap[tplName] = struct{}{}
	}

	return mergedCfgTpl
}

func GetClusterVersionResource(cvName string, cv *appsv1alpha1.ClusterVersion, cli client.Client, ctx context.Context) error {
	if cvName == "" {
		return nil
	}
	clusterVersionKey := client.ObjectKey{
		Namespace: "",
		Name:      cvName,
	}
	if err := cli.Get(ctx, clusterVersionKey, cv); err != nil {
		return WrapError(err, "failed to get clusterversion[%s]", cvName)
	}
	return nil
}

func CheckConfigTemplateReconfigureKey(configSpec appsv1alpha1.ComponentConfigSpec, key string) bool {
	if len(configSpec.Keys) == 0 {
		return true
	}
	for _, keySelector := range configSpec.Keys {
		if keySelector == key {
			return true
		}
	}
	return false
}
