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
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/describe"
	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	. "k8s.io/kubectl/pkg/describe"
	"time"
)

type LogsListOptions struct {
	namespace   string
	clusterName string
	instName    string

	dynamicClient dynamic.Interface
	clientSet     *kubernetes.Clientset
	factory       cmdutil.Factory
	genericclioptions.IOStreams
}

func NewLogsListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &LogsListOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "logs-list",
		Short: "List the supported log files in cluster",
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
	infoObject := cluster.NewClusterObjects()
	if err := cluster.GetAllObjects(o.clientSet, o.dynamicClient, o.namespace, o.clusterName, infoObject); err != nil {
		return err
	}

	engineName := infoObject.ClusterDef.Spec.Type
	logContext, err := engine.LogsContext(engineName)
	if err != nil {
		return err
	}
	c := infoObject.Cluster
	w := NewPrefixWriter(o.Out)
	w.Write(LEVEL_0, "Name:\t%s\n", c.Name)
	w.Write(LEVEL_0, "Namespace:\t%s\n", c.Namespace)
	w.Write(LEVEL_0, "Status:\t%s\n", c.Status.Phase)
	w.Write(LEVEL_0, "AppVersion:\t%s\n", c.Spec.AppVersionRef)
	w.Write(LEVEL_0, "ClusterDefinition:\t%s\n", c.Spec.ClusterDefRef)
	w.Write(LEVEL_0, "TerminationPolicy:\t%s\n", c.Spec.TerminationPolicy)
	w.Write(LEVEL_0, "CreationTimestamp:\t%s\n\n", c.CreationTimestamp.Time.Format(time.RFC1123Z))
	for _, component := range infoObject.ClusterDef.Spec.Components {
		w.Write(LEVEL_1, "Component:\t%s", component)
	}

	var index = 0
	for key, value := range logContext {
		index++
		w.Write(describe.LEVEL_0, "%d. log file type :\t%s\n", index, key)
		w.Write(describe.LEVEL_2, "default file path:\t%s\n", value.DefaultFilePath)
		w.Write(describe.LEVEL_2, "variables:\n")
		for _, v := range value.Variables {
			w.Write(describe.LEVEL_3, "%s:\t%s\n", v, v)
		}
	}
	w.Flush()
	return nil
}
