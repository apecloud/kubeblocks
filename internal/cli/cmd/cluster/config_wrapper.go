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

package cluster

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
)

type configWrapper struct {
	create.CreateOptions

	clusterName   string
	updatedParams map[string]string

	// autofill field
	componentName  string
	configSpecName string
	configFileKey  string

	configTemplateSpec appsv1alpha1.ComponentConfigSpec

	clusterObj    *appsv1alpha1.Cluster
	clusterDefObj *appsv1alpha1.ClusterDefinition
	clusterVerObj *appsv1alpha1.ClusterVersion
}

func (w *configWrapper) ConfigTemplateSpec() *appsv1alpha1.ComponentConfigSpec {
	return &w.configTemplateSpec
}

func (w *configWrapper) ConfigSpecName() string {
	return w.configSpecName
}

func (w *configWrapper) ComponentName() string {
	return w.componentName
}

func (w *configWrapper) ConfigFile() string {
	return w.configFileKey
}

// AutoFillRequiredParam auto fills required param.
func (w *configWrapper) AutoFillRequiredParam() error {
	if err := w.fillComponent(); err != nil {
		return err
	}
	if err := w.fillConfigSpec(); err != nil {
		return err
	}
	return w.fillConfigFile()
}

// ValidateRequiredParam validates required param.
func (w *configWrapper) ValidateRequiredParam() error {
	// step1: check existence of component.
	if w.clusterObj.Spec.GetComponentByName(w.componentName) == nil {
		return makeComponentNotExistErr(w.clusterName, w.componentName)
	}

	// step2: check existence of configmap
	cmObj := corev1.ConfigMap{}
	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(w.clusterName, w.componentName, w.configSpecName),
		Namespace: w.Namespace,
	}
	if err := util.GetResourceObjectFromGVR(types.ConfigmapGVR(), cmKey, w.Dynamic, &cmObj); err != nil {
		return err
	}

	// step3: check existence of config file
	if _, ok := cmObj.Data[w.configFileKey]; !ok {
		return makeNotFoundConfigFileErr(w.configFileKey, w.configSpecName, cfgutil.ToSet(cmObj.Data).AsSlice())
	}

	// TODO support all config file update.
	// if !cfgcore.IsSupportConfigFileReconfigure(w.configTemplateSpec, w.configFileKey) {
	//	return makeNotSupportConfigFileUpdateErr(w.configFileKey, w.configTemplateSpec)
	// }
	return nil
}

func (w *configWrapper) fillComponent() error {
	if w.componentName != "" {
		return nil
	}
	componentNames, err := util.GetComponentsFromResource(w.clusterObj.Spec.ComponentSpecs, w.clusterDefObj)
	if err != nil {
		return err
	}
	if len(componentNames) != 1 {
		return cfgcore.MakeError(multiComponentsErrorMessage)
	}
	w.componentName = componentNames[0]
	return nil
}

