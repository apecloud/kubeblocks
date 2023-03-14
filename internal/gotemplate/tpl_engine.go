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

package gotemplate

import (
	"context"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/StudioSol/set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
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
	goTemplateExtendBuildInRegexSubString = "regexStringSubmatch"
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
	// Add the 'failed' function here.
	// When an error occurs, go template engine can detect it and exit early.
	funcs[buildInSystemFailedName] = failed

	// Add the 'import' function here.
	// Using it, you can make any function in a configmap object visible in this scope.
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
				return "", cfgcore.MakeError("failed to import function: %s, from %v, function is ready import: %v",
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

	// Add the 'call' function here.
	// This function simulates a function call in a programming language.
	// The parameter list of function is similar to function in bash language.
	// The access parameter uses .arg or $.arg.
	// e.g: $.arg0, $.arg1, ...
	//
	// Node: How to do handle the return type of go template functions?
	// It is recommended that you serialize it to string and cast it to a specific type in the calling function.
	// e.g :
	// The return type of the function is map, the return value is $sts,
	// {{- $sts | toJson }}
	// for calling function:
	// {{- $sts := call "test" 10 | fromJson }}
	funcs[buildInSystemCallName] = func(funcName string, args ...interface{}) (string, error) {
		fn, ok := t.importFuncs[funcName]
		if !ok {
			return "", cfgcore.MakeError("not exist func: %s", funcName)
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

// NewTplEngine create go template helper
// To support this caller has a concept of import dependency which is recursive.
//
// As it recurses, it also sets the values to be appropriate for the parameters of the called function,
// it looks like it's calling a local function.
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
