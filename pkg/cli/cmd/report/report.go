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

package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/strings/slices"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	clischeme "github.com/apecloud/kubeblocks/pkg/cli/scheme"
	"github.com/apecloud/kubeblocks/pkg/cli/spinner"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	cliutil "github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	versionFile     = "version.txt"
	manifestsFolder = "manifests"
	eventsFolder    = "events"
	logsFolder      = "logs"

	kubeBlocksReport = "kubeblocks"
	clusterReport    = "cluster"
)

var (
	reportClusterExamples = templates.Examples(`
	# report KubeBlocks status
	kbcli report cluster mycluster

	# report KubeBlocks cluster information to file
	kbcli report cluster mycluster -f filename

	# report KubeBlocks cluster information with logs
	kbcli report cluster mycluster --with-logs

	# report KubeBlocks cluster information with logs and mask sensitive info
	kbcli report cluster mycluster --with-logs --mask

	# report KubeBlocks cluster information with logs since 1 hour ago
	kbcli report cluster mycluster --with-logs --since 1h

	# report KubeBlocks cluster information with logs since given time
	kbcli report cluster mycluster --with-logs --since-time 2023-05-23T00:00:00Z

	# report KubeBlocks cluster information with logs for all containers
	kbcli report cluster mycluster --with-logs --all-containers
	`)

	reportKBExamples = templates.Examples(`
	# report KubeBlocks status
	kbcli report kubeblocks

	# report KubeBlocks information to file
	kbcli report kubeblocks -f filename

	# report KubeBlocks information with logs
	kbcli report kubeblocks --with-logs

	# report KubeBlocks information with logs and mask sensitive info
	kbcli report kubeblocks --with-logs --mask
	`)
)

var _ reportInterface = &reportKubeblocksOptions{}
var _ reportInterface = &reportClusterOptions{}

// NewReportCmd creates command for reports.
func NewReportCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report [kubeblocks | cluster]",
		Short: "report kubeblocks or cluster info.",
	}
	cmd.AddCommand(
		newKubeblocksReportCmd(f, streams),
		newClusterReportCmd(f, streams),
	)
	return cmd
}

type reportInterface interface {
	handleEvents(ctx context.Context) error
	handleLogs(ctx context.Context) error
	handleManifests(ctx context.Context) error
}
type reportOptions struct {
	genericiooptions.IOStreams
	// file name to output
	file string
	// namespace of resource
	namespace string
	// include withLogs or not
	withLogs bool
	// followings flags are for logs
	// all containers or main container only
	allContainers bool
	// since time
	sinceTime string
	// since second
	sinceDuration time.Duration
	// log options
	logOptions *corev1.PodLogOptions
	// various clients
	genericClientSet *genericClientSet
	// enableMask sensitive info or not
	mask bool
	// resource printer, default to YAML printer without managed fields
	resourcePrinter printers.ResourcePrinterFunc
	// JSONYamlPrintFlags is used to print JSON or YAML
	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	// outpout format	, default to YAML
	outputFormat string
	// reportWritter is used to write report to file
	reportWritter reportWritter
}

type reportKubeblocksOptions struct {
	reportOptions
	kubeBlocksSelector metav1.ListOptions
}

type reportClusterOptions struct {
	reportOptions
	clusterName     string
	clusterSelector metav1.ListOptions
	cluster         *appsv1alpha1.Cluster
}

func newReportOptions(f genericiooptions.IOStreams) reportOptions {
	return reportOptions{
		IOStreams:          f,
		JSONYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
	}
}

func (o *reportOptions) complete(f cmdutil.Factory) error {
	var err error

	if o.genericClientSet, err = NewGenericClientSet(f); err != nil {
		return err
	}

	// complete log options
	if o.logOptions, err = o.toLogOptions(); err != nil {
		return err
	}

	// complete printer
	if o.resourcePrinter, err = o.parsePrinter(); err != nil {
		return err
	}

	o.reportWritter = &reportZipWritter{}
	return nil
}

