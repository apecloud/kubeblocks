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
	"os"
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
	"github.com/apecloud/kubeblocks/internal/cli/cmd/describe"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/cluster"
)

var (
	logsListExample = templates.Examples(`
		# Display supported log file in cluster my-cluster with all instance
		kbcli cluster list-logs-type my-cluster

        # Display supported log file in cluster my-cluster with specify component my-component
		kbcli cluster list-logs-type my-cluster --component my-component

		# Display supported log file in cluster my-cluster with specify instance my-instance-0
		kbcli cluster list-logs-type my-cluster --instance my-instance-0`)
)

// ListLogsOptions declares the arguments accepted by the list-logs-type command
type ListLogsOptions struct {
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

// NewListLogsTypeCmd returns list logs type cmd
func NewListLogsTypeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListLogsOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "list-logs-type",
		Short:   "List the supported logs file types in cluster",
		Example: logsListExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Validate(args))
			util.CheckErr(o.Complete(f, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVarP(&o.instName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVar(&o.componentName, "component", "", "Component name.")
	return cmd
}

func (o *ListLogsOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must specify the cluster name")
	}
	return nil
}

func (o *ListLogsOptions) Complete(f cmdutil.Factory, args []string) error {
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
	// hide unnecessary output
	o.exec.Quiet = true
	return err
}

func (o *ListLogsOptions) Run() error {
	clusterGetter := cluster.ObjectsGetter{
		ClientSet:      o.clientSet,
		DynamicClient:  o.dynamicClient,
		Name:           o.clusterName,
		Namespace:      o.namespace,
		WithClusterDef: true,
		WithPod:        true,
	}
	dataObj, err := clusterGetter.Get()
	if err != nil {
		return err
	}
	if err := o.printListLogsMessage(dataObj, o.Out); err != nil {
		return err
	}
	return nil
}

func (o *ListLogsOptions) printHeaderMessage(w cmddes.PrefixWriter, c *dbaasv1alpha1.Cluster) {
	w.Write(describe.Level0, "ClusterName:\t\t%s\n", c.Name)
	w.Write(describe.Level0, "Namespace:\t\t%s\n", c.Namespace)
	w.Write(describe.Level0, "ClusterDefinition:\t%s\n", c.Spec.ClusterDefRef)
}

// printBodyMessage prints message about log files.
func (o *ListLogsOptions) printBodyMessage(w cmddes.PrefixWriter, c *dbaasv1alpha1.Cluster, cd *dbaasv1alpha1.ClusterDefinition, pods *corev1.PodList) {
	for _, p := range pods.Items {
		if len(o.instName) > 0 && !strings.EqualFold(p.Name, o.instName) {
			continue
		}
		componentName, ok := p.Labels[types.ComponentLabelKey]
		if !ok {
			w.Write(describe.Level0, "\nLabel key %s in pod %s isn't set \n", types.ComponentLabelKey, p.Name)
			continue
		}
		if len(o.componentName) > 0 && !strings.EqualFold(o.componentName, componentName) {
			continue
		}
		w.Write(describe.Level0, "\nInstance  Name:\t%s\n", p.Name)
		w.Write(describe.Level0, "Component Name:\t%s\n", componentName)
		var comTypeName string
		logTypeMap := make(map[string]struct{})
		// find component typeName and enabledLogs config according to componentName in pod's label.
		for _, comCluster := range c.Spec.Components {
			if !strings.EqualFold(comCluster.Name, componentName) {
				continue
			}
			comTypeName = comCluster.Type
			for _, logType := range comCluster.EnabledLogs {
				logTypeMap[logType] = struct{}{}
			}
		}
		if len(comTypeName) == 0 {
			w.Write(describe.Level0, "Component name %s in pod's label can't find corresponding typeName, please check cluster.yaml \n", componentName)
			continue
		}
		if len(logTypeMap) == 0 {
			w.Write(describe.Level0, "No logs type found. \nYou can enable the log feature when creating a cluster with option of \"--enable-all-logs=true\"\n")
			continue
		}
		var validCount int
		for _, com := range cd.Spec.Components {
			if !strings.EqualFold(com.TypeName, comTypeName) {
				continue
			}
			for _, logConfig := range com.LogConfigs {
				if _, ok := logTypeMap[logConfig.Name]; ok {
					validCount++
					w.Write(describe.Level0, "Log file type :\t%s\n", logConfig.Name)
					// todo display more log file info
					if len(logConfig.FilePathPattern) > 0 {
						o.printRealFileMessage(&p, logConfig.FilePathPattern)
					}
				}
			}
		}
		if len(logTypeMap) != validCount {
			w.Write(describe.Level0, "EnabledLogs have invalid logTypes, please look up cluster Status.Conditions by `kubectl describe cluster <cluster-name>`\n")
		}
	}
}

// printRealFileMessage prints real files in container
func (o *ListLogsOptions) printRealFileMessage(pod *corev1.Pod, pattern string) {
	o.exec.Pod = pod
	o.exec.Command = []string{"/bin/bash", "-c", "ls -al " + pattern}
	// because tty Raw argument will set ErrOut nil in exec.Run
	o.exec.ErrOut = os.Stdout
	if err := o.exec.Validate(); err != nil {
		fmt.Printf("validate fail when list log files in container by exec command, and error message : %s\n", err.Error())
	}
	if err := o.exec.Run(); err != nil {
		fmt.Printf("run fail when list log files in container by exec command, and error message : %s\n", err.Error())
	}
}

// printLogsContext prints list-logs-type info
func (o *ListLogsOptions) printListLogsMessage(dataObj *cluster.ClusterObjects, out io.Writer) error {
	w := cmddes.NewPrefixWriter(out)
	o.printHeaderMessage(w, dataObj.Cluster)
	o.printBodyMessage(w, dataObj.Cluster, dataObj.ClusterDef, dataObj.Pods)
	w.Flush()
	return nil
}
