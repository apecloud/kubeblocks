/*
Copyright ApeCloud, Inc.

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
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	logsListExample = templates.Examples(`
		# Display supported log files in cluster mycluster with all instance
		kbcli cluster list-logs mycluster

		# Display supported log files in cluster mycluster with specify component my-component
		kbcli cluster list-logs mycluster --component my-component

		# Display supported log files in cluster mycluster with specify instance my-instance-0
		kbcli cluster list-logs mycluster --instance my-instance-0`)
)

// ListLogsOptions declares the arguments accepted by the list-logs command
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

func NewListLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListLogsOptions{
		factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:               "list-logs NAME",
		Short:             "List supported log files in cluster.",
		Example:           logsListExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
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
	o.exec.Config = config
	// hide unnecessary output
	o.exec.Quiet = true
	return err
}

func (o *ListLogsOptions) Run() error {
	clusterGetter := cluster.ObjectsGetter{
		Client:    o.clientSet,
		Dynamic:   o.dynamicClient,
		Name:      o.clusterName,
		Namespace: o.namespace,
		GetOptions: cluster.GetOptions{
			WithClusterDef: true,
			WithPod:        true,
		},
	}
	dataObj, err := clusterGetter.Get()
	if err != nil {
		return err
	}
	if err := o.printListLogs(dataObj); err != nil {
		return err
	}
	return nil
}

// printListLogs prints the result of list-logs command to stdout.
func (o *ListLogsOptions) printListLogs(dataObj *cluster.ClusterObjects) error {
	tbl := printer.NewTablePrinter(o.Out)
	logFilesData := o.gatherLogFilesData(dataObj.Cluster, dataObj.ClusterDef, dataObj.Pods)
	if len(logFilesData) == 0 {
		fmt.Fprintf(o.ErrOut, "No log files found. You can enable the log feature with the kbcli command below.\n"+
			"kbcli cluster update %s --enable-all-logs=true --namespace %s\n", dataObj.Cluster.Name, dataObj.Cluster.Namespace)
	} else {
		tbl.SetHeader("INSTANCE", "LOG-TYPE", "FILE-PATH", "SIZE", "LAST-WRITTEN", "COMPONENT")
		for _, f := range logFilesData {
			tbl.AddRow(f.instance, f.logType, f.filePath, f.size, f.lastWritten, f.component)
		}
		tbl.Print()
	}
	return nil
}

type logFileInfo struct {
	instance    string
	logType     string
	filePath    string
	size        string
	lastWritten string
	component   string
}

// gatherLogFilesData gathers all log files data from every instance of the cluster.
func (o *ListLogsOptions) gatherLogFilesData(c *appsv1alpha1.Cluster, cd *appsv1alpha1.ClusterDefinition, pods *corev1.PodList) []logFileInfo {
	logFileInfoList := make([]logFileInfo, 0, len(pods.Items))
	for _, p := range pods.Items {
		if len(o.instName) > 0 && !strings.EqualFold(p.Name, o.instName) {
			continue
		}
		componentName, ok := p.Labels[constant.KBAppComponentLabelKey]
		if !ok || (len(o.componentName) > 0 && !strings.EqualFold(o.componentName, componentName)) {
			continue
		}
		var compDefName string
		logTypeMap := make(map[string]struct{})
		// find component compDefName and enabledLogs config against componentName in pod's label.
		for _, comCluster := range c.Spec.ComponentSpecs {
			if !strings.EqualFold(comCluster.Name, componentName) {
				continue
			}
			compDefName = comCluster.ComponentDefRef
			for _, logType := range comCluster.EnabledLogs {
				logTypeMap[logType] = struct{}{}
			}
			break
		}
		if len(compDefName) == 0 || len(logTypeMap) == 0 {
			continue
		}
		for _, com := range cd.Spec.ComponentDefs {
			if !strings.EqualFold(com.Name, compDefName) {
				continue
			}
			for _, logConfig := range com.LogConfigs {
				if _, ok := logTypeMap[logConfig.Name]; ok {
					realFile, err := o.getRealFileFromContainer(&p, logConfig.FilePathPattern)
					if err == nil {
						logFileInfoList = append(logFileInfoList, convertToLogFileInfo(realFile, logConfig.Name, p.Name, componentName)...)
					}
				}
			}
			break
		}
	}
	return logFileInfoList
}

// convertToLogFileInfo converts file info in string format to logFileInfo struct.
func convertToLogFileInfo(fileInfo, logType, instName, component string) []logFileInfo {
	fileList := strings.Split(fileInfo, "\n")
	logFileList := make([]logFileInfo, 0, len(fileList))
	for _, file := range fileList {
		fieldList := strings.Fields(file)
		if len(fieldList) == 0 {
			continue
		}
		logFileList = append(logFileList, logFileInfo{
			instance:    instName,
			component:   component,
			logType:     logType,
			size:        fieldList[4],
			lastWritten: strings.Join(fieldList[5:10], " "),
			filePath:    fieldList[10],
		})
	}
	return logFileList
}

// getRealFileFromContainer gets real log files against pattern from container, and returns file info in string format
func (o *ListLogsOptions) getRealFileFromContainer(pod *corev1.Pod, pattern string) (string, error) {
	o.exec.Pod = pod
	// linux cmd : ls -lh --time-style='+%b %d, %Y %H:%M (UTC%:z)' pattern
	o.exec.Command = []string{"/bin/bash", "-c", "ls -lh --time-style='+%b %d, %Y %H:%M (UTC%:z)' " + pattern}
	// set customized output
	out := bytes.Buffer{}
	o.exec.Out = &out
	o.exec.ErrOut = os.Stdout
	o.exec.TTY = false
	if err := o.exec.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}
