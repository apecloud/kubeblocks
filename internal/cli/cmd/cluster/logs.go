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
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdlogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	logsExample = templates.Examples(`
		# Return snapshot logs from cluster mycluster with default primary instance (stdout)
		kbcli cluster logs mycluster

		# Display only the most recent 20 lines from cluster mycluster with default primary instance (stdout)
		kbcli cluster logs mycluster --tail=20

		# Display stdout info of specific instance my-instance-0 (cluster name comes from annotation app.kubernetes.io/instance)
		kbcli cluster logs --instance my-instance-0

		# Return snapshot logs from cluster mycluster with specific instance my-instance-0 (stdout)
		kbcli cluster logs mycluster --instance my-instance-0

		# Return snapshot logs from cluster mycluster with specific instance my-instance-0 and specific container
        # my-container (stdout)
		kbcli cluster logs mycluster --instance my-instance-0 -c my-container

		# Return slow logs from cluster mycluster with default primary instance
		kbcli cluster logs mycluster --file-type=slow

		# Begin streaming the slow logs from cluster mycluster with default primary instance
		kbcli cluster logs -f mycluster --file-type=slow

		# Return the specific file logs from cluster mycluster with specific instance my-instance-0
		kbcli cluster logs mycluster --instance my-instance-0 --file-path=/var/log/yum.log

		# Return the specific file logs from cluster mycluster with specific instance my-instance-0 and specific
        # container my-container
		kbcli cluster logs mycluster --instance my-instance-0 -c my-container --file-path=/var/log/yum.log`)
)

// LogsOptions declares the arguments accepted by the logs command
type LogsOptions struct {
	clusterName string
	fileType    string
	filePath    string
	*exec.ExecOptions
	logOptions cmdlogs.LogsOptions
}

// NewLogsCmd returns the logic of accessing cluster log file
func NewLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	l := &LogsOptions{
		ExecOptions: exec.NewExecOptions(f, streams),
		logOptions: cmdlogs.LogsOptions{
			IOStreams: streams,
		},
	}
	cmd := &cobra.Command{
		Use:               "logs NAME",
		Short:             "Access cluster log file.",
		Example:           logsExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(l.ExecOptions.Complete())
			util.CheckErr(l.complete(args))
			util.CheckErr(l.validate())
			util.CheckErr(l.run())
		},
	}
	l.addFlags(cmd)
	return cmd
}

