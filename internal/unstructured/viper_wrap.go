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

package unstructured

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cast"
	oviper "github.com/spf13/viper"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type viperWrap struct {
	*oviper.Viper

	name   string
	format appsv1alpha1.CfgFileFormat
}

func init() {
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.Ini, createViper(appsv1alpha1.Ini))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.YAML, createViper(appsv1alpha1.YAML))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.JSON, createViper(appsv1alpha1.JSON))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.Dotenv, createViper(appsv1alpha1.Dotenv))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.HCL, createViper(appsv1alpha1.HCL))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.TOML, createViper(appsv1alpha1.TOML))
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.Properties, createViper(appsv1alpha1.Properties))
}

func (v *viperWrap) GetString(key string) (string, error) {
	return cast.ToStringE(v.Get(key))
}

func (v *viperWrap) GetAllParameters() map[string]interface{} {
	return v.AllSettings()
}

func (v *viperWrap) SubConfig(key string) ConfigObject {
	return &viperWrap{
		Viper:  v.Sub(key),
		format: v.format,
	}
}

func (v *viperWrap) Update(key string, value any) error {
	v.Set(key, value)
	return nil
}

func (v *viperWrap) Marshal() (string, error) {
	const tmpFileName = "_config_tmp"

	tmpDir, err := os.MkdirTemp(os.TempDir(), "configuration-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	cfgName := v.name
	if cfgName == "" {
		cfgName = tmpFileName
	}
	tmpFile := filepath.Join(tmpDir, strings.ReplaceAll(cfgName, ".", "_"))
	return dumpCfgContent(v.Viper, tmpFile)
}

func (v viperWrap) Unmarshal(str string) error {
	return v.ReadConfig(bytes.NewReader([]byte(str)))
}

func newCfgViper(cfgType appsv1alpha1.CfgFileFormat) *oviper.Viper {
	defaultKeySep := DelimiterDot
	if cfgType == appsv1alpha1.Properties || cfgType == appsv1alpha1.Dotenv {
		defaultKeySep = CfgDelimiterPlaceholder
	}
	v := oviper.NewWithOptions(oviper.KeyDelimiter(defaultKeySep))
	v.SetConfigType(strings.ToLower(string(cfgType)))
	return v
}

func createViper(format appsv1alpha1.CfgFileFormat) ConfigObjectCreator {
	return func(name string) ConfigObject {
		return &viperWrap{
			name:   name,
			format: format,
			Viper:  newCfgViper(format),
		}
	}
}
