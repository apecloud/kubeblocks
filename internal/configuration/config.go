/*
Copyright ApeCloud Inc.

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
	"reflect"

	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConfigLoaderProvider func(option CfgOption) (*cfgWrapper, error)

const (
	cfgKeyDelimiter = "."
	emptyJSON       = "{}"
)

type ConfigEventContext struct {
	Client  client.Client
	ReqCtx  intctrlutil.RequestCtx
	Cluster *dbaasv1alpha1.Cluster

	ClusterComponent *dbaasv1alpha1.ClusterComponent
	Component        *dbaasv1alpha1.ClusterDefinitionComponent
	ComponentUnits   []appv1.StatefulSet

	Meta *ConfigDiffInformation
	Cfg  *corev1.ConfigMap
	Tpl  *dbaasv1alpha1.ConfigurationTemplateSpec
}

type ConfigEventHandler interface {
	Handle(eventContext ConfigEventContext, lastOpsRequest string, phase dbaasv1alpha1.Phase, err error) error
}

var (
	loaderProvider        = map[ConfigType]ConfigLoaderProvider{}
	configEventHandlerMap = make(map[string]ConfigEventHandler)
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

		v := viper.NewWithOptions(viper.KeyDelimiter(cfgKeyDelimiter))

		v.SetConfigType(string(option.CfgType))
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
			v := viper.NewWithOptions(viper.KeyDelimiter(cfgKeyDelimiter))
			v.SetConfigType(string(option.CfgType))
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

type ConfigDiffInformation struct {
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

func (c *cfgWrapper) Diff(target *cfgWrapper) (*ConfigDiffInformation, error) {
	fromOMap := NewSetFromMap(c.indexer)
	fromNMap := NewSetFromMap(target.indexer)

	addSet := Difference(fromNMap, fromOMap)
	deleteSet := Difference(fromOMap, fromNMap)
	updateSet := Difference(fromOMap, deleteSet)

	reconfigureInfo := &ConfigDiffInformation{
		IsModify:     false,
		AddConfig:    make(map[string]interface{}, addSet.Size()),
		DeleteConfig: make(map[string]interface{}, deleteSet.Size()),
		UpdateConfig: make(map[string][]byte, updateSet.Size()),

		Target:      target,
		LastVersion: c,
	}

	for elem := range *addSet {
		reconfigureInfo.AddConfig[elem] = target.indexer[elem].AllSettings()
		reconfigureInfo.IsModify = true
	}

	for elem := range *deleteSet {
		reconfigureInfo.DeleteConfig[elem] = c.indexer[elem].AllSettings()
		reconfigureInfo.IsModify = true
	}

	for elem := range *updateSet {
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
		return strings.Join([]string{option.IniContext.SectionName, paramKey}, cfgKeyDelimiter)
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

func CreateMergePatch(oldcfg, target interface{}, option CfgOption) (*ConfigDiffInformation, error) {

	ok, err := compareWithConfig(oldcfg, target, option)
	if err != nil {
		return nil, err
	} else if ok {
		return &ConfigDiffInformation{
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

func RegisterConfigEventHandler(name string, handler ConfigEventHandler) {
	configEventHandlerMap[name] = handler
}

func GetConfigEventHandlers() map[string]ConfigEventHandler {
	return configEventHandlerMap
}