func (o *reportOptions) validate() error {
	// make sure sinceTime and sinceSeconds are not both set
	if len(o.sinceTime) > 0 && o.sinceDuration != 0 {
		return fmt.Errorf("only one of --since-time / --since may be used")
	}
	if (!o.withLogs) && (len(o.sinceTime) > 0 || o.sinceDuration != 0 || o.allContainers) {
		return fmt.Errorf("--since-time / --since / --all-contaiiners can only be used when --with-logs is set")
	}
	o.outputFormat = strings.ToLower(o.outputFormat)
	if slices.Index(o.JSONYamlPrintFlags.AllowedFormats(), o.outputFormat) == -1 {
		return fmt.Errorf("output format %s is not supported", o.outputFormat)
	}
	return nil
}

func (o *reportOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.file, "file", "f", "", "zip file for output")
	cmd.Flags().BoolVar(&o.mask, "mask", true, "mask sensitive info for secrets and configmaps")
	cmd.Flags().BoolVar(&o.withLogs, "with-logs", false, "include pod logs")
	cmd.Flags().BoolVar(&o.allContainers, "all-containers", o.allContainers, "Get all containers' logs in the pod(s). Byt default, only the main container (the first container) will have logs recorded.")
	cmd.Flags().StringVar(&o.sinceTime, "since-time", o.sinceTime, i18n.T("Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used."))
	cmd.Flags().DurationVar(&o.sinceDuration, "since", o.sinceDuration, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.")

	cmd.Flags().StringVarP(&o.outputFormat, "output", "o", "json", fmt.Sprintf("Output format. One of: %s.", strings.Join(o.JSONYamlPrintFlags.AllowedFormats(), "|")))
	cmdutil.CheckErr(cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return o.JSONYamlPrintFlags.AllowedFormats(), cobra.ShellCompDirectiveNoFileComp
	}))
}

func (o *reportOptions) toLogOptions() (*corev1.PodLogOptions, error) {
	logOptions := &corev1.PodLogOptions{}

	if len(o.sinceTime) > 0 {
		t, err := util.ParseRFC3339(o.sinceTime, metav1.Now)
		if err != nil {
			return nil, err
		}
		logOptions.SinceTime = &t
	}

	if o.sinceDuration != 0 {
		sec := int64(o.sinceDuration.Round(time.Second).Seconds())
		logOptions.SinceSeconds = &sec
	}

	return logOptions, nil
}

func (o *reportOptions) parsePrinter() (printers.ResourcePrinterFunc, error) {
	var err error
	// by default, use YAML printer without managed fields
	printer, err := o.JSONYamlPrintFlags.ToPrinter(o.outputFormat)
	if err != nil {
		return nil, err
	}
	// wrap printer with typesetter
	typeSetterPrinter := printers.NewTypeSetter(clischeme.Scheme)
	if printer, err = typeSetterPrinter.WrapToPrinter(printer, nil); err != nil {
		return nil, err
	}
	// if mask is enabled, wrap printer with mask printer
	if o.mask {
		printer = &MaskPrinter{Delegate: printer}
	}
	return printer.PrintObj, nil
}

func newKubeblocksReportCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &reportKubeblocksOptions{reportOptions: newReportOptions(streams)}
	cmd := &cobra.Command{
		Use:     "kubeblocks [-f file] [--with-logs] [--mask]",
		Aliases: []string{"kb"},
		Short:   "Report KubeBlocks information, including deployments, events, logs, etc.",
		Args:    cobra.NoArgs,
		Example: reportKBExamples,
		Run: func(cmd *cobra.Command, args []string) {
			cliutil.CheckErr(o.validate())
			cliutil.CheckErr(o.complete(f))
			cliutil.CheckErr(o.run(f, streams))
		},
	}
	o.addFlags(cmd)
	return cmd
}

func (o *reportKubeblocksOptions) complete(f cmdutil.Factory) error {
	if err := o.reportOptions.complete(f); err != nil {
		return err
	}
	o.namespace, _ = cliutil.GetKubeBlocksNamespace(o.genericClientSet.client)
	// complete file name
	o.file = formatReportName(o.file, kubeBlocksReport)
	if exists, _ := cliutil.FileExists(o.file); exists {
		return fmt.Errorf("file already exist will not overwrite")
	}
	// complete kb selector
	o.kubeBlocksSelector = metav1.ListOptions{LabelSelector: buildKubeBlocksSelector()}
	return nil
}

