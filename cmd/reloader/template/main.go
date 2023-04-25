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

	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

const (
	builtinConfigMountPathObject = "configMountPath"
)

var configSpecMountPoint string
var configSpecYaml string

// for rendered output
var outputDir string

func installFlags() {
	pflag.StringVar(&configSpecYaml, "config", "", "specify the config spec to be rendered")
	pflag.StringVar(&configSpecMountPoint, "config-volume", "", "config volume mount point")
	pflag.StringVar(&outputDir, "output-dir", "", "secondary rendered output dir")

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

func main() {
	installFlags()

	files, err := cfgcm.ScanConfigVolume(configSpecMountPoint)
	if err != nil {
		failed(err, "failed to scan config volume")
	}
	baseData, err := cfgutil.FromConfigFiles(files)
	if err != nil {
		failed(err, "failed to create data map")
	}

	configRenderMeta := cfgcm.ConfigRenderMeta{}
	if err := cfgutil.FromYamlConfig(configSpecYaml, &configRenderMeta); err != nil {
		failed(err, "failed to parse config spec")
	}

	mergePolicy, err := plan.NewTemplateMerger(*configRenderMeta.SecondaryRenderedConfigSpec,
		context.TODO(), nil, nil, *configRenderMeta.ComponentConfigSpec)
	if err != nil {
		failed(err, "failed to create template merger")
	}
	engine := gotemplate.NewTplEngine(&gotemplate.TplValues{
		builtinConfigMountPathObject: configSpecMountPoint,
	}, nil,
		fmt.Sprintf("secondary template %s", configRenderMeta.Name), nclient, context.TODO())

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
			tpl = filepath.Join(filepath.Dir(configSpecYaml), tpl)
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
