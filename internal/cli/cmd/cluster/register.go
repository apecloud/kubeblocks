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

package cluster

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var clusterRegisterExample = templates.Examples(`
	# Pull a cluster type to local and register it to "kbcli cluster create" sub-cmd from specified URL
	kbcli cluster register orioledb --url https://github.com/apecloud/helm-charts/releases/download/orioledb-cluster-0.6.0-beta.44/orioledb-cluster-0.6.0-beta.44.tgz
`)

type registerOption struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams

	clusterType cluster.ClusterType
	url         string
	alias       string
}

func newRegisterOption(f cmdutil.Factory, streams genericclioptions.IOStreams) *registerOption {
	o := &registerOption{
		Factory:   f,
		IOStreams: streams,
	}
	return o
}

func newRegisterCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newRegisterOption(f, streams)
	cmd := &cobra.Command{
		Use:     "register [NAME] --url [CHART-URL]",
		Short:   "Pull the cluster chart to the local cache and register the type to 'create' sub-command",
		Example: clusterRegisterExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o.clusterType = cluster.ClusterType(args[0])
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}
	cmd.Flags().StringVar(&o.url, "url", "", "Specify the cluster type chart download url")
	cmd.Flags().StringVar(&o.alias, "alias", "", "Set the cluster type alias")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

// validate will check the
func (o *registerOption) validate() error {
	re := regexp.MustCompile(`^[a-zA-Z0-9]{1,9}$`)
	if !re.MatchString(o.clusterType.String()) {
		return fmt.Errorf("cluster type %s is not appropriate as a subcommand", o.clusterType.String())
	}

	for key := range cluster.ClusterTypeCharts {
		if key == o.clusterType {
			return fmt.Errorf("cluster type %s is already existed", o.clusterType.String())
		}
	}

	return nil
}

func (o *registerOption) run() error {
	chartsDownloader, err := helm.NewDownloader(helm.NewConfig("default", "", "", false))
	if err != nil {
		return err
	}

	_, _, err = chartsDownloader.DownloadTo(o.url, "", cluster.CliChartsCacheDir)
	if err != nil {
		return err
	}
	cluster.GlobalClusterChartConfig.AddConfig(&cluster.TypeInstance{
		Name:  o.clusterType,
		URL:   o.url,
		Alias: o.alias,
	})
	return cluster.GlobalClusterChartConfig.WriteConfigs(cluster.CliClusterChartConfig)

}