func (o *reportKubeblocksOptions) run(f cmdutil.Factory, streams genericiooptions.IOStreams) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := o.reportWritter.Init(o.file, o.resourcePrinter); err != nil {
		return err
	}
	defer func() {
		if err := o.reportWritter.Close(); err != nil {
			klog.Errorf("close zip file error: %v", err)
		}
	}()

	fmt.Fprintf(o.Out, "reporting KubeBlocks information to %s\n", o.file)

	if err := o.reportWritter.WriteKubeBlocksVersion(versionFile, o.genericClientSet.client); err != nil {
		return err
	}
	if err := o.handleManifests(ctx); err != nil {
		return err
	}
	if err := o.handleEvents(ctx); err != nil {
		return err
	}
	if err := o.handleLogs(ctx); err != nil {
		return err
	}

	return nil
}

func (o *reportKubeblocksOptions) handleManifests(ctx context.Context) error {
	var (
		scopedgvrs = []schema.GroupVersionResource{
			types.DeployGVR(),
			types.StatefulSetGVR(),
			types.ConfigmapGVR(),
			types.SecretGVR(),
			types.ServiceGVR(),
			types.RoleGVR(),
			types.RoleBindingGVR(),
		}

		globalGvrs = []schema.GroupVersionResource{
			types.AddonGVR(),
			types.ClusterDefGVR(),
			types.ClusterRoleGVR(),
			types.ClusterRoleBindingGVR(),
		}
	)
	// write manifest
	s := spinner.New(o.Out, spinnerMsg("processing manifests"))
	defer s.Fail()
	// get namespaced resources
	allErrors := make([]error, 0)
	resourceLists := make([]*unstructured.UnstructuredList, 0)
	resourceLists = append(resourceLists, cliutil.ListResourceByGVR(ctx, o.genericClientSet.dynamic, o.namespace, scopedgvrs, []metav1.ListOptions{o.kubeBlocksSelector}, &allErrors)...)
	// get global resources
	resourceLists = append(resourceLists, cliutil.ListResourceByGVR(ctx, o.genericClientSet.dynamic, metav1.NamespaceAll, globalGvrs, []metav1.ListOptions{o.kubeBlocksSelector}, &allErrors)...)
	// get all storage class
	resourceLists = append(resourceLists, cliutil.ListResourceByGVR(ctx, o.genericClientSet.dynamic, metav1.NamespaceAll, []schema.GroupVersionResource{types.StorageClassGVR()}, []metav1.ListOptions{{}}, &allErrors)...)
	if err := o.reportWritter.WriteObjects(manifestsFolder, resourceLists, o.outputFormat); err != nil {
		return err
	}
	s.Success()
	return utilerrors.NewAggregate(allErrors)
}

func (o *reportKubeblocksOptions) handleEvents(ctx context.Context) error {
	// write events
	s := spinner.New(o.Out, spinnerMsg("processing events"))
	defer s.Fail()

	// get all events under kubeblocks namespace
	if events, err := o.genericClientSet.client.CoreV1().Events(o.namespace).List(ctx, metav1.ListOptions{}); err != nil {
		return err
	} else {
		eventMap := map[string][]corev1.Event{o.namespace + "-kubeblocks": events.Items}
		if err := o.reportWritter.WriteEvents(eventsFolder, eventMap, o.outputFormat); err != nil {
			return err
		}
	}
	s.Success()
	return nil
}

func (o *reportKubeblocksOptions) handleLogs(ctx context.Context) error {
	if !o.withLogs {
		return nil
	}
	s := spinner.New(o.Out, spinnerMsg("process pod logs"))
	defer s.Fail()
	// write logs
	podList, err := o.genericClientSet.client.CoreV1().Pods(o.namespace).List(ctx, o.kubeBlocksSelector)
	if err != nil {
		return err
	}

	if err := o.reportWritter.WriteLogs(logsFolder, ctx, o.genericClientSet.client, podList, *o.logOptions, o.allContainers); err != nil {
		return err
	}
	s.Success()
	return nil
}

func newClusterReportCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &reportClusterOptions{reportOptions: newReportOptions(streams)}

	cmd := &cobra.Command{
		Use:               "cluster NAME [-f file] [-with-logs] [-mask]",
		Short:             "Report Cluster information",
		Example:           reportClusterExamples,
		ValidArgsFunction: cliutil.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cliutil.CheckErr(o.validate(args))
			cliutil.CheckErr(o.complete(f))
			cliutil.CheckErr(o.run(f, streams))
		},
	}
	o.addFlags(cmd)
	return cmd
}

func (o *reportClusterOptions) validate(args []string) error {
	if err := o.reportOptions.validate(); err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("only ONE cluster name is allowed")
	}
	o.clusterName = args[0]
	return nil
}

func (o *reportClusterOptions) complete(f cmdutil.Factory) error {
	var err error
	if err := o.reportOptions.complete(f); err != nil {
		return err
	}
	// update namespace
	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	// complete file name

	o.file = formatReportName(o.file, fmt.Sprintf("%s-%s", clusterReport, o.clusterName))

	if exists, _ := cliutil.FileExists(o.file); exists {
		return fmt.Errorf("file already exist will not overwrite")
	}

	o.clusterSelector = metav1.ListOptions{LabelSelector: buildClusterResourceSelector(o.clusterName)}
	return nil
}

func (o *reportClusterOptions) run(f cmdutil.Factory, streams genericiooptions.IOStreams) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var err error
	// make cluster exists before processing
	if _, err = o.genericClientSet.kbClientSet.AppsV1alpha1().Clusters(o.namespace).Get(ctx, o.clusterName, metav1.GetOptions{}); err != nil {
		return err
	}

	if err := o.reportWritter.Init(o.file, o.resourcePrinter); err != nil {
		return err
	}
	defer func() {
		if err := o.reportWritter.Close(); err != nil {
			klog.Errorf("close zip file error: %v", err)
		}
	}()

	fmt.Fprintf(o.Out, "reporting cluster information to %s\n", o.file)

	if err := o.reportWritter.WriteKubeBlocksVersion(versionFile, o.genericClientSet.client); err != nil {
		return err
	}
	if err := o.handleManifests(ctx); err != nil {
		return err
	}
	if err := o.handleEvents(ctx); err != nil {
		return err
	}
	if err := o.handleLogs(ctx); err != nil {
		return err
	}
	return nil
}

func (o *reportClusterOptions) handleManifests(ctx context.Context) error {
	var (
		scopedgvrs = []schema.GroupVersionResource{
			types.DeployGVR(),
			types.StatefulSetGVR(),
			types.ConfigmapGVR(),
			types.SecretGVR(),
			types.ServiceGVR(),
			types.RoleGVR(),
			types.RoleBindingGVR(),
			types.BackupGVR(),
			types.BackupPolicyGVR(),
			types.BackupScheduleGVR(),
			types.ActionSetGVR(),
			types.RestoreGVR(),
			types.PVCGVR(),
		}
		globalGvrs = []schema.GroupVersionResource{
			types.PVGVR(),
		}
	)

	var err error
	if o.cluster, err = o.genericClientSet.kbClientSet.AppsV1alpha1().Clusters(o.namespace).Get(ctx, o.clusterName, metav1.GetOptions{}); err != nil {
		return err
	}

	// write manifest
	s := spinner.New(o.Out, spinnerMsg("processing manifests"))
	defer s.Fail()

	allErrors := make([]error, 0)
	// get namespaced resources
	resourceLists := make([]*unstructured.UnstructuredList, 0)
	// write manifest
	resourceLists = append(resourceLists, cliutil.ListResourceByGVR(ctx, o.genericClientSet.dynamic, o.namespace, scopedgvrs, []metav1.ListOptions{o.clusterSelector}, &allErrors)...)
	resourceLists = append(resourceLists, cliutil.ListResourceByGVR(ctx, o.genericClientSet.dynamic, metav1.NamespaceAll, globalGvrs, []metav1.ListOptions{o.clusterSelector}, &allErrors)...)
	if err := o.reportWritter.WriteObjects("manifests", resourceLists, o.outputFormat); err != nil {
		return err
	}

	if err := o.reportWritter.WriteSingleObject(manifestsFolder, types.KindCluster, o.cluster.Name, o.cluster, o.outputFormat); err != nil {
		return err
	}

	// get cluster definition
	clusterDefName := o.cluster.Spec.ClusterDefRef
	if clusterDef, err := o.genericClientSet.kbClientSet.AppsV1alpha1().ClusterDefinitions().Get(ctx, clusterDefName, metav1.GetOptions{}); err != nil {
		return err
	} else if err = o.reportWritter.WriteSingleObject(manifestsFolder, types.KindClusterDef, clusterDef.Name, clusterDef, o.outputFormat); err != nil {
		return err
	}

	// get cluster version
	clusterVersionName := o.cluster.Spec.ClusterVersionRef
	if clusterVersion, err := o.genericClientSet.kbClientSet.AppsV1alpha1().ClusterVersions().Get(ctx, clusterVersionName, metav1.GetOptions{}); err != nil {
		return err
	} else if err = o.reportWritter.WriteSingleObject(manifestsFolder, types.KindClusterVersion, clusterVersion.Name, clusterVersion, o.outputFormat); err != nil {
		return err
	}

	s.Success()
	return nil
}

