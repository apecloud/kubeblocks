/*
Copyright 2022.

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
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	mxjv2 "github.com/clbanning/mxj/v2"
	"github.com/spf13/viper"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func init() {
	mxjv2.CastValuesToInt(true)
}

// CueValidate cue validate
func CueValidate(cueTpl string) error {
	if len(cueTpl) == 0 {
		return nil
	}

	context := cuecontext.New()
	tpl := context.CompileString(cueTpl)
	return tpl.Validate()
}

func ValidateConfigurationWithCue(cueTpl string, cfgType dbaasv1alpha1.ConfigurationFormatter, rawData string) error {
	cfg := LoadConfiguration(cfgType, rawData)
	if cfg == nil {
		return MakeError("failed to load configuration. [%s]", rawData)
	}

	return CfgDataValidateByCue(cueTpl, cfg)
}

func LoadConfiguration(cfgType dbaasv1alpha1.ConfigurationFormatter, rawData string) interface{} {
	// viper not support xml
	if cfgType == dbaasv1alpha1.XML {
		xmlMap, err := mxjv2.NewMapXml([]byte(rawData), true)
		if err != nil {
			return err
		}
		return xmlMap
	}
	v := viper.New()
	v.SetConfigType(string(cfgType))
	if err := v.ReadConfig(strings.NewReader(rawData)); err != nil {
		return nil
	}

	return v.AllSettings()
}

func CfgDataValidateByCue(cueTpl string, config interface{}) error {
	const (
		ConfigurationCueVariableName = "configuration"
	)

	// yamlData, err := json.Marshal(config)
	// if err != nil {
	//	return false, err
	// }

	context := cuecontext.New()
	tpl := context.CompileString(cueTpl)
	if err := tpl.Err(); err != nil {
		return err
	}

	cfgPath := cue.ParsePath(ConfigurationCueVariableName)
	tpl = tpl.FillPath(cfgPath, config)
	if err := tpl.Err(); err != nil {
		return WrapError(err, "failed to cue template render configure")
	}

	cfg := tpl.LookupPath(cfgPath)
	// debug, _ := cfg.MarshalJSON()
	// fmt.Println(debug)
	return cfg.Validate()
}
