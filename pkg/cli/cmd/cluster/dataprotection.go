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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/create"
	"github.com/apecloud/kubeblocks/pkg/cli/delete"
	"github.com/apecloud/kubeblocks/pkg/cli/list"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

var (
	listBackupPolicyExample = templates.Examples(`
		# list all backup policies
		kbcli cluster list-backup-policy

		# using short cmd to list backup policy of the specified cluster
        kbcli cluster list-bp mycluster
	`)
	editExample = templates.Examples(`
		# edit backup policy
		kbcli cluster edit-backup-policy <backup-policy-name>

        # enable pitr
		kbcli cluster edit-backup-policy <backup-policy-name> --set schedule.logfile.enable=true

	    # using short cmd to edit backup policy
        kbcli cluster edit-bp <backup-policy-name>
	`)
	createBackupExample = templates.Examples(`
		# create a backup, the default type is snapshot.
		kbcli cluster backup mycluster

		# create a snapshot backup
		# create a snapshot of the cluster's persistent volume for backup
		kbcli cluster backup mycluster --type snapshot

		# create a datafile backup
		# backup all files under the data directory and save them to the specified storage, only full backup is supported now.
		kbcli cluster backup mycluster --type datafile 

		# create a backup with specified backup policy
		kbcli cluster backup mycluster --policy <backup-policy-name>
	`)
	listBackupExample = templates.Examples(`
		# list all backups
		kbcli cluster list-backups
	`)
	deleteBackupExample = templates.Examples(`
		# delete a backup named backup-name
		kbcli cluster delete-backup cluster-name --name backup-name
	`)
	createRestoreExample = templates.Examples(`
		# restore a new cluster from a backup
		kbcli cluster restore new-cluster-name --backup backup-name

		# restore a new cluster from point in time
		kbcli cluster restore new-cluster-name --restore-to-time "Apr 13,2023 18:40:35 UTC+0800" --backup logfile-backup
        kbcli cluster restore new-cluster-name --restore-to-time "2023-04-13T18:40:35+08:00" --backup logfile-backup
	`)
	describeBackupExample = templates.Examples(`
		# describe a backup
		kbcli cluster describe-backup backup-default-mycluster-20230616190023
	`)
	describeBackupPolicyExample = templates.Examples(`
		# describe a backup policy
		kbcli cluster describe-backup-policy mycluster-mysql-backup-policy
	`)
)

const annotationTrueValue = "true"

type CreateBackupOptions struct {
	BackupMethod string `json:"backupMethod"`
	BackupName   string `json:"backupName"`
	Role         string `json:"role,omitempty"`
	BackupPolicy string `json:"backupPolicy"`

	create.CreateOptions `json:"-"`
}

type ListBackupOptions struct {
	*list.ListOptions
	BackupName string
}

type DescribeBackupOptions struct {
	Factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	// resource type and names
	Gvr   schema.GroupVersionResource
	names []string

	genericiooptions.IOStreams
}

func (o *CreateBackupOptions) CompleteBackup() error {
	if err := o.Complete(); err != nil {
		return err
	}
	// generate backupName
	if len(o.BackupName) == 0 {
		o.BackupName = strings.Join([]string{"backup", o.Namespace, o.Name, time.Now().Format("20060102150405")}, "-")
	}

	return o.CreateOptions.Complete()
}

func (o *CreateBackupOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	if o.BackupPolicy == "" {
		if err := o.completeDefaultBackupPolicy(); err != nil {
			return err
		}
	} else {
		// check if backup policy exists
		if _, err := o.Dynamic.Resource(types.BackupPolicyGVR()).Namespace(o.Namespace).Get(context.TODO(), o.BackupPolicy, metav1.GetOptions{}); err != nil {
			return err
		}
	}
	if o.BackupMethod == "" {
		// TODO(ldm): if backup policy only has one backup method, use it as default
		//  backup method.
		return fmt.Errorf("missing backup method")
	}
	// TODO: check if pvc exists
	return nil
}

