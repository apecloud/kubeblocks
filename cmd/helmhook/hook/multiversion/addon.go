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
		sourceKind: &addonConvertor{},
		targetKind: &addonConvertor{},
	}
}

type addonConvertor struct{}

func (c *addonConvertor) kind() string {
	return "Addon"
}

func (c *addonConvertor) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
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

func (c *addonConvertor) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return cli.ExtensionsV1().Addons().Get(ctx, name, metav1.GetOptions{})
}

func (c *addonConvertor) convert(source client.Object) []client.Object {
	spec := source.(*extensionsv1alpha1.Addon).Spec
	return []client.Object{
		&extensionsv1.Addon{
			Spec: extensionsv1.AddonSpec{
				Description:          spec.Description,
				Type:                 extensionsv1.AddonType(spec.Type),
				Version:              spec.Version,
				Provider:             spec.Provider,
				Helm:                 c.helm(spec.Helm),
				DefaultInstallValues: c.defaultInstallValues(spec.DefaultInstallValues),
				InstallSpec:          c.installSpec(spec.InstallSpec),
				Installable:          c.installable(spec.Installable),
				CliPlugins:           c.cliPlugins(spec.CliPlugins),
			},
		},
	}
}

func (c *addonConvertor) helm(spec *extensionsv1alpha1.HelmTypeInstallSpec) *extensionsv1.HelmTypeInstallSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.HelmTypeInstallSpec{
		ChartLocationURL:  spec.ChartLocationURL,
		InstallOptions:    extensionsv1.HelmInstallOptions(spec.InstallOptions),
		InstallValues:     c.installValues(spec.InstallValues),
		ValuesMapping:     c.valuesMapping(spec.ValuesMapping),
		ChartsImage:       spec.ChartsImage,
		ChartsPathInImage: spec.ChartsPathInImage,
	}
}

func (c *addonConvertor) installValues(spec extensionsv1alpha1.HelmInstallValues) extensionsv1.HelmInstallValues {
	return extensionsv1.HelmInstallValues{
		URLs:          spec.URLs,
		ConfigMapRefs: c.dataObjectKeySelector(spec.ConfigMapRefs),
		SecretRefs:    c.dataObjectKeySelector(spec.SecretRefs),
		SetValues:     spec.SetValues,
		SetJSONValues: spec.SetJSONValues,
	}
}

func (c *addonConvertor) dataObjectKeySelector(selectors []extensionsv1alpha1.DataObjectKeySelector) []extensionsv1.DataObjectKeySelector {
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

func (c *addonConvertor) valuesMapping(spec extensionsv1alpha1.HelmValuesMapping) extensionsv1.HelmValuesMapping {
	return extensionsv1.HelmValuesMapping{
		HelmValuesMappingItem: c.helmValuesMappingItem(spec.HelmValuesMappingItem),
		ExtraItems:            c.helmValuesMappingExtraItem(spec.ExtraItems),
	}
}

func (c *addonConvertor) helmValuesMappingItem(item extensionsv1alpha1.HelmValuesMappingItem) extensionsv1.HelmValuesMappingItem {
	return extensionsv1.HelmValuesMappingItem{
		HelmValueMap: extensionsv1.HelmValueMapType{
			ReplicaCount: item.HelmValueMap.ReplicaCount,
			PVEnabled:    item.HelmValueMap.PVEnabled,
			StorageClass: item.HelmValueMap.StorageClass,
		},
		HelmJSONMap: extensionsv1.HelmJSONValueMapType{
			Tolerations: item.HelmJSONMap.Tolerations,
		},
		ResourcesMapping: c.resourceMappingItem(item.ResourcesMapping),
	}
}

func (c *addonConvertor) resourceMappingItem(item *extensionsv1alpha1.ResourceMappingItem) *extensionsv1.ResourceMappingItem {
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

func (c *addonConvertor) helmValuesMappingExtraItem(items []extensionsv1alpha1.HelmValuesMappingExtraItem) []extensionsv1.HelmValuesMappingExtraItem {
	if len(items) == 0 {
		return nil
	}
	newItems := make([]extensionsv1.HelmValuesMappingExtraItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.HelmValuesMappingExtraItem{
			HelmValuesMappingItem: c.helmValuesMappingItem(item.HelmValuesMappingItem),
			Name:                  item.Name,
		})
	}
	return newItems
}

func (c *addonConvertor) defaultInstallValues(items []extensionsv1alpha1.AddonDefaultInstallSpecItem) []extensionsv1.AddonDefaultInstallSpecItem {
	if items == nil {
		return nil
	}
	newItems := make([]extensionsv1.AddonDefaultInstallSpecItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.AddonDefaultInstallSpecItem{
			AddonInstallSpec: *c.installSpec(&item.AddonInstallSpec),
			Selectors:        c.selectorRequirement(item.Selectors),
		})
	}
	return newItems
}

func (c *addonConvertor) installSpec(spec *extensionsv1alpha1.AddonInstallSpec) *extensionsv1.AddonInstallSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.AddonInstallSpec{
		AddonInstallSpecItem: c.addonInstallSpecItem(spec.AddonInstallSpecItem),
		Enabled:              spec.Enabled,
		ExtraItems:           c.extraItems(spec.ExtraItems),
	}
}

func (c *addonConvertor) addonInstallSpecItem(item extensionsv1alpha1.AddonInstallSpecItem) extensionsv1.AddonInstallSpecItem {
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

func (c *addonConvertor) extraItems(items []extensionsv1alpha1.AddonInstallExtraItem) []extensionsv1.AddonInstallExtraItem {
	if len(items) == 0 {
		return nil
	}
	newItems := make([]extensionsv1.AddonInstallExtraItem, 0)
	for _, item := range items {
		newItems = append(newItems, extensionsv1.AddonInstallExtraItem{
			AddonInstallSpecItem: c.addonInstallSpecItem(item.AddonInstallSpecItem),
			Name:                 item.Name,
		})
	}
	return newItems
}

func (c *addonConvertor) installable(spec *extensionsv1alpha1.InstallableSpec) *extensionsv1.InstallableSpec {
	if spec == nil {
		return nil
	}
	return &extensionsv1.InstallableSpec{
		Selectors:   c.selectorRequirement(spec.Selectors),
		AutoInstall: spec.AutoInstall,
	}
}

func (c *addonConvertor) selectorRequirement(selectors []extensionsv1alpha1.SelectorRequirement) []extensionsv1.SelectorRequirement {
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

func (c *addonConvertor) cliPlugins(plugins []extensionsv1alpha1.CliPlugin) []extensionsv1.CliPlugin {
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
