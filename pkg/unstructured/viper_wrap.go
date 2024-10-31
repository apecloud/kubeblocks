/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package unstructured

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cast"
	oviper "github.com/spf13/viper"
	"gopkg.in/ini.v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

type viperWrap struct {
	*oviper.Viper

	name   string
	format parametersv1alpha1.CfgFileFormat
}

func init() {
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.Ini, createViper(parametersv1alpha1.Ini))
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.JSON, createViper(parametersv1alpha1.JSON))
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.Dotenv, createViper(parametersv1alpha1.Dotenv))
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.HCL, createViper(parametersv1alpha1.HCL))
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.TOML, createViper(parametersv1alpha1.TOML))
	CfgObjectRegistry().RegisterConfigCreator(parametersv1alpha1.Properties, createViper(parametersv1alpha1.Properties))
}

func (v *viperWrap) GetString(key string) (string, error) {
	return cast.ToStringE(v.Get(key))
}

func (v *viperWrap) GetAllParameters() map[string]interface{} {
	return v.AllSettings()
}

func (v *viperWrap) RemoveKey(key string) error {
	// TODO viper does not support remove key
	return nil
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

func newCfgViper(cfgType parametersv1alpha1.CfgFileFormat) *oviper.Viper {
	defaultKeySep := DelimiterDot
	if cfgType == parametersv1alpha1.Properties || cfgType == parametersv1alpha1.Dotenv {
		defaultKeySep = CfgDelimiterPlaceholder
	}
	// TODO config constraint support LoadOptions
	v := oviper.NewWithOptions(oviper.KeyDelimiter(defaultKeySep), oviper.IniLoadOptions(ini.LoadOptions{
		SpaceBeforeInlineComment: true,
		PreserveSurroundedQuote:  true,
	}))
	v.SetConfigType(strings.ToLower(string(cfgType)))
	return v
}

func createViper(format parametersv1alpha1.CfgFileFormat) ConfigObjectCreator {
	return func(name string) ConfigObject {
		return &viperWrap{
			name:   name,
			format: format,
			Viper:  newCfgViper(format),
		}
	}
}