// completeDefaultBackupPolicy completes the default backup policy.
func (o *CreateBackupOptions) completeDefaultBackupPolicy() error {
	defaultBackupPolicyName, err := o.getDefaultBackupPolicy()
	if err != nil {
		return err
	}
	o.BackupPolicy = defaultBackupPolicyName
	return nil
}

func (o *CreateBackupOptions) getDefaultBackupPolicy() (string, error) {
	clusterObj, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// TODO: support multiple components backup, add --componentDef flag
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s",
			constant.AppInstanceLabelKey, clusterObj.GetName()),
	}
	objs, err := o.Dynamic.
		Resource(types.BackupPolicyGVR()).Namespace(o.Namespace).
		List(context.TODO(), opts)
	if err != nil {
		return "", err
	}
	if len(objs.Items) == 0 {
		return "", fmt.Errorf(`not found any backup policy for cluster "%s"`, o.Name)
	}
	var defaultBackupPolicies []unstructured.Unstructured
	for _, obj := range objs.Items {
		if obj.GetAnnotations()[dptypes.DefaultBackupPolicyAnnotationKey] == annotationTrueValue {
			defaultBackupPolicies = append(defaultBackupPolicies, obj)
		}
	}
	if len(defaultBackupPolicies) == 0 {
		return "", fmt.Errorf(`not found any default backup policy for cluster "%s"`, o.Name)
	}
	if len(defaultBackupPolicies) > 1 {
		return "", fmt.Errorf(`cluster "%s" has multiple default backup policies`, o.Name)
	}
	return defaultBackupPolicies[0].GetName(), nil
}

func NewCreateBackupCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("Backup %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli cluster list-backups --name=%s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}

	o := &CreateBackupOptions{
		CreateOptions: create.CreateOptions{
			IOStreams:       streams,
			Factory:         f,
			GVR:             types.BackupGVR(),
			CueTemplateName: "backup_template.cue",
			CustomOutPut:    customOutPut,
		},
	}
	o.CreateOptions.Options = o

	cmd := &cobra.Command{
		Use:               "backup NAME",
		Short:             "Create a backup for the cluster.",
		Example:           createBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.CompleteBackup())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.BackupMethod, "method", "", "Backup method that defined in backup policy (required)")
	cmd.Flags().StringVar(&o.BackupName, "name", "", "Backup name")
	cmd.Flags().StringVar(&o.BackupPolicy, "policy", "", "Backup policy name, this flag will be ignored when backup-type is snapshot")

	return cmd
}

func PrintBackupList(o ListBackupOptions) error {
	var backupNameMap = make(map[string]bool)
	for _, name := range o.Names {
		backupNameMap[name] = true
	}

	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		if o.BackupName != "" {
			o.Names = []string{o.BackupName}
		}
		_, err := o.Run()
		return err
	}
	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	if o.AllNamespaces {
		o.Namespace = ""
	}
	backupList, err := dynamic.Resource(types.BackupGVR()).Namespace(o.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: o.LabelSelector,
		FieldSelector: o.FieldSelector,
	})
	if err != nil {
		return err
	}

	if len(backupList.Items) == 0 {
		o.PrintNotFoundResources()
		return nil
	}

	// sort the unstructured objects with the creationTimestamp in positive order
	sort.Sort(unstructuredList(backupList.Items))
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "NAMESPACE", "SOURCE-CLUSTER", "METHOD", "STATUS", "TOTAL-SIZE", "DURATION", "CREATE-TIME", "COMPLETION-TIME", "EXPIRATION")
	for _, obj := range backupList.Items {
		backup := &dpv1alpha1.Backup{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, backup); err != nil {
			return err
		}
		// TODO(ldm): find cluster from backup policy target spec.
		sourceCluster := backup.Labels[constant.AppInstanceLabelKey]
		durationStr := ""
		if backup.Status.Duration != nil {
			durationStr = duration.HumanDuration(backup.Status.Duration.Duration)
		}
		statusString := string(backup.Status.Phase)
		if len(o.Names) > 0 && !backupNameMap[backup.Name] {
			continue
		}
		tbl.AddRow(backup.Name, backup.Namespace, sourceCluster, backup.Spec.BackupMethod, statusString, backup.Status.TotalSize,
			durationStr, util.TimeFormat(&backup.CreationTimestamp), util.TimeFormat(backup.Status.CompletionTimestamp),
			util.TimeFormat(backup.Status.Expiration))
	}
	tbl.Print()
	return nil
}

func NewListBackupCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &ListBackupOptions{ListOptions: list.NewListOptions(f, streams, types.BackupGVR())}
	cmd := &cobra.Command{
		Use:               "list-backups",
		Short:             "List backups.",
		Aliases:           []string{"ls-backups"},
		Example:           listBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			if o.BackupName != "" {
				o.Names = []string{o.BackupName}
			}
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete())
			util.CheckErr(PrintBackupList(*o))
		},
	}
	o.AddFlags(cmd)
	cmd.Flags().StringVar(&o.BackupName, "name", "", "The backup name to get the details.")
	return cmd
}

func NewDescribeBackupCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &DescribeBackupOptions{
		Factory:   f,
		IOStreams: streams,
		Gvr:       types.BackupGVR(),
	}
	cmd := &cobra.Command{
		Use:               "describe-backup BACKUP-NAME",
		Short:             "Describe a backup.",
		Aliases:           []string{"desc-backup"},
		Example:           describeBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete(args))
			util.CheckErr(o.Run())
		},
	}
	return cmd
}

func NewDeleteBackupCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.BackupGVR())
	cmd := &cobra.Command{
		Use:               "delete-backup",
		Short:             "Delete a backup.",
		Example:           deleteBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(completeForDeleteBackup(o, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringSliceVar(&o.Names, "name", []string{}, "Backup names")
	o.AddFlags(cmd)
	return cmd
}

// completeForDeleteBackup completes cmd for delete backup
func completeForDeleteBackup(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("Missing cluster name")
	}
	if len(args) > 1 {
		return errors.New("Only supported delete the Backup of one cluster")
	}
	if !o.Force && len(o.Names) == 0 {
		return errors.New("Missing --name as backup name.")
	}
	if o.Force && len(o.Names) == 0 {
		// do force action, for --force and --name unset, delete all backups of the cluster
		// if backup name unset and cluster name set, delete all backups of the cluster
		o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
		o.ConfirmedNames = args
	}
	o.ConfirmedNames = o.Names
	return nil
}

type CreateRestoreOptions struct {
	// backup name to restore in creation
	Backup string `json:"backup,omitempty"`

	// point in time recovery args
	RestoreTime             *time.Time `json:"restoreTime,omitempty"`
	RestoreTimeStr          string     `json:"restoreTimeStr,omitempty"`
	RestoreManagementPolicy string     `json:"volumeRestorePolicy,omitempty"`

	create.CreateOptions `json:"-"`
}

func (o *CreateRestoreOptions) getClusterObject(backup *dpv1alpha1.Backup) (*appsv1alpha1.Cluster, error) {
	// use the cluster snapshot to restore firstly
	clusterString, ok := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
	if ok {
		clusterObj := &appsv1alpha1.Cluster{}
		if err := json.Unmarshal([]byte(clusterString), &clusterObj); err != nil {
			return nil, err
		}
		return clusterObj, nil
	}
	clusterName := backup.Labels[constant.AppInstanceLabelKey]
	return cluster.GetClusterByName(o.Dynamic, clusterName, o.Namespace)
}

func (o *CreateRestoreOptions) Run() error {
	if o.Backup != "" {
		return o.runRestoreFromBackup()
	}
	return nil
}

func (o *CreateRestoreOptions) runRestoreFromBackup() error {
	// get backup
	backup := &dpv1alpha1.Backup{}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, o.Backup); err != nil {
		return err
	}
	if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
		return errors.Errorf(`backup "%s" is not completed.`, backup.Name)
	}
	if len(backup.Labels[constant.AppInstanceLabelKey]) == 0 {
		return errors.Errorf(`missing source cluster in backup "%s", "app.kubernetes.io/instance" is empty in labels.`, o.Backup)
	}

	restoreTimeStr, err := formatRestoreTimeAndValidate(o.RestoreTimeStr, backup)
	if err != nil {
		return err
	}
	// get the cluster object and set the annotation for restore
	clusterObj, err := o.getClusterObject(backup)
	if err != nil {
		return err
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, o.RestoreManagementPolicy, len(clusterObj.Spec.ComponentSpecs), clusterObj.Spec.ComponentSpecs[0].Name, restoreTimeStr)
	if err != nil {
		return err
	}
	clusterObj.ObjectMeta = metav1.ObjectMeta{
		Namespace:   clusterObj.Namespace,
		Name:        o.Name,
		Annotations: map[string]string{constant.RestoreFromBackupAnnotationKey: restoreAnnotation},
	}
	return o.createCluster(clusterObj)
}

