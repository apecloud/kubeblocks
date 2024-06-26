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

package multiversion

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1 "github.com/apecloud/kubeblocks/apis/extensions/v1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

// covert extensionsv1alpha1.addon resources to extensionsv1.addon

var (
	addonResource = "addons"
	addonGVR      = extensionsv1.GroupVersion.WithResource(addonResource)
)

func init() {
	hook.RegisterCRDConversion(addonGVR, hook.NewNoVersion(1, 0), addonHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func addonHandler() hook.ConversionHandler {
	return &convertor{
		sourceKind: &addon{},
		targetKind: &addon{},
	}
}

type addon struct{}

func (a *addon) kind() string {
	return "Addon"
}

func (a *addon) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
	addonList, err := cli.ExtensionsV1alpha1().Addons().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range addonList.Items {
		addons = append(addons, &addonList.Items[i])
	}
	return addons, nil
}

func (a *addon) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return cli.ExtensionsV1().Addons().Get(ctx, name, metav1.GetOptions{})
}

func (a *addon) convert(source client.Object) client.Object {
	spec := source.(*extensionsv1alpha1.Addon).Spec
	return &extensionsv1.Addon{
		Spec: extensionsv1.AddonSpec{
			Description:          spec.Description,
			Type:                 extensionsv1.AddonType(spec.Type),
			Version:              spec.Version,
			Provider:             spec.Provider,
			Helm:                 a.helm(spec.Helm),
			DefaultInstallValues: a.defaultInstallValues(spec.DefaultInstallValues),
			InstallSpec:          a.installSpec(spec.InstallSpec),
			Installable:          a.installable(spec.Installable),
			CliPlugins:           a.cliPlugins(spec.CliPlugins),
		},
	}
}

func (a *addon) helm(spec *extensionsv1alpha1.HelmTypeInstallSpec) *extensionsv1.HelmTypeInstallSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.HelmTypeInstallSpec{
		ChartLocationURL:  spec.ChartLocationURL,
		InstallOptions:    extensionsv1.HelmInstallOptions(spec.InstallOptions),
		InstallValues:     a.installValues(spec.InstallValues),
		ValuesMapping:     a.valuesMapping(spec.ValuesMapping),
		ChartsImage:       spec.ChartsImage,
		ChartsPathInImage: spec.ChartsPathInImage,
	}
}

func (a *addon) installValues(spec extensionsv1alpha1.HelmInstallValues) extensionsv1.HelmInstallValues {
	return extensionsv1.HelmInstallValues{
		URLs:          spec.URLs,
		ConfigMapRefs: a.dataObjectKeySelector(spec.ConfigMapRefs),
		SecretRefs:    a.dataObjectKeySelector(spec.SecretRefs),
		SetValues:     spec.SetValues,
		SetJSONValues: spec.SetJSONValues,
	}
}

func (a *addon) dataObjectKeySelector(selectors []extensionsv1alpha1.DataObjectKeySelector) []extensionsv1.DataObjectKeySelector {
	if len(selectors) == 0 {
		return nil
	}
	newSelectors := make([]extensionsv1.DataObjectKeySelector, 0)
	for _, selector := range selectors {
		newSelectors = append(newSelectors, extensionsv1.DataObjectKeySelector{
			Name: selector.Name,
			Key:  selector.Key,
		})
	}
	return newSelectors
}

func (a *addon) valuesMapping(spec extensionsv1alpha1.HelmValuesMapping) extensionsv1.HelmValuesMapping {
	return extensionsv1.HelmValuesMapping{
		HelmValuesMappingItem: a.helmValuesMappingItem(spec.HelmValuesMappingItem),
		ExtraItems:            a.helmValuesMappingExtraItem(spec.ExtraItems),
	}
}

func (a *addon) helmValuesMappingItem(item extensionsv1alpha1.HelmValuesMappingItem) extensionsv1.HelmValuesMappingItem {
	return extensionsv1.HelmValuesMappingItem{
		HelmValueMap: extensionsv1.HelmValueMapType{
			ReplicaCount: item.HelmValueMap.ReplicaCount,
			PVEnabled:    item.HelmValueMap.PVEnabled,
			StorageClass: item.HelmValueMap.StorageClass,
		},
		HelmJSONMap: extensionsv1.HelmJSONValueMapType{
			Tolerations: item.HelmJSONMap.Tolerations,
		},
		ResourcesMapping: a.resourceMappingItem(item.ResourcesMapping),
	}
}