func (w *configWrapper) fillConfigSpec() error {
	foundConfigSpec := func(configSpecs []appsv1alpha1.ComponentConfigSpec, name string) *appsv1alpha1.ComponentConfigSpec {
		for _, configSpec := range configSpecs {
			if configSpec.Name == name {
				w.configTemplateSpec = configSpec
				return &configSpec
			}
		}
		return nil
	}

	var vComponents []appsv1alpha1.ClusterComponentVersion
	var cComponents = w.clusterObj.Spec.ComponentSpecs
	var dComponents = w.clusterDefObj.Spec.ComponentDefs

	if w.clusterVerObj != nil {
		vComponents = w.clusterVerObj.Spec.ComponentVersions
	}

	configSpecs, err := util.GetConfigTemplateListWithResource(cComponents, dComponents, vComponents, w.componentName, true)
	if err != nil {
		return err
	}
	if len(configSpecs) == 0 {
		return makeNotFoundTemplateErr(w.clusterName, w.componentName)
	}

	if w.configSpecName != "" {
		if foundConfigSpec(configSpecs, w.configSpecName) == nil {
			return makeConfigSpecNotExistErr(w.clusterName, w.componentName, w.configSpecName)
		}
		return nil
	}

	w.configTemplateSpec = configSpecs[0]
	if len(configSpecs) == 1 {
		w.configSpecName = configSpecs[0].Name
		return nil
	}

	supportUpdatedTpl := make([]appsv1alpha1.ComponentConfigSpec, 0)
	for _, configSpec := range configSpecs {
		if ok, err := util.IsSupportReconfigureParams(configSpec, w.updatedParams, w.Dynamic); err == nil && ok {
			supportUpdatedTpl = append(supportUpdatedTpl, configSpec)
		}
	}
	if len(supportUpdatedTpl) == 1 {
		w.configTemplateSpec = configSpecs[0]
		w.configSpecName = supportUpdatedTpl[0].Name
		return nil
	}
	return cfgcore.MakeError(multiConfigTemplateErrorMessage)
}

func (w *configWrapper) fillConfigFile() error {
	if w.configFileKey != "" {
		return nil
	}

	if w.configTemplateSpec.TemplateRef == "" {
		return makeNotFoundTemplateErr(w.clusterName, w.componentName)
	}

	cmObj := corev1.ConfigMap{}
	cmKey := client.ObjectKey{
		Name:      cfgcore.GetComponentCfgName(w.clusterName, w.componentName, w.configSpecName),
		Namespace: w.Namespace,
	}
	if err := util.GetResourceObjectFromGVR(types.ConfigmapGVR(), cmKey, w.Dynamic, &cmObj); err != nil {
		return err
	}
	if len(cmObj.Data) == 0 {
		return cfgcore.MakeError("not supported reconfiguring because there is no config file.")
	}

	keys := w.filterForReconfiguring(cmObj.Data)
	if len(keys) == 1 {
		w.configFileKey = keys[0]
		return nil
	}
	return cfgcore.MakeError(multiConfigFileErrorMessage)
}

func (w *configWrapper) filterForReconfiguring(data map[string]string) []string {
	keys := make([]string, 0, len(data))
	for configFileKey := range data {
		if cfgcore.IsSupportConfigFileReconfigure(w.configTemplateSpec, configFileKey) {
			keys = append(keys, configFileKey)
		}
	}
	return keys
}

func newConfigWrapper(baseOptions create.CreateOptions, clusterName, componentName, configSpec, configKey string, params map[string]string) (*configWrapper, error) {
	var (
		err           error
		clusterObj    *appsv1alpha1.Cluster
		clusterDefObj *appsv1alpha1.ClusterDefinition
	)

	if clusterObj, err = cluster.GetClusterByName(baseOptions.Dynamic, clusterName, baseOptions.Namespace); err != nil {
		return nil, err
	}
	if clusterDefObj, err = cluster.GetClusterDefByName(baseOptions.Dynamic, clusterObj.Spec.ClusterDefRef); err != nil {
		return nil, err
	}

	w := &configWrapper{
		CreateOptions: baseOptions,
		clusterObj:    clusterObj,
		clusterDefObj: clusterDefObj,
		clusterName:   clusterName,

		componentName:  componentName,
		configSpecName: configSpec,
		configFileKey:  configKey,
		updatedParams:  params,
	}

	if w.clusterObj.Spec.ClusterVersionRef == "" {
		return w, err
	}

	clusterVerObj := &appsv1alpha1.ClusterVersion{}
	if err := util.GetResourceObjectFromGVR(types.ClusterVersionGVR(), client.ObjectKey{
		Namespace: "",
		Name:      w.clusterObj.Spec.ClusterVersionRef,
	}, w.Dynamic, clusterVerObj); err != nil {
		return nil, err
	}

	w.clusterVerObj = clusterVerObj
	return w, nil
}
