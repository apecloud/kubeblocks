/*
Copyright ApeCloud Inc.

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

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	cmddes "k8s.io/kubectl/pkg/describe"
	"k8s.io/kubectl/pkg/util/templates"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/describe"
	"github.com/apecloud/kubeblocks/internal/dbctl/exec"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

var (
	logsListExample = templates.Examples(`
		# Display supported log file in cluster my-cluster with all instance
		dbctl cluster list-logs-type my-cluster

        # Display supported log file in cluster my-cluster with specify component my-component
		dbctl cluster list-logs-type my-cluster --component my-component

		# Display supported log file in cluster my-cluster with specify instance my-instance-0
		dbctl cluster list-logs-type my-cluster --instance my-instance-0`)
)

// LogsListOptions declare the arguments accepted by the logs-list-type command
type LogsListOptions struct {
	namespace     string
	clusterName   string
	componentName string
	instName      string

	dynamicClient dynamic.Interface
	clientSet     *kubernetes.Clientset
	factory       cmdutil.Factory
	genericclioptions.IOStreams
	exec *exec.ExecOptions
}

// NewListLogsTypeCmd return logs list type cmd
func NewListLogsTypeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &LogsListOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "list-logs-type",
		Short:   "List the supported logs file types in cluster",
		Example: logsListExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVarP(&o.instName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVar(&o.componentName, "component", "", "Component name.")
	return cmd
}

func (o *LogsListOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must specify the cluster name")
	}
	return nil
}

func (o *LogsListOptions) Complete(f cmdutil.Factory, args []string) error {
	// set cluster name from args
	o.clusterName = args[0]
	config, err := o.factory.ToRESTConfig()
	if err != nil {
		return err
	}
	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.clientSet, err = o.factory.KubernetesClientSet()
	if err != nil {
		return err
	}
	o.dynamicClient, err = f.DynamicClient()
	o.exec = exec.NewExecOptions(o.factory, o.IOStreams)
	o.exec.Input = &exec.ExecInput{}
	o.exec.Config = config
	return err
}

func (o *LogsListOptions) Run() error {
	dataObj := cluster.NewClusterObjects()
	clusterGetter := cluster.ObjectsGetter{
		ClientSet:      o.clientSet,
		DynamicClient:  o.dynamicClient,
		Name:           o.clusterName,
		Namespace:      o.namespace,
		WithAppVersion: false,
		WithConfigMap:  false,
	}
	if err := clusterGetter.Get(dataObj); err != nil {
		return err
	}
	if err := o.printListLogsMessage(dataObj, o.Out); err != nil {
		return err
	}
	return nil
}

func (o *LogsListOptions) printHeaderMessage(w cmddes.PrefixWriter, c *dbaasv1alpha1.Cluster) {
	w.Write(describe.LEVEL_0, "ClusterName:\t\t%s\n", c.Name)
	w.Write(describe.LEVEL_0, "Namespace:\t\t%s\n", c.Namespace)
	w.Write(describe.LEVEL_0, "ClusterDefinition:\t%s\n", c.Spec.ClusterDefRef)
}

// printBodyMessage print message about logs file
func (o *LogsListOptions) printBodyMessage(w cmddes.PrefixWriter, c *dbaasv1alpha1.Cluster, cd *dbaasv1alpha1.ClusterDefinition, pods *corev1.PodList) {
	for _, p := range pods.Items {
		if len(o.instName) > 0 && !strings.EqualFold(p.Name, o.instName) {
			continue
		}
		componentName, ok := p.Labels[types.ComponentLabelKey]
		if len(o.componentName) > 0 && !strings.EqualFold(o.componentName, componentName) {
			continue
		}
		if ok {
			w.Write(describe.LEVEL_0, "\nInstance  Name:\t%s\n", p.Name)
			w.Write(describe.LEVEL_0, "Component Name:\t%s\n", componentName)
			var comTypeName string
			logTypeMap := make(map[string]bool)
			for _, comCluster := range c.Spec.Components {
				if strings.EqualFold(comCluster.Name, componentName) {
					if len(comCluster.EnableLogs) == 0 {
						w.Write(describe.LEVEL_0, "No log file open in %s, please set EnableLogs filed", comCluster.Name)
					} else {
						comTypeName = comCluster.Type
						for _, logType := range comCluster.EnableLogs {
							logTypeMap[logType] = true
						}
					}
					break
				}
			}
			if len(comTypeName) > 0 {
				// w.Write(describe.LEVEL_0, "Component Type:\t%s\n", comTypeName)
				for _, com := range cd.Spec.Components {
					if strings.EqualFold(com.TypeName, comTypeName) {
						for _, logConfig := range com.LogsConfig {
							_, ok := logTypeMap[logConfig.Name]
							if ok {
								w.Write(describe.LEVEL_0, "Log file type :\t%s\n", logConfig.Name)
								// todo display more log file info
								if len(logConfig.FilePathPattern) > 0 {
									o.printRealFileMessage(&p, logConfig.FilePathPattern)
								}
							}
						}
						break
					}
				}
			} else {
				w.Write(describe.LEVEL_0, "\nComponent name: %s can't find corresponding type in cluster yaml. \n", componentName)
			}
		}
	}
}

// printRealFileMessage in container
func (o *LogsListOptions) printRealFileMessage(pod *corev1.Pod, pattern string) {
	o.exec.Pod = pod
	o.exec.Command = []string{"/bin/bash", "-c", "ls -al " + pattern}
	if err := o.exec.Run(); err != nil {
		fmt.Printf("non-existed log file in container which searched by pattern %s\n", pattern)
	}
}

// printLogsContext print logs list type info
func (o *LogsListOptions) printListLogsMessage(dataObj *types.ClusterObjects, out io.Writer) error {
	w := cmddes.NewPrefixWriter(out)
	o.printHeaderMessage(w, dataObj.Cluster)
	o.printBodyMessage(w, dataObj.Cluster, dataObj.ClusterDef, dataObj.Pods)
	w.Flush()
	return nil
}
