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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/pflag"
	corezap "go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cfgcontainer "github.com/apecloud/kubeblocks/internal/configuration/container"

	"github.com/apecloud/kubeblocks/cmd/tpl/app"
)

var clusterYaml string
var clusterDefYaml string

var outputDir string
var clearOutputDir bool
var helmOutputDir string
var helmTemplateDir string

var opts app.RenderedOptions

func installFlags() {
	pflag.StringVar(&clusterYaml, "cluster", "", "the cluster yaml file")
	pflag.StringVar(&clusterDefYaml, "cluster-definition", "", "the cluster definition yaml file")

	pflag.StringVarP(&outputDir, "output-dir", "o", "", "specify the output directory")

	pflag.StringVar(&opts.ConfigSpec, "config-spec", "", "specify the config spec to be rendered")
	pflag.BoolVarP(&opts.AllConfigSpecs, "all", "a", false, "template all config specs")

	// mock cluster object
	pflag.Int32VarP(&opts.Replicas, "replicas", "r", 1, "specify the replicas of the component")
	pflag.StringVar(&opts.DataVolumeName, "volume-name", "", "specify the data volume name of the component")
	pflag.StringVar(&opts.ComponentName, "component-name", "", "specify the component name of the clusterdefinition")
	pflag.StringVar(&helmTemplateDir, "helm", "", "specify the helm template dir")
	pflag.StringVar(&helmOutputDir, "helm-output", "", "specify the helm template output dir")
	pflag.StringVar(&opts.CPU, "cpu", "", "specify the cpu of the component")
	pflag.StringVar(&opts.Memory, "memory", "", "specify the memory of the component")
	pflag.BoolVar(&clearOutputDir, "clean", false, "specify whether to clear the output dir")

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

func main() {

	opts = app.RenderedOptions{
		// for mock cluster object
		Namespace: "default",
		Name:      "cluster-" + app.RandomString(6),
	}
	installFlags()

	if err := checkAndHelmTemplate(); err != nil {
		fmt.Printf("failed to exec helm template: %v", err)
		os.Exit(-1)
	}

	if helmOutputDir == "" {
		fmt.Printf("helm template dir is empty")
		os.Exit(-1)
	}

	workflow, err := app.NewWorkflowTemplateRender(helmOutputDir, opts)
	if err != nil {
		fmt.Printf("failed to create workflow: %v", err)
		os.Exit(-1)
	}

	if clearOutputDir && outputDir != "" {
		_ = os.RemoveAll(outputDir)
	}
	if outputDir == "" {
		outputDir = filepath.Join("./output", app.RandomString(6))
	}

	if err := workflow.Do(outputDir); err != nil {
		fmt.Printf("failed to render workflow: %v", err)
		os.Exit(-1)
	}
}

func checkAndHelmTemplate() error {
	if helmTemplateDir != "" || helmOutputDir == "" {
		helmOutputDir = filepath.Join("./helm-output", app.RandomString(6))
	}

	if helmTemplateDir == "" || helmOutputDir == "" {
		return nil
	}
	cmd := exec.Command("helm", "template", helmTemplateDir, "--output-dir", helmOutputDir)
	stdout, err := cfgcontainer.ExecShellCommand(cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	fmt.Println(stdout)
	return nil
}