func (o *reportClusterOptions) handleEvents(ctx context.Context) error {
	s := spinner.New(o.Out, spinnerMsg("processing events"))
	defer s.Fail()

	events := make(map[string][]corev1.Event, 0)
	// get all events of cluster
	clusterEvents, err := o.genericClientSet.client.CoreV1().Events(o.namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", o.clusterName)})
	if err != nil {
		return err
	}
	events["cluster-"+o.clusterName] = clusterEvents.Items

	// get all events for pods
	podList, err := o.genericClientSet.client.CoreV1().Pods(o.namespace).List(ctx, o.clusterSelector)
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		if podEvents, err := o.genericClientSet.client.CoreV1().Events(o.namespace).List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name)}); err != nil {
			return err
		} else {
			events["pod-"+pod.Name] = podEvents.Items
		}
	}

	if err = o.reportWritter.WriteEvents(eventsFolder, events, o.outputFormat); err != nil {
		return err
	}
	s.Success()
	return nil
}

func (o *reportClusterOptions) handleLogs(ctx context.Context) error {
	if !o.withLogs {
		return nil
	}

	s := spinner.New(o.Out, spinnerMsg("process pod logs"))
	defer s.Fail()

	// get all events for pods
	if podList, err := o.genericClientSet.client.CoreV1().Pods(o.namespace).List(ctx, o.clusterSelector); err != nil {
		return err
	} else if err := o.reportWritter.WriteLogs(logsFolder, ctx, o.genericClientSet.client, podList, *o.logOptions, o.allContainers); err != nil {
		return err
	}

	s.Success()
	return nil
}

func spinnerMsg(format string, a ...any) spinner.Option {
	return spinner.WithMessage(fmt.Sprintf("%-50s", fmt.Sprintf(format, a...)))
}

func formatReportName(fileName string, kind string) string {
	if len(fileName) > 0 {
		return fileName
	}
	return fmt.Sprintf("report-%s-%s.zip", kind, time.Now().Local().Format("2006-01-02-15-04-05"))
}

func buildClusterResourceSelector(clusterName string) string {
	// app.kubernetes.io/instance: <clusterName>
	// app.kubernetes.io/managed-by: kubeblocks
	return fmt.Sprintf("%s=%s, %s=%s", constant.AppInstanceLabelKey, clusterName, constant.AppManagedByLabelKey, constant.AppName)
}

func buildKubeBlocksSelector() string {
	// app.kubernetes.io/name: kubeblocks
	// app.kubernetes.io/instance: kubeblocks
	return fmt.Sprintf("%s=%s,%s=%s",
		constant.AppInstanceLabelKey, types.KubeBlocksReleaseName,
		constant.AppNameLabelKey, types.KubeBlocksChartName)
}
