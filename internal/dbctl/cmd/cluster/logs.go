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
	"io"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubectl/pkg/polymorphichelpers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdlogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/exec"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

var (
	logsExample = templates.Examples(i18n.T(`
		# Return snapshot logs from cluster mysql-cluster with default primary instance (stdout)
		dbctl cluster logs mysql-cluster

		# Display only the most recent 20 lines from cluster mysql-cluster with default leader instance (stdout)
		dbctl cluster logs --tail=20 mysql-cluster

		# Return snapshot logs from cluster mysql-cluster with specify instance mysql-cluster-replicasets-0 (stdout)
		dbctl cluster logs mysql-cluster -i mysql-cluster-replicasets-0 

		# Return snapshot logs from cluster mysql-cluster with specify instance mysql-cluster-replicasets-0 and specify mysql container (stdout)
		dbctl cluster logs mysql-cluster -i mysql-cluster-replicasets-0 -c mysql

		# Return slow logs from cluster mysql-cluster with default leader instance
		dbctl cluster logs mysql-cluster --file-type=slow

		# Begin streaming the slow logs from cluster mysql-cluster with default leader instance
		dbctl cluster logs -f mysql-cluster --file-type=slow

		# Return the specify file logs from cluster mysql-cluster with specify instance mysql-cluster-replicasets-0
		dbctl cluster logs mysql-cluster -i mysql-cluster-replicasets-0 --file-path=/var/log/yum.log

		# Return the specify file logs from cluster mysql-cluster with specify instance mysql-cluster-replicasets-0 and specify mysql container
		dbctl cluster logs mysql-cluster -i mysql-cluster-replicasets-0 -c mysql --file-path=/var/log/yum.log`))
)

// LogsOptions declare the arguments accepted by the logs command
type LogsOptions struct {
	use         string
	short       string
	clusterName string
	instName    string
	fileType    string
	filePath    string
	*exec.ExecOptions
	logOptions cmdlogs.LogsOptions
}

// NewLogsCmd return the logic of accessing up-to-date server log file
func NewLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	l := &LogsOptions{
		use:         "logs",
		short:       "Access up-to-date server log file",
		ExecOptions: exec.NewExecOptions(f, streams),
		logOptions: cmdlogs.LogsOptions{
			IOStreams: streams,
		},
	}
	input := &exec.ExecInput{
		Use:      l.use,
		Short:    l.short,
		Example:  logsExample,
		Validate: l.validate,
		Complete: l.complete,
		AddFlags: l.addFlags,
		Run:      l.run,
	}
	return l.Build(input)
}

func (o *LogsOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.instName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVarP(&o.logOptions.Container, "container", "c", "", "Container name.")
	cmd.Flags().BoolVarP(&o.logOptions.Follow, "follow", "f", false, "Specify if the logs should be streamed.")
	cmd.Flags().Int64Var(&o.logOptions.Tail, "tail", -1, "Lines of recent log file to display. Defaults to -1 with showing all log lines.")
	cmd.Flags().Int64Var(&o.logOptions.LimitBytes, "limit-bytes", 0, "Maximum bytes of logs to return.")
	cmd.Flags().BoolVar(&o.logOptions.Prefix, "prefix", false, "Prefix each log line with the log source (pod name and container name). Only take effect for stdout&stderr.")
	cmd.Flags().BoolVar(&o.logOptions.IgnoreLogErrors, "ignore-errors", false, "If watching / following pod logs, allow for any errors that occur to be non-fatal. Only take effect for stdout&stderr.")
	cmd.Flags().BoolVar(&o.logOptions.Timestamps, "timestamps", false, "Include timestamps on each line in the log output. Only take effect for stdout&stderr.")
	cmd.Flags().StringVar(&o.logOptions.SinceTime, "since-time", o.logOptions.SinceTime, i18n.T("Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr."))
	cmd.Flags().DurationVar(&o.logOptions.SinceSeconds, "since", o.logOptions.SinceSeconds, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr.")
	cmd.Flags().BoolVarP(&o.logOptions.Previous, "previous", "p", o.logOptions.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists. Only take effect for stdout&stderr.")

	cmd.Flags().StringVar(&o.fileType, "file-type", "", "Log-file type. Can see the output info of logs-list cmd. No set file-path and file-type will output stdout/stderr of target container.")
	cmd.Flags().StringVar(&o.filePath, "file-path", "", "Log-file path. Specify target file path and have a premium priority. No set file-path and file-type will output stdout/stderr of target container.")

	cmd.MarkFlagsMutuallyExclusive("file-path", "file-type")
	cmd.MarkFlagsMutuallyExclusive("since", "since-time")
}

// run custom run logic for logs
func (o *LogsOptions) run() (bool, error) {
	if o.isStdoutForContainer() {
		return false, o.runLogs()
	}
	return true, nil
}

// complete custom complete function for logs
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
	// no set podName and find the default pod of cluster
	if len(o.PodName) == 0 {
		if o.PodName, err = cluster.GetDefaultPodName(dynamicClient, o.clusterName, o.Namespace); err != nil {
			return err
		}
	}
	pod, err := o.ClientSet.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	filePath := o.filePath
	if len(filePath) == 0 {
		if o.isStdoutForContainer() {
			// no set file-path and file-type, and will output container's stdout & stderr, like kubectl logs
			o.logOptions.RESTClientGetter = o.Factory
			o.logOptions.LogsForObject = polymorphichelpers.LogsForObjectFn
			o.logOptions.Object = pod
			o.logOptions.Options, _ = o.logOptions.ToLogOptions()
		} else {
			// get cluster engine name
			engineName, err := cluster.GetClusterTypeByPod(pod)
			if err != nil {
				return err
			}
			logContext, err := engine.LogsContext(engineName)
			if err != nil {
				return err
			}
			fileInfo, ok := logContext[o.fileType]
			if !ok {
				return fmt.Errorf("file type %s is not supported yet", o.fileType)
			}
			filePath, err = getLogFilePath(fileInfo, o.clusterName)
			if err != nil {
				return err
			}
		}
	}
	o.Command = assembleTailCommand(o.logOptions.Follow, o.logOptions.Tail, o.logOptions.LimitBytes, filePath)
	o.ContainerName = o.logOptions.Container
	o.Pod = pod
	return nil
}

func (o *LogsOptions) validate() error {
	if len(o.clusterName) == 0 {
		return fmt.Errorf("cluster name must be specified")
	}
	if o.logOptions.LimitBytes < 0 {
		return fmt.Errorf("--limit-bytes must be greater than 0")
	}
	if o.logOptions.Tail < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}
	if o.isStdoutForContainer() {
		if len(o.logOptions.SinceTime) > 0 && o.logOptions.SinceSeconds != 0 {
			return fmt.Errorf("at most one of `sinceTime` or `sinceSeconds` may be specified")
		}

		logsOptions, ok := o.logOptions.Options.(*corev1.PodLogOptions)
		if !ok {
			return fmt.Errorf("unexpected logs options object")
		}
		if logsOptions.SinceSeconds != nil && *logsOptions.SinceSeconds < int64(0) {
			return fmt.Errorf("--since must be greater than 0")
		}

		if logsOptions.TailLines != nil && *logsOptions.TailLines < -1 {
			return fmt.Errorf("--tail must be greater than or equal to -1")
		}
	}
	return nil
}

