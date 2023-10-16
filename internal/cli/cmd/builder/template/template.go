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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

type renderTPLCmdOpts struct {
	genericiooptions.IOStreams

	Factory cmdutil.Factory
	// dynamic dynamic.Interface

	clusterYaml    string
	clusterDef     string
	clusterVersion string

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
	viper.SetDefault(constant.KubernetesClusterDomainEnv, constant.DefaultDNSDomain)
	workflow, err := NewWorkflowTemplateRender(o.helmOutputDir, o.opts, o.clusterDef, o.clusterVersion)
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
	cmd.Flags().StringVar(&o.clusterVersion, "cluster-version", "", "specify the cluster version name")
	cmd.Flags().StringVar(&o.clusterDef, "cluster-definition", "", "specify the cluster definition name")
	cmd.Flags().StringVarP(&o.outputDir, "output-dir", "o", "", "specify the output directory")

	cmd.Flags().StringVar(&o.opts.ConfigSpec, "config-spec", "", "specify the config spec to be rendered")

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

	return HelmTemplate(o.helmTemplateDir, o.helmOutputDir)
}

func NewComponentTemplateRenderCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
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
		Short:   "tpl - a developer tool integrated with KubeBlocks that can help developers quickly generate rendered configurations or scripts based on Helm templates, and discover errors in the template before creating the database cluster.",
		Example: templateExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run())
		},
	}
	o.buildTemplateFlags(cmd)
	return cmd
}
