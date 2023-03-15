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
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/StudioSol/set"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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

	ClusterComponent      *appsv1alpha1.ClusterComponentSpec
	Component             *appsv1alpha1.ClusterComponentDefinition
	IndepRoledStatefulSet []appv1.StatefulSet

	TplName          string
	ConfigPatch      *ConfigPatchInfo
	CfgCM            *corev1.ConfigMap
	ConfigConstraint *appsv1alpha1.ConfigConstraintSpec

	PolicyStatus PolicyExecStatus
}

type ConfigEventHandler interface {
	Handle(eventContext ConfigEventContext, lastOpsRequest string, phase appsv1alpha1.Phase, err error) error
}

const (
	Unconfirmed int32 = -1
	NotStarted  int32 = 0
)

const (
	delimiterDot = "."
	emptyJSON    = "{}"

	// In order to verify a configuration file, the configuration file is converted to a UnstructuredObject.
	// When there is a special character '.' in the parameter will cause the parameter of the configuration file parsing to be messed up.
	//   e.g. pg parameters: auto_explain.log_analyze = 'True'
	// To solve this problem, the cfgDelimiterPlaceholder variable is introduced to ensure that no such string exists in a configuration file.
	cfgDelimiterPlaceholder = "@#@"
)

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
			v:         make([]*viper.Viper, 1),
			indexer:   make(map[string]*viper.Viper, 1),
		}

		v := NewCfgViper(option.CfgType)
		if err := v.ReadConfig(bytes.NewReader(option.RawData)); err != nil {
			option.Log.Error(err, "failed to parse config!", "context", option.RawData)
			return nil, err
		}
		meta.v[0] = v
		meta.indexer[meta.name] = v
		return &meta, nil
	}

	// For CM/TPL
	loaderProvider[CfgCmType] = func(option CfgOption) (*cfgWrapper, error) {
		if option.K8sKey == nil {
			return nil, MakeError("invalid k8s resource[%v]", option)
		}

		ctx := option.K8sKey
		if ctx.Configurations == nil && ctx.ResourceFn != nil {
			configs, err := ctx.ResourceFn(ctx.CfgKey)
			if err != nil {
				return nil, WrapError(err, "failed to get cm, cm key: [%v]", ctx.CfgKey)
			}
			ctx.Configurations = configs
		}

		fileCount := len(ctx.Configurations)
		meta := cfgWrapper{
			name:      path.Base(ctx.CfgKey.Name),
			fileCount: fileCount,
			v:         make([]*viper.Viper, fileCount),
			indexer:   make(map[string]*viper.Viper, 1),
		}

		var index = 0
		for fileName, content := range ctx.Configurations {
			if ctx.CMKeys != nil && !ctx.CMKeys.InArray(fileName) {
				continue
			}
			v := NewCfgViper(option.CfgType)
			if err := v.ReadConfig(bytes.NewReader([]byte(content))); err != nil {
				return nil, WrapError(err, "failed to load config: filename[%s]", fileName)
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
	indexer   map[string]*viper.Viper
	v         []*viper.Viper
}

type dataConfig struct {
	// Option is config for
	Option CfgOption

	// cfgWrapper reference configuration template or configmap
	*cfgWrapper
}

func NewConfigLoader(option CfgOption) (*dataConfig, error) {
	loader, ok := loaderProvider[option.Type]
	if !ok {
		return nil, MakeError("not support config type: %s", option.Type)
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
	cfg := c.getCfgViper(option)
	if cfg == nil {
		return MakeError("not any configuration. option:[%v]", option)
	}

	// TODO support param delete
	for paramKey, paramValue := range params {
		vi := reflect.ValueOf(paramValue)
		if vi.Kind() != reflect.Ptr || !vi.IsNil() {
			cfg.Set(c.generateKey(paramKey, option, cfg), paramValue)
		}
	}

	return nil
}

func (c *cfgWrapper) ToCfgContent() (map[string]string, error) {
	fileContents := make(map[string]string, c.fileCount)

	// Viper not support writer to buffer
	tmpDir, err := os.MkdirTemp(os.TempDir(), "configuration-")
	if err != nil {
		return nil, WrapError(err, "failed to create temp directory!")
	}
	defer os.RemoveAll(tmpDir)

	for fileName, v := range c.indexer {
		tmpFile := filepath.Join(tmpDir, strings.ReplaceAll(fileName, ".", "_"))
		content, err := DumpCfgContent(v, tmpFile)
		if err != nil {
			return nil, WrapError(err,
				"failed to generate config file[%s], meta: [%v]", fileName, v)
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

func (c *cfgWrapper) Diff(target *cfgWrapper) (*ConfigPatchInfo, error) {
	fromOMap := ToSet(c.indexer)
	fromNMap := ToSet(target.indexer)

	addSet := Difference(fromNMap, fromOMap)
	deleteSet := Difference(fromOMap, fromNMap)
	updateSet := Difference(fromOMap, deleteSet)

	reconfigureInfo := &ConfigPatchInfo{
		IsModify:     false,
		AddConfig:    make(map[string]interface{}, addSet.Length()),
		DeleteConfig: make(map[string]interface{}, deleteSet.Length()),
		UpdateConfig: make(map[string][]byte, updateSet.Length()),

		Target:      target,
		LastVersion: c,
	}

	for elem := range addSet.Iter() {
		reconfigureInfo.AddConfig[elem] = target.indexer[elem].AllSettings()
		reconfigureInfo.IsModify = true
	}

	for elem := range deleteSet.Iter() {
		reconfigureInfo.DeleteConfig[elem] = c.indexer[elem].AllSettings()
		reconfigureInfo.IsModify = true
	}

	for elem := range updateSet.Iter() {
		old := c.indexer[elem]
		new := target.indexer[elem]

		patch, err := jsonPatch(old.AllSettings(), new.AllSettings())
		if err != nil {
			return nil, err
		}
		if len(patch) > len(emptyJSON) {
			reconfigureInfo.UpdateConfig[elem] = patch
			reconfigureInfo.IsModify = true
		}
	}

	return reconfigureInfo, nil
}

func NewCfgViper(cfgType appsv1alpha1.CfgFileFormat) *viper.Viper {
	defaultKeySep := delimiterDot
	if cfgType == appsv1alpha1.Properties || cfgType == appsv1alpha1.Dotenv {
		defaultKeySep = cfgDelimiterPlaceholder
	}
	v := viper.NewWithOptions(viper.KeyDelimiter(defaultKeySep))
	v.SetConfigType(strings.ToLower(string(cfgType)))
	return v
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

func (c *cfgWrapper) Query(jsonpath string, option CfgOpOption) ([]byte, error) {
	if option.AllSearch && c.fileCount > 1 {
		return c.queryAllCfg(jsonpath, option)
	}

	cfg := c.getCfgViper(option)
	if cfg == nil {
		return nil, MakeError("not any configuration. option:[%v]", option)
	}

	iniContext := option.IniContext
	if iniContext != nil && len(iniContext.SectionName) > 0 {
		if !cfg.InConfig(iniContext.SectionName) {
			return nil, MakeError("configuration not exist section [%s]", iniContext.SectionName)
		}
		cfg = cfg.Sub(iniContext.SectionName)
	}

	// var jsonString interface{}
	// if err := cfg.Unmarshal(&jsonString); err != nil {
	//	 return nil, WrapError(err, "failed to unmarshalled configure! [%v]", cfg)
	// }
	return retrievalWithJSONPath(cfg.AllSettings(), jsonpath)
}

func (c *cfgWrapper) queryAllCfg(jsonpath string, option CfgOpOption) ([]byte, error) {
	tops := make(map[string]interface{}, c.fileCount)

	for filename, v := range c.indexer {
		tops[filename] = v.AllSettings()
	}
	return retrievalWithJSONPath(tops, jsonpath)
}

func (c cfgWrapper) getCfgViper(option CfgOpOption) *viper.Viper {
	if len(c.v) == 0 {
		return nil
	}

	if len(option.FileName) == 0 {
		return c.v[0]
	} else {
		return c.indexer[option.FileName]
	}
}

func (c *cfgWrapper) generateKey(paramKey string, option CfgOpOption, v *viper.Viper) string {
	if option.IniContext != nil && len(option.IniContext.SectionName) > 0 {
		return strings.Join([]string{option.IniContext.SectionName, paramKey}, delimiterDot)
	}

	return paramKey
}

func DumpCfgContent(v *viper.Viper, tmpPath string) (string, error) {
	dirPath := filepath.Dir(tmpPath)
	v.AddConfigPath(dirPath)
	if err := v.WriteConfigAs(tmpPath); err != nil {
		return "", err
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func FromCMKeysSelector(keys []string) *set.LinkedHashSetString {
	var cmKeySet *set.LinkedHashSetString
	if len(keys) > 0 {
		cmKeySet = set.NewLinkedHashSetString(keys...)
	}
	return cmKeySet
}

func CreateMergePatch(oldcfg, target interface{}, option CfgOption) (*ConfigPatchInfo, error) {

	ok, err := compareWithConfig(oldcfg, target, option)
	if err != nil {
		return nil, err
	} else if ok {
		return &ConfigPatchInfo{
			IsModify: false,
		}, err
	}

	old, err := NewConfigLoader(withOption(option, oldcfg))
	if err != nil {
		return nil, WrapError(err, "failed to create config: [%s]", oldcfg)
	}

	new, err := NewConfigLoader(withOption(option, target))
	if err != nil {
		return nil, WrapError(err, "failed to create config: [%s]", oldcfg)
	}

	return old.Diff(new.cfgWrapper)
}

func GenerateVisualizedParamsList(configPatch *ConfigPatchInfo, formatConfig *appsv1alpha1.FormatterConfig, sets *set.LinkedHashSetString) []VisualizedParam {
	if !configPatch.IsModify {
		return nil
	}

	var trimPrefix = ""
	if formatConfig != nil && formatConfig.Format == appsv1alpha1.Ini && formatConfig.IniConfig != nil {
		trimPrefix = formatConfig.IniConfig.SectionName
	}

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
		prefix += delimiterDot
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

// isQuotesString check whether a string is quoted.
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
