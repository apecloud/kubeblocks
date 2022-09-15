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

package controllerutil

import (
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

const (
	DBaaSTpl = "dbaas_tpl"

	DefaultTemplateOps = "missingkey=error"
)

type TplValues map[string]interface{}
type TplFunctions template.FuncMap

type TplEngine struct {
	tpl       *template.Template
	tplValues *TplValues
}

func (t TplEngine) Render(context string) (string, error) {

	var buf strings.Builder
	tpl := template.Must(t.tpl.Parse(context))
	if err := tpl.Execute(&buf, t.tplValues); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func NewTplEngine(values *TplValues, funcs *TplFunctions) *TplEngine {
	coreBuiltinFuncs := sprig.TxtFuncMap()

	// custom funcs
	if funcs != nil && len(*funcs) > 0 {
		for k, v := range *funcs {
			coreBuiltinFuncs[k] = v
		}
	}

	engine := TplEngine{
		tpl:       template.New(DBaaSTpl),
		tplValues: values,
	}

	engine.tpl.Option(DefaultTemplateOps)
	engine.tpl.Funcs(coreBuiltinFuncs)
	return &engine
}
