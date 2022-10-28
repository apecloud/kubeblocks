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
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/exec"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

type LogsOptions struct {
	use         string
	short       string
	clusterName string
	instName    string
	container   string
	follow      bool
	tail        int
	limitBytes  int
	fileType    string
	filePath    string
	*exec.ExecOptions
}

// NewLogsCmd return the logic of accessing up-to-date server log file
func NewLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	l := &LogsOptions{
		use:         "logs",
		short:       "Access up-to-date server log file",
		ExecOptions: exec.NewExecOptions(f, streams),
	}
	input := &exec.ExecInput{
		Use:      l.use,
		Short:    l.short,
		Validate: l.validate,
		Complete: l.complete,
		AddFlags: l.addFlags,
	}
	return l.Build(input)
}

func (o *LogsOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.instName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVarP(&o.container, "container", "c", "", "Container name.")
	cmd.Flags().BoolVarP(&o.follow, "follow", "f", false, "Specify if the logs should be streamed")
	cmd.Flags().IntVar(&o.tail, "tail", -1, "Lines of recent log file to display. Defaults to -1 with showing all log lines")
	cmd.Flags().IntVar(&o.limitBytes, "limit-bytes", 0, "Maximum bytes of logs to return")
	cmd.Flags().StringVar(&o.fileType, "file-type", "", "Log file type. Can see the output info of logs-list cmd.")
	cmd.Flags().StringVar(&o.filePath, "file-path", "", "Log file path. Specify target file path and have a premium priority.")
	cmd.MarkFlagsMutuallyExclusive("file-path", "file-type")
}

// complete logs logic
func (o *LogsOptions) complete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify the cluster name to retrieve logs")
	}
	o.clusterName = args[0]

	dynamicClient, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	o.PodName = o.instName
	// no input podName and find the default pod of cluster
	if len(o.PodName) == 0 {
		if o.PodName, err = cluster.GetDefaultPodName(dynamicClient, o.clusterName, o.Namespace); err != nil {
			return err
		}
	}
	pod, err := o.ClientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// get cluster engine name
	engineName, err := cluster.GetClusterTypeByPod(pod)
	if err != nil {
		return err
	}
	var filePath string
	// specify file path and direct use this
	if len(o.filePath) > 0 {
		filePath = o.filePath
	} else {
		logContext, err := engine.LogsContext(engineName)
		if err != nil {
			return err
		}
		fileInfo, ok := logContext[o.fileType]
		if !ok {
			return fmt.Errorf("file type %s is not supported yet", o.fileType)
		}
		filePath, err = GetLogFilePath(fileInfo)
		if err != nil {
			return err
		}
	}
	o.Command = AssembleTailCommand(o.follow, o.tail, o.limitBytes, filePath)
	o.ContainerName = o.container
	o.Pod = pod
	return nil
}

func GetLogFilePath(logVar engine.LogVariables) (string, error) {
	// todo get filepath from config manager
	return logVar.DefaultFilePath, nil
}

func AssembleTailCommand(follow bool, tail int, limitBytes int, filePath string) []string {
	command := make([]string, 0, 5)
	command = append(command, "tail")
	if follow {
		command = append(command, "-f")
	}
	if tail == -1 {
		command = append(command, "--lines=+1")
	} else {
		command = append(command, "--lines="+strconv.Itoa(tail))
	}
	if limitBytes > 0 {
		command = append(command, "--bytes="+strconv.Itoa(limitBytes))
	}
	command = append(command, filePath)
	fmt.Println(command)
	return command
}

func (o *LogsOptions) validate() error {
	if len(o.clusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	if o.limitBytes < 0 {
		return fmt.Errorf("--limit-bytes must be greater than 0")
	}
	if o.tail < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}
	return nil
}
