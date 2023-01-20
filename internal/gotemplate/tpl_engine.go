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
)

const (
	DefaultTemplateOps = "missingkey=error"
)

const (
	buildInSystemFailedName = "faield"
	buildInSystemImportName = "import"
	buildInSystemCallName   = "call"
)

type TplValues map[string]interface{}
type BuiltInObjectsFunc map[string]interface{}

type TplEngine struct {
	tpl       *template.Template
	tplValues *TplValues

	cli client.Client
	ctx context.Context
}

func (t *TplEngine) Render(context string) (string, error) {

	var buf strings.Builder
	tpl := template.Must(t.tpl.Parse(context))
	if err := tpl.Execute(&buf, t.tplValues); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (t *TplEngine) initSystemFunMap(funcs template.FuncMap) {
	importModules := set.NewLinkedHashSetString()
	importFuncs := make(map[string]struct {
		namespace string
		name      string
		tpl       string
	})
	funcs[buildInSystemFailedName] = failed
	funcs[buildInSystemImportName] = func(namespacedName string) (string, error) {
		if importModules.InArray(namespacedName) {
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
			if fn, ok := importFuncs[key]; ok {
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
			importFuncs[key] = struct {
				namespace string
				name      string
				tpl       string
			}{namespace: fields[0], name: fields[1], tpl: value}
		}
		return "", nil
	}
	funcs[buildInSystemCallName] = func(funcName string, args ...interface{}) (string, error) {
		fn, ok := importFuncs[funcName]
		if !ok {
			return "", cfgcore.MakeError("not exist func: %s", funcName)
		}

		values := structTplValue(args...)
		engine := NewTplEngine(&values, nil, types.NamespacedName{
			Name:      fn.name,
			Namespace: fn.namespace,
		}.String(), t.cli, t.ctx)
		return engine.Render(fn.tpl)
	}

	t.tpl.Option(DefaultTemplateOps)
	t.tpl.Funcs(funcs)
}

func NewTplEngine(values *TplValues, funcs *BuiltInObjectsFunc, tplName string, cli client.Client, ctx context.Context) *TplEngine {
	coreBuiltinFuncs := sprig.TxtFuncMap()

	// custom funcs
	if funcs != nil && len(*funcs) > 0 {
		for k, v := range *funcs {
			coreBuiltinFuncs[k] = v
		}
	}

	engine := TplEngine{
		tpl:       template.New(tplName),
		tplValues: values,
		ctx:       ctx,
		cli:       cli,
	}

	engine.initSystemFunMap(coreBuiltinFuncs)
	return &engine
}