func (o *CreateRestoreOptions) createCluster(cluster *appsv1alpha1.Cluster) error {
	clusterGVR := types.ClusterGVR()
	cluster.Status = appsv1alpha1.ClusterStatus{}
	cluster.TypeMeta = metav1.TypeMeta{
		Kind:       types.KindCluster,
		APIVersion: clusterGVR.Group + "/" + clusterGVR.Version,
	}
	// convert the cluster object and create it.
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cluster)
	if err != nil {
		return err
	}
	unstructuredObj := &unstructured.Unstructured{Object: unstructuredMap}
	if unstructuredObj, err = o.Dynamic.Resource(clusterGVR).Namespace(o.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
		return err
	}
	if !o.Quiet {
		fmt.Fprintf(o.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}
	return nil
}

func isTimeInRange(t time.Time, start time.Time, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}

func (o *CreateRestoreOptions) Validate() error {
	if o.Backup == "" {
		return fmt.Errorf("must be specified one of the --backup ")
	}

	if o.Name == "" {
		name, err := generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("failed to generate a random cluster name")
		}
		o.Name = name
	}
	return nil
}

func NewCreateRestoreCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &CreateRestoreOptions{}
	o.CreateOptions = create.CreateOptions{
		IOStreams: streams,
		Factory:   f,
		Options:   o,
	}

	cmd := &cobra.Command{
		Use:     "restore",
		Short:   "Restore a new cluster from backup.",
		Example: createRestoreExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete())
			util.CheckErr(o.Validate())
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.Backup, "backup", "", "Backup name")
	cmd.Flags().StringVar(&o.RestoreTimeStr, "restore-to-time", "", "point in time recovery(PITR)")
	cmd.Flags().StringVar(&o.RestoreManagementPolicy, "volume-restore-policy", "Parallel", "the volume claim restore policy, supported values: [Serial, Parallel]")
	return cmd
}

func NewListBackupPolicyCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.BackupPolicyGVR())
	cmd := &cobra.Command{
		Use:               "list-backup-policy",
		Short:             "List backups policies.",
		Aliases:           []string{"list-bp"},
		Example:           listBackupPolicyExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			o.Names = nil
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete())
			util.CheckErr(printBackupPolicyList(*o))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

