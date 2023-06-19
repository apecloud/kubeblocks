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
	"encoding/json"
	"path"
	"reflect"
	"strings"

	"github.com/StudioSol/set"
	"github.com/spf13/cast"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/unstructured"
)

type ConfigLoaderProvider func(option CfgOption) (*cfgWrapper, error)

// ReconfiguringProgress defines the progress percentage.
// range: 0~100
// Unconfirmed(-1) describes an uncertain progress, e.g: fsm is failed.
// +enum
type ReconfiguringProgress int32

type PolicyExecStatus struct {
	PolicyName string
	ExecStatus string
	Status     string

	SucceedCount  int32
	ExpectedCount int32
}

type ConfigEventContext struct {
	Client  client.Client
	ReqCtx  intctrlutil.RequestCtx
	Cluster *appsv1alpha1.Cluster

	ClusterComponent *appsv1alpha1.ClusterComponentSpec
	Component        *appsv1alpha1.ClusterComponentDefinition
	ComponentUnits   []appv1.StatefulSet

	ConfigSpecName   string
	ConfigPatch      *ConfigPatchInfo
	ConfigMap        *corev1.ConfigMap
	ConfigConstraint *appsv1alpha1.ConfigConstraintSpec

	PolicyStatus PolicyExecStatus
}

type ConfigEventHandler interface {
	Handle(eventContext ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.OpsPhase, err error) error
}

const (
	Unconfirmed int32 = -1
	NotStarted  int32 = 0
)

const emptyJSON = "{}"

var (
	loaderProvider        = map[ConfigType]ConfigLoaderProvider{}
	ConfigEventHandlerMap = make(map[string]ConfigEventHandler)
)

func init() {
	// For RAW
	loaderProvider[CfgRawType] = func(option CfgOption) (*cfgWrapper, error) {
		if len(option.RawData) == 0 {
			return nil, MakeError("rawdata not empty! [%v]", option)
		}

		meta := cfgWrapper{
			name:      "raw",
			fileCount: 0,
			v:         make([]unstructured.ConfigObject, 1),
			indexer:   make(map[string]unstructured.ConfigObject, 1),
		}

		v, err := unstructured.LoadConfig(meta.name, string(option.RawData), option.CfgType)
		if err != nil {
			option.Log.Error(err, "failed to parse config!", "context", option.RawData)
			return nil, err
		}

		meta.v[0] = v
		meta.indexer[meta.name] = v
		return &meta, nil
	}

	// For CM/TPL
	loaderProvider[CfgCmType] = func(option CfgOption) (*cfgWrapper, error) {
		if option.ConfigResource == nil {
			return nil, MakeError("invalid k8s resource[%v]", option)
		}

		ctx := option.ConfigResource
		if ctx.ConfigData == nil && ctx.ResourceReader != nil {
			configs, err := ctx.ResourceReader(ctx.CfgKey)
			if err != nil {
				return nil, WrapError(err, "failed to get cm, cm key: [%v]", ctx.CfgKey)
			}
			ctx.ConfigData = configs
		}

		fileCount := len(ctx.ConfigData)
		meta := cfgWrapper{
			name:      path.Base(ctx.CfgKey.Name),
			fileCount: fileCount,
			v:         make([]unstructured.ConfigObject, fileCount),
			indexer:   make(map[string]unstructured.ConfigObject, 1),
		}

		var err error
		var index = 0
		var v unstructured.ConfigObject
		for fileName, content := range ctx.ConfigData {
			if ctx.CMKeys != nil && !ctx.CMKeys.InArray(fileName) {
				continue
			}
			if v, err = unstructured.LoadConfig(fileName, content, option.CfgType); err != nil {
				return nil, WrapError(err, "failed to load config: filename[%s], type[%s]", fileName, option.CfgType)
			}
			meta.indexer[fileName] = v
			meta.v[index] = v
			index++
		}
		return &meta, nil
	}

	// For TPL
	loaderProvider[CfgTplType] = loaderProvider[CfgCmType]
}

