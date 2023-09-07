package cluster

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var clusterPullExample = templates.Examples(`
	# Pull a cluster type to local and register it to "kbcli cluster create" sub-cmd from KubeBlocks community
	kbcli cluster pull mysql 
	
	# Pull a cluster type to local and register it to "kbcli cluster create" sub-cmd from specified Url
	kbcli cluster pull orioledb --url https://github.com/apecloud/helm-charts/releases/download/orioledb-cluster-0.6.0-beta.44/orioledb-cluster-0.6.0-beta.44.tgz
`)

type pullOption struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams

	clusterType cluster.ClusterType
	url         string
	alias       string
}

func newPullOption(f cmdutil.Factory, streams genericclioptions.IOStreams) *pullOption {
	o := &pullOption{
		Factory:   f,
		IOStreams: streams,
	}
	return o
}

func newPullCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newPullOption(f, streams)
	cmd := &cobra.Command{
		Use:     "pull [NAME] [CHART-URL]",
		Short:   "Pull the cluster chart to the local cache and add the NAME to 'create' sub-command",
		Example: clusterCreateExample,
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
func (o *pullOption) validate() error {
	if len(o.clusterType) > 15 {
		return fmt.Errorf("cluster type %s is too long as a sub command", o.clusterType.String())
	}

	for key, _ := range cluster.ClusterTypeCharts {
		if key == o.clusterType {
			return fmt.Errorf("cluster type %s is already exsited", o.clusterType.String())
		}
	}

	return nil
}

func (o *pullOption) run() error {
	chartsDownloader, err := helm.NewDownloader(helm.NewConfig("default", "", "", false))
	if err != nil {
		return err
	}
	homeDir, err := util.GetCliHomeDir()
	if err != nil {
		return err
	}
	chartsCacheDir := filepath.Join(homeDir, types.CliChartsCache)
	_, _, err = chartsDownloader.DownloadTo(o.url, "", chartsCacheDir)

	cluster.ClusterList = append(cluster.ClusterList, &cluster.ClusterTypes{
		Name:  o.clusterType,
		Url:   o.url,
		Alias: o.alias,
	})
	newConfig, err := yaml.Marshal(cluster.ClusterList)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(homeDir, types.CliClusterTypeConfigs), newConfig, 0666)

}