// printBackupPolicyList prints the backup policy list.
func printBackupPolicyList(o list.ListOptions) error {
	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		_, err := o.Run()
		return err
	}
	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	if o.AllNamespaces {
		o.Namespace = ""
	}
	backupPolicyList, err := dynamic.Resource(types.BackupPolicyGVR()).Namespace(o.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: o.LabelSelector,
		FieldSelector: o.FieldSelector,
	})
	if err != nil {
		return err
	}

	if len(backupPolicyList.Items) == 0 {
		o.PrintNotFoundResources()
		return nil
	}

	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "NAMESPACE", "DEFAULT", "CLUSTER", "CREATE-TIME", "STATUS")
	for _, obj := range backupPolicyList.Items {
		defaultPolicy, ok := obj.GetAnnotations()[dptypes.DefaultBackupPolicyAnnotationKey]
		backupPolicy := &dpv1alpha1.BackupPolicy{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, backupPolicy); err != nil {
			return err
		}
		if !ok {
			defaultPolicy = "false"
		}
		createTime := obj.GetCreationTimestamp()
		tbl.AddRow(obj.GetName(), obj.GetNamespace(), defaultPolicy, obj.GetLabels()[constant.AppInstanceLabelKey],
			util.TimeFormat(&createTime), backupPolicy.Status.Phase)
	}
	tbl.Print()
	return nil
}

type updateBackupPolicyFieldFunc func(backupPolicy *dpv1alpha1.BackupPolicy, targetVal string) error

type editBackupPolicyOptions struct {
	namespace string
	name      string
	dynamic   dynamic.Interface
	Factory   cmdutil.Factory

	GVR schema.GroupVersionResource
	genericiooptions.IOStreams
	editContent       []editorRow
	editContentKeyMap map[string]updateBackupPolicyFieldFunc
	original          string
	target            string
	values            []string
	isTest            bool
}

type editorRow struct {
	// key content key (required).
	key string
	// value jsonpath for backupPolicy.spec.
	jsonpath string
	// updateFunc applies the modified value to backupPolicy (required).
	updateFunc updateBackupPolicyFieldFunc
}

func NewEditBackupPolicyCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := editBackupPolicyOptions{Factory: f, IOStreams: streams, GVR: types.BackupPolicyGVR()}
	cmd := &cobra.Command{
		Use:                   "edit-backup-policy",
		DisableFlagsInUseLine: true,
		Aliases:               []string{"edit-bp"},
		Short:                 "Edit backup policy",
		Example:               editExample,
		ValidArgsFunction:     util.ResourceNameCompletionFunc(f, types.BackupPolicyGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.runEditBackupPolicy())
		},
	}
	cmd.Flags().StringArrayVar(&o.values, "set", []string{},
		"set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	return cmd
}

func (o *editBackupPolicyOptions) complete(args []string) error {
	var err error
	if len(args) == 0 {
		return fmt.Errorf("missing backupPolicy name")
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to update one backupPolicy or quote cronExpression")
	}
	o.name = args[0]
	if o.namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}
	updateRepoName := func(backupPolicy *dpv1alpha1.BackupPolicy, targetVal string) error {
		// check if the backup repo exists
		if targetVal != "" {
			_, err := o.dynamic.Resource(types.BackupRepoGVR()).Get(context.Background(), targetVal, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
		if backupPolicy != nil {
			if targetVal != "" {
				backupPolicy.Spec.BackupRepoName = &targetVal
			} else {
				backupPolicy.Spec.BackupRepoName = nil
			}
		}
		return nil
	}

	o.editContent = []editorRow{
		{
			key:      "backupRepoName",
			jsonpath: "backupRepoName",
			updateFunc: func(backupPolicy *dpv1alpha1.BackupPolicy, targetVal string) error {
				return updateRepoName(backupPolicy, targetVal)
			},
		},
	}
	o.editContentKeyMap = map[string]updateBackupPolicyFieldFunc{}
	for _, v := range o.editContent {
		if v.updateFunc == nil {
			return fmt.Errorf("updateFunc can not be nil")
		}
		o.editContentKeyMap[v.key] = v.updateFunc
	}
	return nil
}

func (o *editBackupPolicyOptions) runEditBackupPolicy() error {
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	key := client.ObjectKey{
		Name:      o.name,
		Namespace: o.namespace,
	}
	err := util.GetResourceObjectFromGVR(types.BackupPolicyGVR(), key, o.dynamic, &backupPolicy)
	if err != nil {
		return err
	}
	if len(o.values) == 0 {
		edited, err := o.runWithEditor(backupPolicy)
		if err != nil {
			return err
		}
		o.values = strings.Split(edited, "\n")
	}
	return o.applyChanges(backupPolicy)
}

func (o *editBackupPolicyOptions) runWithEditor(backupPolicy *dpv1alpha1.BackupPolicy) (string, error) {
	editor := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})
	contents, err := o.buildEditorContent(backupPolicy)
	if err != nil {
		return "", err
	}
	addHeader := func() string {
		return fmt.Sprintf(`# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
%s
`, *contents)
	}
	if o.isTest {
		// only for testing
		return "", nil
	}
	edited, _, err := editor.LaunchTempFile(fmt.Sprintf("%s-edit-", backupPolicy.Name), "", bytes.NewBufferString(addHeader()))
	if err != nil {
		return "", err
	}
	return string(edited), nil
}

