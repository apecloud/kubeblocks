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

package configuration

import (
	"fmt"

	"github.com/StudioSol/set"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type ConfigType string

const (
	CfgCmType    ConfigType = "configmap"
	CfgTplType   ConfigType = "configConstraint"
	CfgLocalType ConfigType = "local"
	CfgRawType   ConfigType = "raw"
)

type RawConfig struct {
	// formatter
	Type appsv1alpha1.CfgFileFormat

	RawData string
}

type IniContext struct {
	SectionName string
}

// XMLContext TODO(zt) Support Xml config
type XMLContext struct {
}

type CfgOpOption struct {
	// optional
	VolumeName string
	// optional
	FileName string

	// option
	// for all configuration
	AllSearch bool

	// optional
	IniContext *IniContext
	// optional
	XMLContext *XMLContext
}

type ParameterPair struct {
	Key   string
	Value string
}

// ParameterUpdateType describes how to update the parameters.
// +enum
type ParameterUpdateType string

const (
	AddedType   ParameterUpdateType = "add"
	DeletedType ParameterUpdateType = "delete"
	UpdatedType ParameterUpdateType = "update"
)

type VisualizedParam struct {
	Key        string
	UpdateType ParameterUpdateType
	Parameters []ParameterPair
}

type ConfigOperator interface {
	// MergeFrom update parameter by keyvalue
	MergeFrom(params map[string]interface{}, option CfgOpOption) error

	// MergeFromConfig(fileContent []byte, option CfgOpOption) error
	// MergePatch(jsonPatch []byte, option CfgOpOption) error
	// Diff(target *ConfigOperator) (*ConfigPatchInfo, error)

	// Query get parameter
	Query(jsonpath string, option CfgOpOption) ([]byte, error)

	// ToCfgContent to configuration file content
	ToCfgContent() (map[string]string, error)
}

type GetResourceFn func(key client.ObjectKey) (map[string]string, error)

type ConfigResource struct {
	CfgKey         client.ObjectKey
	ResourceReader GetResourceFn

	// configmap data
	ConfigData map[string]string
	CMKeys     *set.LinkedHashSetString
}

type CfgOption struct {
	Type ConfigType
	Log  logr.Logger

	// formatter
	CfgType appsv1alpha1.CfgFileFormat

	// Path for CfgLocalType test
	Path    string
	RawData []byte

	// ConfigResource for k8s resource
	ConfigResource *ConfigResource
}

func FromConfigData(data map[string]string, cmKeys *set.LinkedHashSetString) *ConfigResource {
	return &ConfigResource{
		ConfigData: data,
		CMKeys:     cmKeys,
	}
}

// GenerateTPLUniqLabelKeyWithConfig generate uniq key for configuration template
// reference: docs/img/reconfigure-cr-relationship.drawio.png
func GenerateTPLUniqLabelKeyWithConfig(configKey string) string {
	return GenerateUniqKeyWithConfig(constant.ConfigurationTplLabelPrefixKey, configKey)
}

// GenerateUniqKeyWithConfig is similar to getInstanceCfgCMName, generate uniq label or annotations for configuration template
func GenerateUniqKeyWithConfig(label string, configKey string) string {
	return fmt.Sprintf("%s-%s", label, configKey)
}

// GenerateConstraintsUniqLabelKeyWithConfig generate uniq key for configure template
// reference: docs/img/reconfigure-cr-relationship.drawio.png
func GenerateConstraintsUniqLabelKeyWithConfig(configKey string) string {
	return GenerateUniqKeyWithConfig(constant.ConfigurationConstraintsLabelPrefixKey, configKey)
}

// GetInstanceCMName  {{statefull.Name}}-{{clusterVersion.Name}}-{{tpl.Name}}-"config"
func GetInstanceCMName(obj client.Object, tpl *appsv1alpha1.ComponentTemplateSpec) string {
	return getInstanceCfgCMName(obj.GetName(), tpl.Name)
	// return fmt.Sprintf("%s-%s-config", sts.GetName(), tpl.VolumeName)
}

// getInstanceCfgCMName configmap generation rule for configuration file.
// {{statefulset.Name}}-{{volumeName}}
func getInstanceCfgCMName(objName, tplName string) string {
	return fmt.Sprintf("%s-%s", objName, tplName)
}

// GetComponentCfgName is similar to getInstanceCfgCMName, without statefulSet object.
func GetComponentCfgName(clusterName, componentName, tplName string) string {
	return getInstanceCfgCMName(fmt.Sprintf("%s-%s", clusterName, componentName), tplName)
}
