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

package tools

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/builder/template"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type migrateOptions struct {
	genericiooptions.IOStreams

	Factory cmdutil.Factory
	// dynamic dynamic.Interface

	helmTemplateDir   string
	scriptsOutputPath string
	regex             string
	cmName            string
	overwrite         bool
}

func (o *migrateOptions) complete() error {
	if o.helmTemplateDir == "" {
		return cfgcore.MakeError("helm template dir is required")
	}
	if ok, _ := cfgutil.CheckPathExists(o.helmTemplateDir); !ok {
		return cfgcore.MakeError("helm template dir is not exists")
	}
	if o.regex == "" && o.cmName == "" {
		return cfgcore.MakeError("regex or cm name are required")
	}

	if o.scriptsOutputPath == "" {
		return cfgcore.MakeError("scripts output path is required")
	}

	ok, err := cfgutil.CheckPathExists(o.scriptsOutputPath)
	if err != nil {
		return err
	}
	if !ok {
		err = os.MkdirAll(o.scriptsOutputPath, os.ModePerm)
	}
	return err
}

func (o *migrateOptions) run() error {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	output := filepath.Join(tmpDir, "helm-output")
	if err := template.HelmTemplate(o.helmTemplateDir, output); err != nil {
		return err
	}

	allObjects, err := template.CreateObjectsFromDirectory(output)
	if err != nil {
		return err
	}

	if o.cmName != "" {
		return o.processSpecConfigMap(allObjects)
	}
	_ = template.GetTypedResourceObjectBySignature(allObjects, generics.ConfigMapSignature, o.processConfigMap)
	return nil
}

func (o *migrateOptions) processSpecConfigMap(objects []client.Object) error {
	cm := template.GetTypedResourceObjectBySignature(objects, generics.ConfigMapSignature, template.WithResourceName(o.cmName))
	if cm == nil {
		return cfgcore.MakeError("configmap %s not found", o.cmName)
	}
	dumpHelmScripts(cm.Data, o.scriptsOutputPath, o.Out, o.overwrite)
	return nil
}

func (o *migrateOptions) processConfigMap(object client.Object) (r bool) {
	r = false
	cm, ok := object.(*corev1.ConfigMap)
	if !ok {
		return
	}
	if strings.Contains(cm.Name, "script") {
		dumpHelmScripts(cm.Data, o.scriptsOutputPath, o.Out, o.overwrite)
	}
	return
}

func dumpHelmScripts(data map[string]string, outputPath string, out io.Writer, overwrite bool) {
	if outputPath == "" {
		return
	}
	for k, v := range data {
		f := filepath.Join(outputPath, k)
		if ok, _ := cfgutil.CheckPathExists(f); ok {
			fmt.Fprintf(out, "file [%s] is exists\n", printer.BoldRed(k))
			if !overwrite {
				os.Exit(-1)
			}
		}
		util.CheckErr(os.WriteFile(f, []byte(v), os.ModePerm))
	}
}

func (o *migrateOptions) buildFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.helmTemplateDir, "helm", "", "specify the helm template dir")
	cmd.Flags().StringVar(&o.scriptsOutputPath, "output", "", "specify the scripts output path")
	cmd.Flags().StringVar(&o.cmName, "configmap", "", "specify the configmap name")
	cmd.Flags().StringVar(&o.regex, "regex", "", "specify the regex for configmap")
	cmd.Flags().BoolVar(&o.overwrite, "force", false, "whether overwrite the exists file")
}

var migrateExamples = templates.Examples(` 
`)

func NewMigrateHelmScriptsCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &migrateOptions{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "migrate-scripts",
		Aliases: []string{"migrate"},
		Short:   "migrate - a developer tool.",
		Example: migrateExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run())
		},
	}
	o.buildFlags(cmd)
	return cmd
}
