/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	settings := v.AllSettings()
	path := strings.Split(strings.ToLower(key), cfgKeyDelimiter(v.format))
	parent := settings
	for _, part := range path[:len(path)-1] {
		child, ok := parent[part].(map[string]interface{})
		if !ok {
			return nil
		}
		parent = child
	}
	leaf := path[len(path)-1]
	if _, ok := parent[leaf]; !ok {
		return nil
	}
	delete(parent, leaf)

	updated := newCfgViper(v.format)
	if err := updated.MergeConfigMap(settings); err != nil {
		return err
	}
	v.Viper = updated
	return nil
}

func (v *viperWrap) SubConfig(key string) ConfigObject {
	sub := v.Sub(key)
	if sub == nil {
		// Stay consistent with the other ConfigObject implementations
		// (properties / redis / xml / yaml) which return nil when the
		// requested sub-section is absent. Returning a non-nil wrapper
		// around a nil viper would panic on later AllKeys / AllSettings
		// calls; the existing nil-check in pkg/configuration/core/config.go
		// already expects nil for absent sections.
		return nil
	}
	return &viperWrap{
		Viper:  sub,
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

const emptyJSON = "{}"

func (v *viperWrap) Unmarshal(str string) error {
	if v.format == parametersv1alpha1.JSON && str == "" {
		str = emptyJSON
	}
	return v.ReadConfig(bytes.NewReader([]byte(str)))
}

func newCfgViper(cfgType parametersv1alpha1.CfgFileFormat) *oviper.Viper {
	// TODO config constraint support LoadOptions
	v := oviper.NewWithOptions(oviper.KeyDelimiter(cfgKeyDelimiter(cfgType)), oviper.IniLoadOptions(ini.LoadOptions{
		SpaceBeforeInlineComment: true,
		PreserveSurroundedQuote:  true,
	}))
	v.SetConfigType(strings.ToLower(string(cfgType)))
	return v
}

func cfgKeyDelimiter(cfgType parametersv1alpha1.CfgFileFormat) string {
	if cfgType == parametersv1alpha1.Properties || cfgType == parametersv1alpha1.Dotenv {
		return CfgDelimiterPlaceholder
	}
	return DelimiterDot
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
