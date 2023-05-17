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

package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcontainer "github.com/apecloud/kubeblocks/internal/configuration/container"
)

type renderTPLCmdOpts struct {
	genericclioptions.IOStreams

	Factory cmdutil.Factory
	// dynamic dynamic.Interface

	clusterYaml    string
	clusterDefYaml string

	outputDir       string
	clearOutputDir  bool
	helmOutputDir   string
	helmTemplateDir string

	opts RenderedOptions
}

func (o *renderTPLCmdOpts) complete() error {
	if err := o.checkAndHelmTemplate(); err != nil {
		return err
	}

	if o.helmOutputDir == "" {
		return cfgcore.MakeError("helm template dir is empty")
	}

	if o.clearOutputDir && o.outputDir != "" {
		_ = os.RemoveAll(o.outputDir)
	}
	if o.outputDir == "" {
		o.outputDir = filepath.Join("./output", RandomString(6))
	}
	return nil
}

func (o *renderTPLCmdOpts) run() error {
	workflow, err := NewWorkflowTemplateRender(o.helmOutputDir, o.opts)
	if err != nil {
		return err
	}
	return workflow.Do(o.outputDir)
}

var templateExamples = templates.Examples(`
    # builder template: Provides a mechanism to rendered template for ComponentConfigSpec and ComponentScriptSpec in the ClusterComponentDefinition.
    # builder template --helm deploy/redis --memory=64Gi --cpu=16 --replicas=3 --component-name=redis --config-spec=redis-replication-config  

    # build all configspec
    kbcli builder template --helm deploy/redis -a
`)

// buildReconfigureCommonFlags build common flags for reconfigure command
func (o *renderTPLCmdOpts) buildTemplateFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.clusterYaml, "cluster", "", "the cluster yaml file")
	cmd.Flags().StringVar(&o.clusterDefYaml, "cluster-definition", "", "the cluster definition yaml file")
	cmd.Flags().StringVarP(&o.outputDir, "output-dir", "o", "", "specify the output directory")

	cmd.Flags().StringVar(&o.opts.ConfigSpec, "config-spec", "", "specify the config spec to be rendered")
	cmd.Flags().BoolVarP(&o.opts.AllConfigSpecs, "all", "a", false, "template all config specs")

	// mock cluster object
	cmd.Flags().Int32VarP(&o.opts.Replicas, "replicas", "r", 1, "specify the replicas of the component")
	cmd.Flags().StringVar(&o.opts.DataVolumeName, "volume-name", "", "specify the data volume name of the component")
	cmd.Flags().StringVar(&o.opts.ComponentName, "component-name", "", "specify the component name of the clusterdefinition")
	cmd.Flags().StringVar(&o.helmTemplateDir, "helm", "", "specify the helm template dir")
	cmd.Flags().StringVar(&o.helmOutputDir, "helm-output", "", "specify the helm template output dir")
	cmd.Flags().StringVar(&o.opts.CPU, "cpu", "", "specify the cpu of the component")
	cmd.Flags().StringVar(&o.opts.Memory, "memory", "", "specify the memory of the component")
	cmd.Flags().BoolVar(&o.clearOutputDir, "clean", false, "specify whether to clear the output dir")
}

func (o *renderTPLCmdOpts) checkAndHelmTemplate() error {
	if o.helmTemplateDir == "" || o.helmOutputDir != "" {
		return nil
	}

	if o.helmTemplateDir != "" && o.helmOutputDir == "" {
		o.helmOutputDir = filepath.Join("./helm-output", RandomString(6))
	}
	cmd := exec.Command("helm", "template", o.helmTemplateDir, "--output-dir", o.helmOutputDir)
	stdout, err := cfgcontainer.ExecShellCommand(cmd)
	if err != nil {
		return err
	}
	fmt.Println(stdout)
	return nil
}

func NewComponentTemplateRenderCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &renderTPLCmdOpts{
		Factory:   f,
		IOStreams: streams,
		opts: RenderedOptions{
			// for mock cluster object
			Namespace: "default",
			Name:      "cluster-" + RandomString(6),
		},
	}
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"tpl"},
		Short:   "tpl - a developer tool integrated with Kubeblocks that can help developers quickly generate rendered configurations or scripts based on Helm templates, and discover errors in the template before creating the database cluster.",
		Example: templateExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run())
		},
	}
	o.buildTemplateFlags(cmd)
	return cmd
}