func (a *addon) resourceMappingItem(item *extensionsv1alpha1.ResourceMappingItem) *extensionsv1.ResourceMappingItem {
	if item == nil {
		return nil
	}
	newItem := &extensionsv1.ResourceMappingItem{
		Storage: item.Storage,
	}
	if item.CPU != nil {
		newItem.CPU = &extensionsv1.ResourceReqLimItem{
			Requests: item.CPU.Requests,
			Limits:   item.CPU.Limits,
		}
	}
	if item.Memory != nil {
		newItem.Memory = &extensionsv1.ResourceReqLimItem{
			Requests: item.Memory.Requests,
			Limits:   item.Memory.Limits,
		}
	}
	return newItem
}

func (a *addon) helmValuesMappingExtraItem(items []extensionsv1alpha1.HelmValuesMappingExtraItem) []extensionsv1.HelmValuesMappingExtraItem {
	if len(items) == 0 {
		return nil
	}
	newItems := make([]extensionsv1.HelmValuesMappingExtraItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.HelmValuesMappingExtraItem{
			HelmValuesMappingItem: a.helmValuesMappingItem(item.HelmValuesMappingItem),
			Name:                  item.Name,
		})
	}
	return newItems
}

func (a *addon) defaultInstallValues(items []extensionsv1alpha1.AddonDefaultInstallSpecItem) []extensionsv1.AddonDefaultInstallSpecItem {
	if items == nil {
		return nil
	}
	newItems := make([]extensionsv1.AddonDefaultInstallSpecItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.AddonDefaultInstallSpecItem{
			AddonInstallSpec: *a.installSpec(&item.AddonInstallSpec),
			Selectors:        a.selectorRequirement(item.Selectors),
		})
	}
	return newItems
}

func (a *addon) installSpec(spec *extensionsv1alpha1.AddonInstallSpec) *extensionsv1.AddonInstallSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.AddonInstallSpec{
		AddonInstallSpecItem: a.addonInstallSpecItem(spec.AddonInstallSpecItem),
		Enabled:              spec.Enabled,
		ExtraItems:           a.extraItems(spec.ExtraItems),
	}
}

func (a *addon) addonInstallSpecItem(item extensionsv1alpha1.AddonInstallSpecItem) extensionsv1.AddonInstallSpecItem {
	return extensionsv1.AddonInstallSpecItem{
		Replicas:     item.Replicas,
		PVEnabled:    item.PVEnabled,
		StorageClass: item.StorageClass,
		Tolerations:  item.Tolerations,
		Resources: extensionsv1.ResourceRequirements{
			Limits:   item.Resources.Limits,
			Requests: item.Resources.Requests,
		},
	}
}

func (a *addon) extraItems(items []extensionsv1alpha1.AddonInstallExtraItem) []extensionsv1.AddonInstallExtraItem {
	if len(items) == 0 {
		return nil
	}
	newItems := make([]extensionsv1.AddonInstallExtraItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.AddonInstallExtraItem{
			AddonInstallSpecItem: a.addonInstallSpecItem(item.AddonInstallSpecItem),
			Name:                 item.Name,
		})
	}
	return newItems
}

func (a *addon) installable(spec *extensionsv1alpha1.InstallableSpec) *extensionsv1.InstallableSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.InstallableSpec{
		Selectors:   a.selectorRequirement(spec.Selectors),
		AutoInstall: spec.AutoInstall,
	}
}

func (a *addon) selectorRequirement(selectors []extensionsv1alpha1.SelectorRequirement) []extensionsv1.SelectorRequirement {
	if len(selectors) == 0 {
		return nil
	}
	newSelectors := make([]extensionsv1.SelectorRequirement, 0)
	for _, selector := range selectors {
		newSelectors = append(newSelectors, extensionsv1.SelectorRequirement{
			Key:      extensionsv1.AddonSelectorKey(selector.Key),
			Operator: extensionsv1.LineSelectorOperator(selector.Operator),
			Values:   selector.Values,
		})
	}
	return newSelectors
}

func (a *addon) cliPlugins(plugins []extensionsv1alpha1.CliPlugin) []extensionsv1.CliPlugin {
	if plugins == nil {
		return nil
	}
	newPlugins := make([]extensionsv1.CliPlugin, 0)
	for _, plugin := range plugins {
		newPlugins = append(newPlugins, extensionsv1.CliPlugin{
			Name:            plugin.Name,
			IndexRepository: plugin.IndexRepository,
			Description:     plugin.Description,
		})
	}
	return newPlugins
}
