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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
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
	`)
)

type CreateBackupOptions struct {
	BackupType   string `json:"backupType"`
	BackupName   string `json:"backupName"`
	Role         string `json:"role,omitempty"`
	BackupPolicy string `json:"backupPolicy"`
	TTL          string `json:"ttl,omitempty"`
	create.BaseOptions
}

type CreateBackupPolicyOptions struct {
	ClusterName      string `json:"clusterName,omitempty"`
	TTL              string `json:"ttl,omitempty"`
	ConnectionSecret string `json:"connectionSecret,omitempty"`
	PolicyTemplate   string `json:"policyTemplate,omitempty"`
	create.BaseOptions
}

type CreateVolumeSnapshotClassOptions struct {
	Driver string `json:"driver"`
	Name   string `json:"name"`
	create.BaseOptions
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
		if annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
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
		if annotations["snapshot.storage.kubernetes.io/is-default-class"] == "true" {
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

	connectionSecret, err := o.getConnectionSecret()
	if err != nil {
		return err
	}

	backupPolicyTemplate, err := o.getDefaultBackupPolicyTemplate()
	if err != nil {
		return err
	}
	// apply backup policy
	policyOptions := CreateBackupPolicyOptions{
		TTL:              o.TTL,
		ClusterName:      o.Name,
		ConnectionSecret: connectionSecret,
		PolicyTemplate:   backupPolicyTemplate,
		BaseOptions:      o.BaseOptions,
	}
	policyOptions.Name = "backup-policy-" + o.Namespace + "-" + o.Name
	inputs := create.Inputs{
		CueTemplateName: "backuppolicy_template.cue",
		ResourceName:    types.ResourceBackupPolicies,
		Group:           types.DPAPIGroup,
		Version:         types.DPAPIVersion,
		BaseOptionsObj:  &policyOptions.BaseOptions,
		Options:         policyOptions,
	}

	// cluster backup do 2 following things:
	// 1. create or apply the backupPolicy, cause backupJob reference to a backupPolicy,
	//   and backupPolicy reference to the cluster.
	//   so it need apply the backupPolicy after the first backupPolicy created.
	// 2. create a backupJob.
	if err := policyOptions.BaseOptions.Run(inputs); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	o.BackupPolicy = policyOptions.Name

	return nil
}

func (o *CreateBackupOptions) getConnectionSecret() (string, error) {
	// find secret from cluster label
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
			constant.AppInstanceLabelKey, o.Name,
			constant.AppManagedByLabelKey, constant.AppName),
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	secretObjs, err := o.Dynamic.Resource(gvr).Namespace(o.Namespace).List(context.TODO(), opts)
	if err != nil {
		return "", err
	}
	if len(secretObjs.Items) == 0 {
		return "", fmt.Errorf("not found connection credential for cluster %s", o.Name)
	}
	return secretObjs.Items[0].GetName(), nil
}

func (o *CreateBackupOptions) getDefaultBackupPolicyTemplate() (string, error) {
	clusterObj, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// find backupPolicyTemplate from cluster label
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s",
			constant.ClusterDefLabelKey, clusterObj.GetLabels()[constant.ClusterDefLabelKey]),
	}
	objs, err := o.Dynamic.
		Resource(types.BackupPolicyTemplateGVR()).
		List(context.TODO(), opts)
	if err != nil {
		return "", err
	}
	if len(objs.Items) == 0 {
		return "", fmt.Errorf("not found any backupPolicyTemplate for cluster %s", o.Name)
	}
	return objs.Items[0].GetName(), nil
}

func NewCreateBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateBackupOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
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
		ResourceNameGVRForCompletion: types.ClusterGVR(),
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.BackupType, "backup-type", "snapshot", "Backup type")
			cmd.Flags().StringVar(&o.BackupName, "backup-name", "", "Backup name")
			cmd.Flags().StringVar(&o.Role, "role", "", "backup on cluster role")
			cmd.Flags().StringVar(&o.TTL, "ttl", "168h0m0s", "Time to live")
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

func printBackupList(o *list.ListOptions) error {
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

	clusterNameMap, err := getClusterNameMap(dynamic, o)
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
		tbl.AddRow(backup.Name, clusterName, backup.Spec.BackupType, backup.Status.Phase, backup.Status.TotalSize,
			durationStr, util.TimeFormat(&backup.CreationTimestamp), util.TimeFormat(backup.Status.CompletionTimestamp))
	}
	tbl.Print()
	return nil
}

func NewListBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.BackupGVR())
	cmd := &cobra.Command{
		Use:               "list-backups",
		Short:             "List backups.",
		Aliases:           []string{"ls-backups"},
		Example:           listBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			o.Names = nil
			util.CheckErr(o.Complete())
			util.CheckErr(printBackupList(o))
		},
	}
	o.AddFlags(cmd)
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
	// get backup job
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
	cluster, err := o.getClusterObject(backup)
	if err != nil {
		return err
	}
	restoreAnnotation, err := getRestoreFromBackupAnnotation(backup, len(cluster.Spec.ComponentSpecs), cluster.Spec.ComponentSpecs[0].Name)
	if err != nil {
		return err
	}
	cluster.Status = appsv1alpha1.ClusterStatus{}
	cluster.ObjectMeta = metav1.ObjectMeta{
		Namespace:   cluster.Namespace,
		Name:        o.Name,
		Annotations: map[string]string{constant.RestoreFromBackUpAnnotationKey: restoreAnnotation},
	}
	clusterGVR := types.ClusterGVR()
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

func (o *CreateRestoreOptions) Validate() error {
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