func (o *LogsOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.PodName, "instance", "i", "", "Instance name.")
	cmd.Flags().StringVarP(&o.logOptions.Container, "container", "c", "", "Container name.")
	cmd.Flags().BoolVarP(&o.logOptions.Follow, "follow", "f", false, "Specify if the logs should be streamed.")
	cmd.Flags().Int64Var(&o.logOptions.Tail, "tail", -1, "Lines of recent log file to display. Defaults to -1 for showing all log lines.")
	cmd.Flags().Int64Var(&o.logOptions.LimitBytes, "limit-bytes", 0, "Maximum bytes of logs to return.")
	cmd.Flags().BoolVar(&o.logOptions.Prefix, "prefix", false, "Prefix each log line with the log source (pod name and container name). Only take effect for stdout&stderr.")
	cmd.Flags().BoolVar(&o.logOptions.IgnoreLogErrors, "ignore-errors", false, "If watching / following pod logs, allow for any errors that occur to be non-fatal. Only take effect for stdout&stderr.")
	cmd.Flags().BoolVar(&o.logOptions.Timestamps, "timestamps", false, "Include timestamps on each line in the log output. Only take effect for stdout&stderr.")
	cmd.Flags().StringVar(&o.logOptions.SinceTime, "since-time", o.logOptions.SinceTime, "Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr.")
	cmd.Flags().DurationVar(&o.logOptions.SinceSeconds, "since", o.logOptions.SinceSeconds, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr.")
	cmd.Flags().BoolVarP(&o.logOptions.Previous, "previous", "p", o.logOptions.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists. Only take effect for stdout&stderr.")

	cmd.Flags().StringVar(&o.fileType, "file-type", "", "Log-file type. List them with list-logs cmd. When file-path and file-type are unset, output stdout/stderr of target container.")
	cmd.Flags().StringVar(&o.filePath, "file-path", "", "Log-file path. File path has a priority over file-type. When file-path and file-type are unset, output stdout/stderr of target container.")

	cmd.MarkFlagsMutuallyExclusive("file-path", "file-type")
	cmd.MarkFlagsMutuallyExclusive("since", "since-time")
}

// run customs logic for logs
func (o *LogsOptions) run() error {
	if o.isStdoutForContainer() {
		return o.runLogs()
	}
	return o.ExecOptions.Run()
}

// complete customs complete function for logs
func (o *LogsOptions) complete(args []string) error {
	if len(args) == 0 && len(o.PodName) == 0 {
		return fmt.Errorf("cluster name or instance name should be specified")
	}
	if len(args) > 0 {
		o.clusterName = args[0]
	}
	// podName not set, find the default pod of cluster
	if len(o.PodName) == 0 {
		infos := cluster.GetSimpleInstanceInfos(o.Dynamic, o.clusterName, o.Namespace)
		if len(infos) == 0 || infos[0].Name == constant.ComponentStatusDefaultPodName {
			return fmt.Errorf("failed to find the default instance, please check cluster status")
		}
		// first element is the default instance to connect
		o.PodName = infos[0].Name
	}
	pod, err := o.Client.CoreV1().Pods(o.Namespace).Get(context.TODO(), o.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// cluster name is not specified, get from pod label
	if o.clusterName == "" {
		if name, ok := pod.Annotations[constant.AppInstanceLabelKey]; !ok {
			return fmt.Errorf("failed to find the cluster to which the instance belongs")
		} else {
			o.clusterName = name
		}
	}
	var command string
	switch {
	case len(o.filePath) > 0:
		command = assembleTail(o.logOptions.Follow, o.logOptions.Tail, o.logOptions.LimitBytes) + " " + o.filePath
	case o.isStdoutForContainer():
		{
			// file-path and file-type are unset, output container's stdout & stderr, like kubectl logs
			o.logOptions.RESTClientGetter = o.Factory
			o.logOptions.LogsForObject = polymorphichelpers.LogsForObjectFn
			o.logOptions.Object = pod
			o.logOptions.Options, _ = o.logOptions.ToLogOptions()
		}
	default: // find corresponding file path by file type
		{
			clusterGetter := cluster.ObjectsGetter{
				Client:    o.Client,
				Dynamic:   o.Dynamic,
				Name:      o.clusterName,
				Namespace: o.Namespace,
				GetOptions: cluster.GetOptions{
					WithClusterDef: true,
				},
			}
			obj, err := clusterGetter.Get()
			if err != nil {
				return err
			}
			if command, err = o.createFileTypeCommand(pod, obj); err != nil {
				return err
			}
		}
	}
	o.Command = []string{"/bin/bash", "-c", command}
	o.ContainerName = o.logOptions.Container
	o.Pod = pod
	// hide unnecessary output
	o.Quiet = true
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

// createFileTypeCommand creates command against log file type
func (o *LogsOptions) createFileTypeCommand(pod *corev1.Pod, obj *cluster.ClusterObjects) (string, error) {
	var command string
	componentName, ok := pod.Labels[constant.KBAppComponentLabelKey]
	if !ok {
		return command, fmt.Errorf("get component name from pod labels fail")
	}
	var compDefName string
	for _, comCluster := range obj.Cluster.Spec.ComponentSpecs {
		if strings.EqualFold(comCluster.Name, componentName) {
			compDefName = comCluster.ComponentDefRef
			break
		}
	}
	if len(compDefName) == 0 {
		return command, fmt.Errorf("get pod component definition name in cluster.yaml fail")
	}
	var filePathPattern string
	for _, com := range obj.ClusterDef.Spec.ComponentDefs {
		if strings.EqualFold(com.Name, compDefName) {
			for _, logConfig := range com.LogConfigs {
				if strings.EqualFold(logConfig.Name, o.fileType) {
					filePathPattern = logConfig.FilePathPattern
					break
				}
			}
			break
		}
	}
	if len(filePathPattern) > 0 {
		command = "ls " + filePathPattern + " | xargs " + assembleTail(o.logOptions.Follow, o.logOptions.Tail, o.logOptions.LimitBytes)
	} else {
		return command, fmt.Errorf("can't get file path pattern by type %s", o.fileType)
	}
	return command, nil
}

// assembleCommand assembles tail command for log file
func assembleTail(follow bool, tail int64, limitBytes int64) string {
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
	return strings.Join(command, " ")
}

func (o *LogsOptions) isStdoutForContainer() bool {
	if len(o.filePath) == 0 {
		return len(o.fileType) == 0 || strings.EqualFold(o.fileType, "stdout") || strings.EqualFold(o.fileType, "stderr")
	}
	return false
}

// runLogs retrieves stdout/stderr logs
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