// getLogFilePath get file path according to variable of engine
func getLogFilePath(logInfo engine.LogVariables, clusterName string) (string, error) {
	if len(logInfo.DefaultFilePath) > 0 {
		return logInfo.DefaultFilePath, nil
	} else {
		// todo get filepath variable from ConfigManager module
		return "", nil
	}
}

// assembleTailCommand assemble tail command for log file
func assembleTailCommand(follow bool, tail int64, limitBytes int64, filePath string) []string {
	command := make([]string, 0, 5)
	command = append(command, "tail")
	if follow {
		command = append(command, "-f")
	}
	if tail == -1 {
		command = append(command, "--lines=+1")
	} else {
		command = append(command, "--lines="+strconv.FormatInt(tail, 10))
	}
	if limitBytes > 0 {
		command = append(command, "--bytes="+strconv.FormatInt(limitBytes, 10))
	}
	command = append(command, filePath)
	return command
}

func (o *LogsOptions) isStdoutForContainer() bool {
	if len(o.filePath) == 0 {
		return len(o.fileType) == 0 || strings.EqualFold(o.fileType, "stdout") || strings.EqualFold(o.fileType, "stderr")
	}
	return false
}

// runLogs retrieve stdout/stderr logs
func (o *LogsOptions) runLogs() error {
	requests, err := o.logOptions.LogsForObject(o.logOptions.RESTClientGetter, o.logOptions.Object, o.logOptions.Options, 60*time.Second, false)
	if err != nil {
		return err
	}
	for objRef, request := range requests {
		out := o.addPrefixIfNeeded(objRef, o.Out)
		if err := cmdlogs.DefaultConsumeRequest(request, out); err != nil {
			if !o.logOptions.IgnoreLogErrors {
				return err
			}
			fmt.Fprintf(o.Out, "error: %v\n", err)
		}
	}
	return nil
}

func (o *LogsOptions) addPrefixIfNeeded(ref corev1.ObjectReference, writer io.Writer) io.Writer {
	if !o.logOptions.Prefix || ref.FieldPath == "" || ref.Name == "" {
		return writer
	}
	prefix := fmt.Sprintf("[pod/%s/%s] ", ref.Name, o.ContainerName)
	return &prefixingWriter{
		prefix: []byte(prefix),
		writer: writer,
	}
}

type prefixingWriter struct {
	prefix []byte
	writer io.Writer
}

func (pw *prefixingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n, err := pw.writer.Write(append(pw.prefix, p...))
	if n > len(p) {
		// To comply with the io.Writer interface requirements we must
		// return a number of bytes written from p (0 <= n <= len(p)),
		// so we are ignoring the length of the prefix here.
		return len(p), err
	}
	return n, err
}
