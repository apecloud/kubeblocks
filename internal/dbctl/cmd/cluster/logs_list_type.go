/*
Copyright 2022 The KubeBlocks Authors

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

package cluster

import (
	"fmt"
	"io"
	"strings"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	cmddes "k8s.io/kubectl/pkg/describe"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/describe"
	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

var (
	logsListExample = templates.Examples(i18n.T(`
		# Display supported log file from cluster mysql-cluster with default leader instance
		dbctl cluster logs-list mysql-cluster

		# Display supported log file from cluster mysql-cluster with specify instance release-name-replicasets-0
		dbctl cluster logs-list mysql-cluster -i release-name-replicasets-0`))
)

// LogsListOptions declare the arguments accepted by the logs-list-type command
type LogsListOptions struct {
	namespace   string
	clusterName string
	instName    string

	dynamicClient dynamic.Interface
	clientSet     *kubernetes.Clientset
	factory       cmdutil.Factory
	genericclioptions.IOStreams
	dataObj *types.ClusterObjects
}

// NewLogsListTypeCmd return logs list type cmd
func NewLogsListTypeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &LogsListOptions{
		factory:   f,
		IOStreams: streams,
		dataObj:   cluster.NewClusterObjects(),
	}

	cmd := &cobra.Command{
		Use:     "logs-list-type",
		Short:   "List the supported log file types in cluster",
		Example: logsListExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVarP(&o.instName, "instance", "i", "", "Instance name.")
	return cmd
}

func (o *LogsListOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must specify the cluster name")
	}
	return nil
}

func (o *LogsListOptions) Complete(f cmdutil.Factory, args []string) (err error) {
	// set cluster name from args
	o.clusterName = args[0]
	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return
	}
	o.clientSet, err = o.factory.KubernetesClientSet()
	if err != nil {
		return
	}
	o.dynamicClient, err = f.DynamicClient()
	return
}

func (o *LogsListOptions) Run() error {
	clusterGetter := cluster.ObjectsGetter{
		ClientSet:      o.clientSet,
		DynamicClient:  o.dynamicClient,
		Name:           o.clusterName,
		Namespace:      o.namespace,
		WithAppVersion: false,
		WithConfigMap:  false,
	}
	if err := clusterGetter.Get(o.dataObj); err != nil {
		return err
	}
	if err := printLogContext(o.dataObj, o.Out, o.instName); err != nil {
		return err
	}
	return nil
}

// printLogContext print logs list type info
func printLogContext(dataObj *types.ClusterObjects, out io.Writer, instName string) error {
	engineName := dataObj.ClusterDef.Spec.Type
	logContext, err := engine.LogsContext(engineName)
	if err != nil {
		return err
	}
	c := dataObj.Cluster
	w := cmddes.NewPrefixWriter(out)
	w.Write(describe.LEVEL_0, "ClusterName:\t\t%s\n", c.Name)
	w.Write(describe.LEVEL_0, "Namespace:\t\t%s\n", c.Namespace)
	w.Write(describe.LEVEL_0, "AppVersion:\t\t%s\n", c.Spec.AppVersionRef)
	w.Write(describe.LEVEL_0, "ClusterDefinition:\t%s\n", c.Spec.ClusterDefRef)
	for _, p := range dataObj.Pods.Items {
		if len(instName) > 0 && !strings.EqualFold(p.Name, instName) {
			continue
		}
		componentName, ok := p.Labels[types.ComponentLabelKey]
		if ok {
			w.Write(describe.LEVEL_0, "\nInstance Name:\t%s\n", p.Name)
			w.Write(describe.LEVEL_0, "Component Name:\t%s\n", componentName)
			for key, value := range logContext {
				w.Write(describe.LEVEL_0, "Log file type :\t%s\n", key)
				w.Write(describe.LEVEL_2, "Log file describe:\t\t%s\n", value.Describe)
				w.Write(describe.LEVEL_2, "Log related variables:\t%s\n", strings.Join(value.Variables, ", "))
				// todo output more log file info
			}
		}
	}
	w.Flush()
	return nil
}