// buildEditorContent builds the editor content.
func (o *editBackupPolicyOptions) buildEditorContent(backPolicy *dpv1alpha1.BackupPolicy) (*string, error) {
	var contents []string
	for _, v := range o.editContent {
		// get the value with jsonpath
		val, err := o.getValueWithJsonpath(backPolicy.Spec, v.jsonpath)
		if err != nil {
			return nil, err
		}
		if val == nil {
			continue
		}
		row := fmt.Sprintf("%s=%s", v.key, *val)
		o.original += row
		contents = append(contents, row)
	}
	result := strings.Join(contents, "\n")
	return &result, nil
}

// getValueWithJsonpath gets the value with jsonpath.
func (o *editBackupPolicyOptions) getValueWithJsonpath(spec dpv1alpha1.BackupPolicySpec, path string) (*string, error) {
	parser := jsonpath.New("edit-backup-policy").AllowMissingKeys(true)
	pathExpression, err := get.RelaxedJSONPathExpression(path)
	if err != nil {
		return nil, err
	}
	if err = parser.Parse(pathExpression); err != nil {
		return nil, err
	}
	values, err := parser.FindResults(spec)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		if len(v) == 0 {
			continue
		}
		v1 := v[0]
		switch v1.Kind() {
		case reflect.Ptr, reflect.Interface:
			if v1.IsNil() {
				return nil, nil
			}
			val := fmt.Sprintf("%v", v1.Elem())
			return &val, nil
		default:
			val := fmt.Sprintf("%v", v1.Interface())
			return &val, nil
		}
	}
	return nil, nil
}

// applyChanges applies the changes of backupPolicy.
func (o *editBackupPolicyOptions) applyChanges(backupPolicy *dpv1alpha1.BackupPolicy) error {
	for _, v := range o.values {
		row := strings.TrimSpace(v)
		if strings.HasPrefix(row, "#") || row == "" {
			continue
		}
		o.target += row
		arr := strings.Split(row, "=")
		if len(arr) != 2 {
			return fmt.Errorf(`invalid row: %s, format should be "key=value"`, v)
		}
		updateFn, ok := o.editContentKeyMap[arr[0]]
		if !ok {
			return fmt.Errorf(`invalid key: %s`, arr[0])
		}
		arr[1] = strings.Trim(arr[1], `"`)
		arr[1] = strings.Trim(arr[1], `'`)
		if err := updateFn(backupPolicy, arr[1]); err != nil {
			return err
		}
	}
	// if no changes, return.
	if o.original == o.target {
		fmt.Fprintln(o.Out, "updated (no change)")
		return nil
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(backupPolicy)
	if err != nil {
		return err
	}
	if _, err = o.dynamic.Resource(types.BackupPolicyGVR()).Namespace(backupPolicy.Namespace).Update(context.TODO(),
		&unstructured.Unstructured{Object: obj}, metav1.UpdateOptions{}); err != nil {
		return err
	}
	fmt.Fprintln(o.Out, "updated")
	return nil
}

type describeBackupPolicyOptions struct {
	namespace string
	names     []string
	dynamic   dynamic.Interface
	Factory   cmdutil.Factory
	client    clientset.Interface

	genericiooptions.IOStreams
}

func (o *describeBackupPolicyOptions) Complete(args []string) error {
	var err error

	if len(args) == 0 {
		return fmt.Errorf("backupPolicy name should be specified")
	}

	o.names = args

	if o.client, err = o.Factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}

	if o.namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	return nil
}

