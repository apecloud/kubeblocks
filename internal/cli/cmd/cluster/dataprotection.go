/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/edit"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	listBackupPolicyExample = templates.Examples(`
		# list all backup policy
		kbcli cluster list-backup-policy 
        
		# using short cmd to list backup policy of specified cluster 
        kbcli cluster list-bp mycluster
	`)
	editExample = templates.Examples(`
		# edit backup policy
		kbcli cluster edit-backup-policy <backup-policy-name>

	    # using short cmd to edit backup policy 
        kbcli cluster edit-bp <backup-policy-name>
	`)
	createBackupExample = templates.Examples(`
		# create a backup
		kbcli cluster backup cluster-name
	`)
	listBackupExample = templates.Examples(`
		# list all backup
		kbcli cluster list-backups
	`)
	deleteBackupExample = templates.Examples(`
		# delete a backup named backup-name
		kbcli cluster delete-backup cluster-name --name backup-name
	`)
	listRestoreExample = templates.Examples(`
		# list all restore
		kbcli cluster list-restore
	`)
	deleteRestoreExample = templates.Examples(`
		# delete a restore named restore-name
		kbcli cluster delete-restore cluster-name --name restore-name
	`)
	createRestoreExample = templates.Examples(`
		# restore a new cluster from a backup
		kbcli cluster restore new-cluster-name --backup backup-name

		# restore a new cluster from point in time
		kbcli cluster restore new-cluster-name --restore-to-time "Apr 13,2023 18:40:35 UTC+0800" --source-cluster mycluster
	`)
)

const annotationTrueValue = "true"

type CreateBackupOptions struct {
	BackupType   string `json:"backupType"`
	BackupName   string `json:"backupName"`
	Role         string `json:"role,omitempty"`
	BackupPolicy string `json:"backupPolicy"`
	create.BaseOptions
}

type CreateVolumeSnapshotClassOptions struct {
	Driver string `json:"driver"`
	Name   string `json:"name"`
	create.BaseOptions
}

type ListBackupOptions struct {
	*list.ListOptions
	BackupName string
}

func (o *CreateVolumeSnapshotClassOptions) Complete() error {
	objs, err := o.Dynamic.
		Resource(types.StorageClassGVR()).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, sc := range objs.Items {
		annotations := sc.GetAnnotations()
		if annotations == nil {
			continue
		}
		if annotations["storageclass.kubernetes.io/is-default-class"] == annotationTrueValue {
			o.Driver, _, _ = unstructured.NestedString(sc.Object, "provisioner")
			o.Name = "default-vsc"
		}
	}
	// warning if not found default storage class
	if o.Driver == "" {
		return fmt.Errorf("no default StorageClass found, snapshot-controller may not work")
	}
	return nil
}

func (o *CreateVolumeSnapshotClassOptions) Create() error {
	objs, err := o.Dynamic.
		Resource(types.VolumeSnapshotClassGVR()).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, vsc := range objs.Items {
		annotations := vsc.GetAnnotations()
		if annotations == nil {
			continue
		}
		// skip creation if default volumesnapshotclass exists.
		if annotations["snapshot.storage.kubernetes.io/is-default-class"] == annotationTrueValue {
			return nil
		}
	}

	inputs := create.Inputs{
		CueTemplateName: "volumesnapshotclass_template.cue",
		ResourceName:    "volumesnapshotclasses",
		Group:           "snapshot.storage.k8s.io",
		Version:         types.K8sCoreAPIVersion,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
	}
	if err := o.BaseOptions.Run(inputs); err != nil {
		return err
	}
	return nil
}

func (o *CreateBackupOptions) Complete() error {
	// generate backupName
	if len(o.BackupName) == 0 {
		o.BackupName = strings.Join([]string{"backup", o.Namespace, o.Name, time.Now().Format("20060102150405")}, "-")
	}
	return nil
}

func (o *CreateBackupOptions) Validate() error {
	// validate
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	if o.BackupPolicy == "" {
		return o.completeDefaultBackupPolicy()
	}
	// check if backup policy exists
	_, err := o.Dynamic.Resource(types.BackupPolicyGVR()).Namespace(o.Namespace).Get(context.TODO(), o.BackupPolicy, metav1.GetOptions{})
	// TODO: check if pvc exists
	return err
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
		Resource(types.BackupPolicyGVR()).
		List(context.TODO(), opts)
	if err != nil {
		return "", err
	}
	if len(objs.Items) == 0 {
		return "", fmt.Errorf(`not found any backup policy for cluster "%s"`, o.Name)
	}
	var defaultBackupPolicies []unstructured.Unstructured
	for _, obj := range objs.Items {
		if obj.GetAnnotations()[constant.DefaultBackupPolicyAnnotationKey] == annotationTrueValue {
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

func NewCreateBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateBackupOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	customOutPut := func(opt *create.BaseOptions) {
		output := fmt.Sprintf("Backup %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli cluster list-backups --name=%s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}
	inputs := create.Inputs{
		Use:                          "backup",
		Short:                        "Create a backup.",
		Example:                      createBackupExample,
		CueTemplateName:              "backup_template.cue",
		ResourceName:                 types.ResourceBackups,
		Group:                        types.DPAPIGroup,
		Version:                      types.DPAPIVersion,
		BaseOptionsObj:               &o.BaseOptions,
		Options:                      o,
		Factory:                      f,
		Complete:                     o.Complete,
		Validate:                     o.Validate,
		CustomOutPut:                 customOutPut,
		ResourceNameGVRForCompletion: types.ClusterGVR(),
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.BackupType, "backup-type", "snapshot", "Backup type")
			cmd.Flags().StringVar(&o.BackupName, "backup-name", "", "Backup name")
			cmd.Flags().StringVar(&o.BackupPolicy, "backup-policy", "", "Backup policy name, this flag will be ignored when backup-type is snapshot")
		},
	}
	return create.BuildCommand(inputs)
}

// getClusterNameMap get cluster list by namespace and convert to map.
func getClusterNameMap(dClient dynamic.Interface, o *list.ListOptions) (map[string]struct{}, error) {
	clusterList, err := dClient.Resource(types.ClusterGVR()).Namespace(o.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	clusterMap := make(map[string]struct{})
	for _, v := range clusterList.Items {
		clusterMap[v.GetName()] = struct{}{}
	}
	return clusterMap, nil
}

func printBackupList(o ListBackupOptions) error {
	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
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

	clusterNameMap, err := getClusterNameMap(dynamic, o.ListOptions)
	if err != nil {
		return err
	}

	// sort the unstructured objects with the creationTimestamp in positive order
	sort.Sort(unstructuredList(backupList.Items))
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "CLUSTER", "TYPE", "STATUS", "TOTAL-SIZE", "DURATION", "CREATE-TIME", "COMPLETION-TIME")
	for _, obj := range backupList.Items {
		backup := &dataprotectionv1alpha1.Backup{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, backup); err != nil {
			return err
		}
		clusterName := backup.Labels[constant.AppInstanceLabelKey]
		if _, ok := clusterNameMap[clusterName]; !ok {
			clusterName = fmt.Sprintf("%s (deleted)", clusterName)
		}
		durationStr := ""
		if backup.Status.Duration != nil {
			durationStr = duration.HumanDuration(backup.Status.Duration.Duration)
		}
		if len(o.BackupName) > 0 {
			if o.BackupName == obj.GetName() {
				tbl.AddRow(backup.Name, clusterName, backup.Spec.BackupType, backup.Status.Phase, backup.Status.TotalSize,
					durationStr, util.TimeFormat(&backup.CreationTimestamp), util.TimeFormat(backup.Status.CompletionTimestamp))
			}
			continue
		}
		tbl.AddRow(backup.Name, clusterName, backup.Spec.BackupType, backup.Status.Phase, backup.Status.TotalSize,
			durationStr, util.TimeFormat(&backup.CreationTimestamp), util.TimeFormat(backup.Status.CompletionTimestamp))
	}
	tbl.Print()
	return nil
}

func NewListBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListBackupOptions{ListOptions: list.NewListOptions(f, streams, types.OpsGVR())}
	cmd := &cobra.Command{
		Use:               "list-backups",
		Short:             "List backups.",
		Aliases:           []string{"ls-backup"},
		Example:           listBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			o.Names = nil
			util.CheckErr(o.Complete())
			util.CheckErr(printBackupList(*o))
		},
	}
	o.AddFlags(cmd)
	cmd.Flags().StringVar(&o.BackupName, "name", "", "The backup name to get the details.")
	return cmd
}

func NewDeleteBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.BackupGVR())
	cmd := &cobra.Command{
		Use:               "delete-backup",
		Short:             "Delete a backup.",
		Example:           deleteBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(completeForDeleteBackup(o, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringSliceVar(&o.Names, "name", []string{}, "Backup names")
	o.AddFlags(cmd)
	return cmd
}

// completeForDeleteBackup complete cmd for delete backup
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
		// do force action, if specified --force and not specified --name, all backups with the cluster will be deleted
		// if no specify backup name and cluster name is specified. it will delete all backups with the cluster
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
	RestoreTime    *time.Time `json:"restoreTime,omitempty"`
	RestoreTimeStr string     `json:"restoreTimeStr,omitempty"`
	SourceCluster  string     `json:"sourceCluster,omitempty"`

	create.BaseOptions
}

func (o *CreateRestoreOptions) getClusterObject(backup *dataprotectionv1alpha1.Backup) (*appsv1alpha1.Cluster, error) {
	clusterName := backup.Labels[constant.AppInstanceLabelKey]
	clusterObj, err := cluster.GetClusterByName(o.Dynamic, clusterName, o.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		// if the source cluster does not exist, obtain it from the cluster snapshot of the backup.
		clusterString, ok := backup.Annotations[constant.ClusterSnapshotAnnotationKey]
		if !ok {
			return nil, fmt.Errorf("source cluster: %s not found", clusterName)
		}
		err = json.Unmarshal([]byte(clusterString), &clusterObj)
	}
	return clusterObj, err
}

func (o *CreateRestoreOptions) Run() error {
	if o.Backup != "" {
		return o.runRestoreFromBackup()
	} else if o.RestoreTime != nil {
		return o.runPITR()
	}
	return nil
}

func (o *CreateRestoreOptions) runRestoreFromBackup() error {
	// get backup
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cluster.GetK8SClientObject(o.Dynamic, backup, types.BackupGVR(), o.Namespace, o.Backup); err != nil {
		return err
	}
	if backup.Status.Phase != dataprotectionv1alpha1.BackupCompleted {
		return errors.Errorf(`backup "%s" is not completed.`, backup.Name)
	}
	if len(backup.Labels[constant.AppInstanceLabelKey]) == 0 {
		return errors.Errorf(`missing source cluster in backup "%s", "app.kubernetes.io/instance" is empty in labels.`, o.Backup)
	}
	// get the cluster object and set the annotation for restore
	clusterObj, err := o.getClusterObject(backup)
	if err != nil {
		return err
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, len(clusterObj.Spec.ComponentSpecs), clusterObj.Spec.ComponentSpecs[0].Name)
	if err != nil {
		return err
	}
	clusterObj.ObjectMeta = metav1.ObjectMeta{
		Namespace:   clusterObj.Namespace,
		Name:        o.Name,
		Annotations: map[string]string{constant.RestoreFromBackUpAnnotationKey: restoreAnnotation},
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

func (o *CreateRestoreOptions) runPITR() error {
	objs, err := o.Dynamic.Resource(types.BackupGVR()).Namespace(o.Namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				constant.AppInstanceLabelKey, o.SourceCluster),
		})
	if err != nil {
		return err
	}
	backup := &dataprotectionv1alpha1.Backup{}

	// no need check items len because it is validated by o.validateRestoreTime().
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objs.Items[0].Object, backup); err != nil {
		return err
	}
	// TODO: use opsRequest to create cluster.
	// get the cluster object and set the annotation for restore
	clusterObj, err := o.getClusterObject(backup)
	if err != nil {
		return err
	}
	clusterObj.ObjectMeta = metav1.ObjectMeta{
		Namespace: clusterObj.Namespace,
		Name:      o.Name,
		Annotations: map[string]string{
			// TODO: use constant annotation key
			"kubeblocks.io/restore-from-time":           o.RestoreTime.Format(time.RFC3339),
			"kubeblocks.io/restore-from-source-cluster": o.SourceCluster,
		},
	}
	return o.createCluster(clusterObj)
}

func isTimeInRange(t time.Time, start time.Time, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}

func (o *CreateRestoreOptions) validateRestoreTime() error {
	if o.RestoreTimeStr == "" && o.SourceCluster == "" {
		return nil
	}
	if o.RestoreTimeStr == "" && o.SourceCluster == "" {
		return fmt.Errorf("--source-cluster must be specified if specified --restore-to-time")
	}
	restoreTime, err := util.TimeParse(o.RestoreTimeStr, time.Second)
	if err != nil {
		return err
	}
	o.RestoreTime = &restoreTime
	objs, err := o.Dynamic.Resource(types.BackupGVR()).Namespace(o.Namespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				constant.AppInstanceLabelKey, o.SourceCluster),
		})
	if err != nil {
		return err
	}
	backups := make([]dataprotectionv1alpha1.Backup, 0)
	for _, i := range objs.Items {
		obj := dataprotectionv1alpha1.Backup{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(i.Object, &obj); err != nil {
			return err
		}
		backups = append(backups, obj)
	}
	recoverableTime := dataprotectionv1alpha1.GetRecoverableTimeRange(backups)
	for _, i := range recoverableTime {
		if isTimeInRange(restoreTime, i.StartTime.Time, i.StopTime.Time) {
			return nil
		}
	}
	return fmt.Errorf("restore-to-time is out of time range, you can view the recoverable time: \n"+
		"\tkbcli cluster describe %s -n %s", o.SourceCluster, o.Namespace)
}

func (o *CreateRestoreOptions) Validate() error {
	if o.Backup == "" && o.RestoreTimeStr == "" {
		return fmt.Errorf("must be specified one of the --backup or --restore-to-time")
	}
	if err := o.validateRestoreTime(); err != nil {
		return err
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

func NewCreateRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateRestoreOptions{}
	o.IOStreams = streams
	inputs := create.Inputs{
		BaseOptionsObj: &create.BaseOptions{IOStreams: streams},
		Options:        o,
		Factory:        f,
	}
	cmd := &cobra.Command{
		Use:               "restore",
		Short:             "Restore a new cluster from backup.",
		Example:           createRestoreExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(inputs, args))
			util.CheckErr(o.Validate())
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.Backup, "backup", "", "Backup name")
	cmd.Flags().StringVar(&o.RestoreTimeStr, "restore-to-time", "", "point in time recovery(PITR)")
	cmd.Flags().StringVar(&o.SourceCluster, "source-cluster", "", "source cluster name")
	return cmd
}

func NewListRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.RestoreJobGVR())
	cmd := &cobra.Command{
		Use:               "list-restores",
		Short:             "List all restore jobs.",
		Aliases:           []string{"ls-restores"},
		Example:           listRestoreExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			o.Names = nil
			_, err := o.Run()
			util.CheckErr(err)
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewDeleteRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.RestoreJobGVR())
	cmd := &cobra.Command{
		Use:               "delete-restore",
		Short:             "Delete a restore job.",
		Example:           deleteRestoreExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(completeForDeleteRestore(o, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringSliceVar(&o.Names, "name", []string{}, "Restore names")
	o.AddFlags(cmd)
	return cmd
}

// completeForDeleteRestore complete cmd for delete restore
func completeForDeleteRestore(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("Missing cluster name")
	}
	if len(args) > 1 {
		return errors.New("Only supported delete the restore of one cluster")
	}
	if !o.Force && len(o.Names) == 0 {
		return errors.New("Missing --name as restore name.")
	}
	if o.Force && len(o.Names) == 0 {
		// do force action, if specified --force and not specified --name, all restores with the cluster will be deleted
		// if no specify restore name and cluster name is specified. it will delete all restores with the cluster
		o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
		o.ConfirmedNames = args
	}
	o.ConfirmedNames = o.Names
	return nil
}

func NewListBackupPolicyCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.OpsGVR())
	cmd := &cobra.Command{
		Use:               "list-backup-policy",
		Short:             "List backups policies.",
		Aliases:           []string{"list-bp"},
		Example:           listBackupPolicyExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			o.Names = nil
			util.CheckErr(o.Complete())
			util.CheckErr(printBackupPolicyList(*o))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

// printBackupPolicyList prints the backup policy list.
func printBackupPolicyList(o list.ListOptions) error {
	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
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
	tbl.SetHeader("NAME", "DEFAULT", "CLUSTER", "CREATE-TIME")
	for _, obj := range backupPolicyList.Items {
		defaultPolicy, ok := obj.GetAnnotations()[constant.DefaultBackupPolicyAnnotationKey]
		if !ok {
			defaultPolicy = "false"
		}
		createTime := obj.GetCreationTimestamp()
		tbl.AddRow(obj.GetName(), defaultPolicy, obj.GetLabels()[constant.AppInstanceLabelKey], util.TimeFormat(&createTime))
	}
	tbl.Print()
	return nil
}

func NewEditBackupPolicyCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := edit.NewEditOptions(f, streams, types.BackupPolicyGVR())
	cmd := &cobra.Command{
		Use:                   "edit-backup-policy",
		DisableFlagsInUseLine: true,
		Aliases:               []string{"edit-bp"},
		Short:                 "Edit backup policy",
		Example:               editExample,
		ValidArgsFunction:     util.ResourceNameCompletionFunc(f, types.BackupPolicyGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.AddFlags(cmd)
	return cmd
}
