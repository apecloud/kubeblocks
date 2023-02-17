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
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	mxjv2 "github.com/clbanning/mxj/v2"
	"github.com/spf13/viper"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// CueType define cue type
// +enum
type CueType string

const (
	NullableType            CueType = "nullable"
	FloatType               CueType = "float"
	IntType                 CueType = "integer"
	BoolType                CueType = "boolean"
	StringType              CueType = "string"
	StructType              CueType = "object"
	ListType                CueType = "array"
	K8SQuantityType         CueType = "quantity"
	ClassicStorageType      CueType = "storage"
	ClassicTimeDurationType CueType = "timeDuration"
)

func init() {
	// disable cast to float
	mxjv2.CastValuesToFloat(false)
	// enable cast to bool
	mxjv2.CastValuesToBool(true)
	// enable cast to int
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

func ValidateConfigurationWithCue(cueTpl string, cfgType appsv1alpha1.ConfigurationFormatter, rawData string) error {
	cfg, err := loadConfiguration(cfgType, rawData)
	if err != nil {
		return WrapError(err, "failed to load configuration. [%s]", rawData)
	}

	return cfgDataValidateByCue(cueTpl, cfg)
}

func loadConfiguration(cfgType appsv1alpha1.ConfigurationFormatter, rawData string) (map[string]interface{}, error) {
	// viper not support xml
	if cfgType == appsv1alpha1.XML {
		return mxjv2.NewMapXml([]byte(rawData), true)
	}

	var v *viper.Viper
	// TODO hack, viper parse problem
	if cfgType == appsv1alpha1.DOTENV {
		v = viper.NewWithOptions(viper.KeyDelimiter("#"))
	} else {
		v = viper.New()
	}
	v.SetConfigType(string(cfgType))
	v.SetTypeByDefaultValue(true)
	if err := v.ReadConfig(strings.NewReader(rawData)); err != nil {
		return nil, err
	}
	return v.AllSettings(), nil
}

func cfgDataValidateByCue(cueTpl string, data interface{}) error {
	defaultValidatePath := "configuration"
	context := cuecontext.New()
	tpl := context.CompileString(cueTpl)
	if err := tpl.Err(); err != nil {
		return err
	}

	if err := processCfgNotStringParam(data, context, tpl); err != nil {
		return err
	}

	var paths []string
	cueValue := tpl.LookupPath(cue.ParsePath(defaultValidatePath))
	if cueValue.Err() == nil {
		paths = []string{defaultValidatePath}
	}

	tpl = tpl.Fill(data, paths...)
	if err := tpl.Err(); err != nil {
		return WrapError(err, "failed to cue template render configure")
	}

	return tpl.Validate()
}