func (o *describeBackupPolicyOptions) Run() error {
	for _, name := range o.names {
		backupPolicyObj := &dpv1alpha1.BackupPolicy{}
		if err := cluster.GetK8SClientObject(o.dynamic, backupPolicyObj, types.BackupPolicyGVR(), o.namespace, name); err != nil {
			return err
		}
		if err := o.printBackupPolicyObj(backupPolicyObj); err != nil {
			return err
		}
	}
	return nil
}

func (o *describeBackupPolicyOptions) printBackupPolicyObj(obj *dpv1alpha1.BackupPolicy) error {
	printer.PrintLine("Summary:")
	realPrintPairStringToLine("Name", obj.Name)
	realPrintPairStringToLine("Cluster", obj.Labels[constant.AppInstanceLabelKey])
	realPrintPairStringToLine("Namespace", obj.Namespace)
	if obj.Spec.BackupRepoName != nil {
		realPrintPairStringToLine("Backup Repo Name", *obj.Spec.BackupRepoName)
	}

	printer.PrintLine("\nBackup Methods:")
	p := printer.NewTablePrinter(o.Out)
	p.SetHeader("Name", "ActionSet", "snapshot-volumes")
	for _, v := range obj.Spec.BackupMethods {
		p.AddRow(v.Name, v.ActionSetName, strconv.FormatBool(*v.SnapshotVolumes))
	}
	p.Print()

	return nil
}

func NewDescribeBackupPolicyCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &describeBackupPolicyOptions{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:                   "describe-backup-policy",
		DisableFlagsInUseLine: true,
		Aliases:               []string{"describe-bp"},
		Short:                 "Describe backup policy",
		Example:               describeBackupPolicyExample,
		ValidArgsFunction:     util.ResourceNameCompletionFunc(f, types.BackupPolicyGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete(args))
			util.CheckErr(o.Run())
		},
	}
	return cmd
}

func (o *DescribeBackupOptions) Complete(args []string) error {
	var err error

	if len(args) == 0 {
		return fmt.Errorf("backup name should be specified")
	}

	o.names = args

	if o.client, err = o.Factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}

	if o.namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	return nil
}

func (o *DescribeBackupOptions) Run() error {
	for _, name := range o.names {
		backupObj := &dpv1alpha1.Backup{}
		if err := cluster.GetK8SClientObject(o.dynamic, backupObj, o.Gvr, o.namespace, name); err != nil {
			return err
		}
		if err := o.printBackupObj(backupObj); err != nil {
			return err
		}
	}
	return nil
}

