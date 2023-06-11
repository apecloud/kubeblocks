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

package gotemplate

import (
	"context"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
)

const (
	DefaultTemplateOps = "missingkey=error"
)

const (
	buildInSystemFailedName = "failed"
	buildInSystemImportName = "import"
	buildInSystemCallName   = "call"
)

const (
	goTemplateExtendBuildInRegexSubString      = "regexStringSubmatch"
	goTemplateExtendBuildInFromYamlString      = "fromYaml"
	goTemplateExtendBuildInFromYamlArrayString = "fromYamlArray"
)

type TplValues map[string]interface{}
type BuiltInObjectsFunc map[string]interface{}

type functional struct {
	// cm Namespace
	namespace string
	// cm Name
	name string
	// go template context
	tpl string
}

type TplEngine struct {
	tpl       *template.Template
	tplValues *TplValues

	importModules *set.LinkedHashSetString
	importFuncs   map[string]functional

	cli types2.ReadonlyClient
	ctx context.Context
}

func (t *TplEngine) GetTplEngine() *template.Template {
	return t.tpl
}

func (t *TplEngine) Render(context string) (string, error) {
	var buf strings.Builder
	tpl, err := t.tpl.Parse(context)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, t.tplValues); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (t *TplEngine) initSystemFunMap(funcs template.FuncMap) {
	// When an error occurs, go template engine can detect it and exit early.
	funcs[buildInSystemFailedName] = failed

	// With 'import', you can make a function in configmap visible in this scope.
	funcs[buildInSystemImportName] = func(namespacedName string) (string, error) {
		if t.importModules.InArray(namespacedName) {
			return "", nil
		}
		fields := strings.SplitN(namespacedName, ".", 2)
		if len(fields) != 2 {
			return "", cfgcore.MakeError("invalid import namespaceName: %s", namespacedName)
		}

		cm := &corev1.ConfigMap{}
		if err := t.cli.Get(t.ctx, client.ObjectKey{
			Namespace: fields[0],
			Name:      fields[1],
		}, cm); err != nil {
			return "", err
		}

		if !isSystemFuncsCM(cm) {
			return "", cfgcore.MakeError("cm: %v is not template functions.", client.ObjectKeyFromObject(cm))
		}
		for key, value := range cm.Data {
			if fn, ok := t.importFuncs[key]; ok {
				return "", cfgcore.MakeError("failed to import function: %s, from %v, function is already imported: %v",
					key, client.ObjectKey{
						Namespace: fields[0],
						Name:      fields[1],
					},
					client.ObjectKey{
						Namespace: fn.namespace,
						Name:      fn.name,
					})
			}
			t.importFuncs[key] = functional{namespace: fields[0], name: fields[1], tpl: value}
		}
		return "", nil
	}

	// Function 'call' simulates a function call in a programming language.
	// The parameter list of function is similar to function in bash language.
	// The access parameter uses .arg or $.arg.
	// e.g: $.arg0, $.arg1, ...
	//
	// Note: How to handle the return type of go template functions?
	// It is recommended that serialize it to string and cast it to a specific type in the calling function.
	// e.g :
	// The return type of the function is map, the return value is $sts,
	// {{- $sts | toJson }}
	// for calling function:
	// {{- $sts := call "test" 10 | fromJson }}
	funcs[buildInSystemCallName] = func(funcName string, args ...interface{}) (string, error) {
		fn, ok := t.importFuncs[funcName]
		if !ok {
			return "", cfgcore.MakeError("not existed func: %s", funcName)
		}

		values := ConstructFunctionArgList(args...)
		engine := NewTplEngine(&values, nil, types.NamespacedName{
			Name:      fn.name,
			Namespace: fn.namespace,
		}.String(), t.cli, t.ctx)

		engine.importSelfModuleFuncs(t.importFuncs, func(tpl functional) bool {
			return tpl.namespace == fn.namespace && tpl.name == fn.name
		})
		return engine.Render(fn.tpl)
	}

	// Wrap regex.FindStringSubmatch
	funcs[goTemplateExtendBuildInRegexSubString] = regexStringSubmatch
	funcs[goTemplateExtendBuildInFromYamlString] = fromYAML
	funcs[goTemplateExtendBuildInFromYamlArrayString] = fromYAMLArray

	t.tpl.Option(DefaultTemplateOps)
	t.tpl.Funcs(funcs)
}

func (t *TplEngine) importSelfModuleFuncs(funcs map[string]functional, fn func(tpl functional) bool) {
	for fnName, tpl := range funcs {
		if fn(tpl) {
			t.importFuncs[fnName] = tpl
		}
	}
}

// NewTplEngine creates go template helper
func NewTplEngine(values *TplValues, funcs *BuiltInObjectsFunc, tplName string, cli types2.ReadonlyClient, ctx context.Context) *TplEngine {
	coreBuiltinFuncs := sprig.TxtFuncMap()

	// custom funcs
	if funcs != nil && len(*funcs) > 0 {
		for k, v := range *funcs {
			coreBuiltinFuncs[k] = v
		}
	}

	engine := TplEngine{
		tpl:           template.New(tplName),
		tplValues:     values,
		ctx:           ctx,
		cli:           cli,
		importModules: set.NewLinkedHashSetString(),
		importFuncs:   make(map[string]functional),
	}

	engine.initSystemFunMap(coreBuiltinFuncs)
	return &engine
}
