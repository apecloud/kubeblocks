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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	corezap "go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
)

const (
	builtinConfigMountPathObject = "ConfigMountPath"
)

var configSpecMountPoint string
var lazyRenderedConfig string

// for rendered output
var outputDir string
var setParams []string

func installFlags() {
	pflag.StringVar(&lazyRenderedConfig, "config", "", "specify the config spec to be rendered")
	pflag.StringVar(&configSpecMountPoint, "config-volume", "", "config volume mount point")
	pflag.StringVar(&outputDir, "output-dir", "", "secondary rendered output dir")
	pflag.StringSliceVar(&setParams, "set", nil, "set parameter")

	opts := zap.Options{
		Development: true,
		Level: func() *corezap.AtomicLevel {
			lvl := corezap.NewAtomicLevelAt(corezap.InfoLevel)
			return &lvl
		}(),
	}

	opts.BindFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
}

func failed(err error, msg string) {
	ctrl.Log.Error(err, msg)
	os.Exit(-1)
}

func buildTplValues() *gotemplate.TplValues {
	values := gotemplate.TplValues{}
	for _, param := range setParams {
		fields := strings.SplitN(param, "=", 2)
		if len(fields) == 2 {
			values[fields[0]] = fields[1]
		} else if len(fields) == 1 {
			values[fields[0]] = nil
		}
	}
	values[builtinConfigMountPathObject] = configSpecMountPoint
	return &values
}

func main() {
	installFlags()

	if configSpecMountPoint == "" {
		failed(cfgcore.MakeError("config volume mount point is empty"), "")
	}

	if lazyRenderedConfig == "" {
		failed(cfgcore.MakeError("config spec yaml is empty"), "")
	}

	if outputDir == "" {
		failed(cfgcore.MakeError("output dir is empty"), "")
	}

	files, err := cfgcm.ScanConfigVolume(configSpecMountPoint)
	if err != nil {
		failed(err, "failed to scan config volume")
	}
	baseData, err := cfgutil.FromConfigFiles(files)
	if err != nil {
		failed(err, "failed to create data map")
	}

	configRenderMeta := cfgcm.ConfigLazyRenderedMeta{}
	if err := cfgutil.FromYamlConfig(filepath.Join(lazyRenderedConfig, cfgcm.KBConfigSpecLazyRenderedYamlFile), &configRenderMeta); err != nil {
		failed(err, "failed to parse config spec")
	}

	mergePolicy, err := configuration.NewTemplateMerger(configRenderMeta.LegacyRenderedConfigSpec.ConfigTemplateExtension,
		context.TODO(), nil, nil, *configRenderMeta.ComponentConfigSpec, &appsv1alpha1.ConfigConstraintSpec{
			FormatterConfig: &configRenderMeta.FormatterConfig,
		})
	if err != nil {
		failed(err, "failed to create template merger")
	}

	engine := gotemplate.NewTplEngine(buildTplValues(), nil, fmt.Sprintf("secondary template %s", configRenderMeta.Name), nclient, context.TODO())

	renderedData, err := secondaryRender(engine, configRenderMeta.Templates)
	if err != nil {
		failed(err, "failed to render secondary templates")
	}

	mergedData, err := mergePolicy.Merge(baseData, renderedData)
	if err != nil {
		failed(err, "failed to merge data")
	}

	if err := dumpRenderedData(mergedData); err != nil {
		failed(err, "failed to dump rendered data")
	}
}

func dumpRenderedData(data map[string]string) error {
	exist, err := cfgutil.CheckPathExists(outputDir)
	if err != nil {
		return err
	}
	if !exist {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
	}
	for fileName, fileContext := range data {
		if err := os.WriteFile(filepath.Join(outputDir, fileName), []byte(fileContext), 0644); err != nil {
			return err
		}
	}
	return nil
}

func secondaryRender(engine *gotemplate.TplEngine, templates []string) (map[string]string, error) {
	renderedData := make(map[string]string, len(templates))
	for _, tpl := range templates {
		tpl = strings.TrimSpace(tpl)
		if tpl == "" {
			continue
		}
		if !filepath.IsAbs(tpl) {
			tpl = filepath.Join(lazyRenderedConfig, tpl)
		}

		b, err := os.ReadFile(tpl)
		if err != nil {
			return nil, err
		}
		rendered, err := engine.Render(string(b))
		if err != nil {
			return nil, err
		}
		renderedData[filepath.Base(tpl)] = rendered
	}
	return renderedData, nil
}