func (o *DescribeBackupOptions) printBackupObj(obj *dpv1alpha1.Backup) error {
	targetCluster := obj.Labels[constant.AppInstanceLabelKey]
	printer.PrintLineWithTabSeparator(
		printer.NewPair("Name", obj.Name),
		printer.NewPair("Cluster", targetCluster),
		printer.NewPair("Namespace", obj.Namespace),
	)
	printer.PrintLine("\nSpec:")
	realPrintPairStringToLine("Method", obj.Spec.BackupMethod)
	realPrintPairStringToLine("Policy Name", obj.Spec.BackupPolicyName)

	printer.PrintLine("\nStatus:")
	realPrintPairStringToLine("Phase", string(obj.Status.Phase))
	realPrintPairStringToLine("Total Size", obj.Status.TotalSize)
	if obj.Status.BackupMethod != nil {
		realPrintPairStringToLine("ActionSet Name", obj.Status.BackupMethod.ActionSetName)
	}
	if obj.Status.BackupRepoName != "" {
		realPrintPairStringToLine("Repository", obj.Status.BackupRepoName)
	}
	if obj.Status.PersistentVolumeClaimName != "" {
		realPrintPairStringToLine("PVC Name", obj.Status.PersistentVolumeClaimName)
	}
	if obj.Status.Duration != nil {
		realPrintPairStringToLine("Duration", duration.HumanDuration(obj.Status.Duration.Duration))
	}
	realPrintPairStringToLine("Expiration Time", util.TimeFormat(obj.Status.Expiration))
	realPrintPairStringToLine("Start Time", util.TimeFormat(obj.Status.StartTimestamp))
	realPrintPairStringToLine("Completion Time", util.TimeFormat(obj.Status.CompletionTimestamp))
	// print failure reason, ignore error
	_ = o.enhancePrintFailureReason(obj.Name, obj.Status.FailureReason)

	realPrintPairStringToLine("Path", obj.Status.Path)

	if obj.Status.TimeRange != nil {
		realPrintPairStringToLine("Time Range Start", util.TimeFormat(obj.Status.TimeRange.Start))
		realPrintPairStringToLine("Time Range End", util.TimeFormat(obj.Status.TimeRange.End))
	}

	if len(obj.Status.VolumeSnapshots) > 0 {
		printer.PrintLine("\nVolume Snapshots:")
		for _, v := range obj.Status.VolumeSnapshots {
			realPrintPairStringToLine("Name", v.Name)
			realPrintPairStringToLine("Content Name", v.ContentName)
			realPrintPairStringToLine("Volume Name:", v.VolumeName)
			realPrintPairStringToLine("Size", v.Size)
		}
	}

	// get all events about backup
	events, err := o.client.CoreV1().Events(o.namespace).Search(scheme.Scheme, obj)
	if err != nil {
		return err
	}

	// print the warning events
	printer.PrintAllWarningEvents(events, o.Out)

	return nil
}

func realPrintPairStringToLine(name, value string, spaceCount ...int) {
	if value != "" {
		printer.PrintPairStringToLine(name, value, spaceCount...)
	}
}

// print the pod error logs if failure reason has occurred
// TODO: the failure reason should be improved in the backup controller
func (o *DescribeBackupOptions) enhancePrintFailureReason(backupName, failureReason string, spaceCount ...int) error {
	if failureReason == "" {
		return nil
	}
	ctx := context.Background()
	// get the latest job log details.
	labels := fmt.Sprintf("%s=%s",
		dptypes.DataProtectionLabelBackupNameKey, backupName,
	)
	jobList, err := o.client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return err
	}
	var failedJob *batchv1.Job
	for _, i := range jobList.Items {
		if i.Status.Failed > 0 {
			failedJob = &i
			break
		}
	}
	if failedJob != nil {
		podLabels := fmt.Sprintf("%s=%s",
			"controller-uid", failedJob.UID,
		)
		podList, err := o.client.CoreV1().Pods(failedJob.Namespace).List(ctx, metav1.ListOptions{LabelSelector: podLabels})
		if err != nil {
			return err
		}
		if len(podList.Items) > 0 {
			tailLines := int64(5)
			req := o.client.CoreV1().
				Pods(podList.Items[0].Namespace).
				GetLogs(podList.Items[0].Name, &corev1.PodLogOptions{TailLines: &tailLines})
			data, err := req.DoRaw(ctx)
			if err != nil {
				return err
			}
			failureReason = fmt.Sprintf("%s\n pod %s error logs:\n%s",
				failureReason, podList.Items[0].Name, string(data))
		}
	}
	printer.PrintPairStringToLine("Failure Reason", failureReason, spaceCount...)

	return nil
}