type cfgWrapper struct {
	// name is config name
	name string
	// volumeName string

	// fileCount
	fileCount int
	// indexer   map[string]*viper.Viper
	indexer map[string]unstructured.ConfigObject
	v       []unstructured.ConfigObject
}

type dataConfig struct {
	// Option is config for
	Option CfgOption

	// cfgWrapper references configuration template or configmap
	*cfgWrapper
}

func NewConfigLoader(option CfgOption) (*dataConfig, error) {
	loader, ok := loaderProvider[option.Type]
	if !ok {
		return nil, MakeError("not supported config type: %s", option.Type)
	}

	meta, err := loader(option)
	if err != nil {
		return nil, err
	}

	return &dataConfig{
		Option:     option,
		cfgWrapper: meta,
	}, nil
}

// Option for operator
type Option func(ctx *CfgOpOption)

func (c *cfgWrapper) MergeFrom(params map[string]interface{}, option CfgOpOption) error {
	cfg := c.getConfigObject(option)
	if cfg == nil {
		return MakeError("not found the config file:[%s]", option.FileName)
	}

	// TODO support param delete
	for paramKey, paramValue := range params {
		vi := reflect.ValueOf(paramValue)
		if vi.Kind() != reflect.Ptr || !vi.IsNil() {
			if err := cfg.Update(c.generateKey(paramKey, option), paramValue); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *cfgWrapper) ToCfgContent() (map[string]string, error) {
	fileContents := make(map[string]string, c.fileCount)
	for fileName, v := range c.indexer {
		content, err := v.Marshal()
		if err != nil {
			return nil, err
		}
		fileContents[fileName] = content
	}
	return fileContents, nil
}

type ConfigPatchInfo struct {
	IsModify bool
	// new config
	AddConfig map[string]interface{}

	// delete config
	DeleteConfig map[string]interface{}

	// update config
	// patch json
	UpdateConfig map[string][]byte

	Target      *cfgWrapper
	LastVersion *cfgWrapper
}

func NewCfgOptions(filename string, options ...Option) CfgOpOption {
	context := CfgOpOption{
		FileName: filename,
	}

	for _, op := range options {
		op(&context)
	}

	return context
}

func WithFormatterConfig(formatConfig *appsv1alpha1.FormatterConfig) Option {
	return func(ctx *CfgOpOption) {
		if formatConfig.Format == appsv1alpha1.Ini && formatConfig.IniConfig != nil {
			ctx.IniContext = &IniContext{
				SectionName: formatConfig.IniConfig.SectionName,
			}
		}
	}
}

func NestedPrefixField(formatConfig *appsv1alpha1.FormatterConfig) string {
	if formatConfig != nil && formatConfig.Format == appsv1alpha1.Ini && formatConfig.IniConfig != nil {
		return formatConfig.IniConfig.SectionName
	}
	return ""
}

func (c *cfgWrapper) Query(jsonpath string, option CfgOpOption) ([]byte, error) {
	if option.AllSearch && c.fileCount > 1 {
		return c.queryAllCfg(jsonpath, option)
	}

	cfg := c.getConfigObject(option)
	if cfg == nil {
		return nil, MakeError("not found the config file:[%s]", option.FileName)
	}

	iniContext := option.IniContext
	if iniContext != nil && len(iniContext.SectionName) > 0 {
		cfg = cfg.SubConfig(iniContext.SectionName)
		if cfg == nil {
			return nil, MakeError("the section[%s] does not exist in the config file", iniContext.SectionName)
		}
	}

	return util.RetrievalWithJSONPath(cfg.GetAllParameters(), jsonpath)
}

func (c *cfgWrapper) queryAllCfg(jsonpath string, option CfgOpOption) ([]byte, error) {
	tops := make(map[string]interface{}, c.fileCount)

	for filename, v := range c.indexer {
		tops[filename] = v.GetAllParameters()
	}
	return util.RetrievalWithJSONPath(tops, jsonpath)
}

func (c cfgWrapper) getConfigObject(option CfgOpOption) unstructured.ConfigObject {
	if len(c.v) == 0 {
		return nil
	}

	if len(option.FileName) == 0 {
		return c.v[0]
	} else {
		return c.indexer[option.FileName]
	}
}

func (c *cfgWrapper) generateKey(paramKey string, option CfgOpOption) string {
	if option.IniContext != nil && len(option.IniContext.SectionName) > 0 {
		return strings.Join([]string{option.IniContext.SectionName, paramKey}, unstructured.DelimiterDot)
	}

	return paramKey
}

func FromCMKeysSelector(keys []string) *set.LinkedHashSetString {
	var cmKeySet *set.LinkedHashSetString
	if len(keys) > 0 {
		cmKeySet = set.NewLinkedHashSetString(keys...)
	}
	return cmKeySet
}

func GenerateVisualizedParamsList(configPatch *ConfigPatchInfo, formatConfig *appsv1alpha1.FormatterConfig, sets *set.LinkedHashSetString) []VisualizedParam {
	if !configPatch.IsModify {
		return nil
	}

	var trimPrefix = NestedPrefixField(formatConfig)

	r := make([]VisualizedParam, 0)
	r = append(r, generateUpdateParam(configPatch.UpdateConfig, trimPrefix, sets)...)
	r = append(r, generateUpdateKeyParam(configPatch.AddConfig, trimPrefix, AddedType, sets)...)
	r = append(r, generateUpdateKeyParam(configPatch.DeleteConfig, trimPrefix, DeletedType, sets)...)
	return r
}

func generateUpdateParam(updatedParams map[string][]byte, trimPrefix string, sets *set.LinkedHashSetString) []VisualizedParam {
	r := make([]VisualizedParam, 0, len(updatedParams))

	for key, b := range updatedParams {
		// TODO support keys
		if sets != nil && sets.Length() > 0 && !sets.InArray(key) {
			continue
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			return nil
		}
		if params := checkAndFlattenMap(v, trimPrefix); params != nil {
			r = append(r, VisualizedParam{
				Key:        key,
				Parameters: params,
				UpdateType: UpdatedType,
			})
		}
	}
	return r
}

func checkAndFlattenMap(v any, trim string) []ParameterPair {
	m := cast.ToStringMap(v)
	if m != nil && trim != "" {
		m = cast.ToStringMap(m[trim])
	}
	if m != nil {
		return flattenMap(m, "")
	}
	return nil
}

func flattenMap(m map[string]interface{}, prefix string) []ParameterPair {
	if prefix != "" {
		prefix += unstructured.DelimiterDot
	}

	r := make([]ParameterPair, 0)
	for k, val := range m {
		fullKey := prefix + k
		switch m2 := val.(type) {
		case map[string]interface{}:
			r = append(r, flattenMap(m2, fullKey)...)
		default:
			r = append(r, ParameterPair{
				Key:   fullKey,
				Value: cast.ToString(val),
			})
		}
	}
	return r
}

func generateUpdateKeyParam(files map[string]interface{}, trimPrefix string, updatedType ParameterUpdateType, sets *set.LinkedHashSetString) []VisualizedParam {
	r := make([]VisualizedParam, 0, len(files))

	for key, params := range files {
		if sets != nil && sets.Length() > 0 && !sets.InArray(key) {
			continue
		}
		if params := checkAndFlattenMap(params, trimPrefix); params != nil {
			r = append(r, VisualizedParam{
				Key:        key,
				Parameters: params,
				UpdateType: updatedType,
			})
		}
	}
	return r
}

// isQuotesString checks whether a string is quoted.
func isQuotesString(str string) bool {
	const (
		singleQuotes = '\''
		doubleQuotes = '"'
	)

	if len(str) < 2 {
		return false
	}

	firstChar := str[0]
	lastChar := str[len(str)-1]
	return (firstChar == singleQuotes && lastChar == singleQuotes) || (firstChar == doubleQuotes && lastChar == doubleQuotes)
}
