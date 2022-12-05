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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/create"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/list"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/builder"
)

var (
	createBackupExample = templates.Examples(`
		# create a backup
		kbcli cluster backup cluster-name
	`)
	listBackupExample = templates.Examples(`
		# list all backup
		kbcli cluster list-backup
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
	ClusterName string `json:"clusterName,omitempty"`
	TTL         string `json:"ttl,omitempty"`
	create.BaseOptions
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

	// apply backup policy
	policyOptions := CreateBackupPolicyOptions{
		TTL:         o.TTL,
		ClusterName: o.Name,
		BaseOptions: o.BaseOptions,
	}
	policyOptions.Name = "backup-policy-" + o.Namespace + "-" + o.Name
	inputs := create.Inputs{
		CueTemplateName: "backuppolicy_template.cue",
		ResourceName:    types.ResourceBackupPolicies,
		Group:           types.DPGroup,
		Version:         types.DPVersion,
		BaseOptionsObj:  &policyOptions.BaseOptions,
		Options:         policyOptions,
	}

	// cluster backup do 2 following things:
	// 1. create or apply the backupPolicy, cause backupJob reference to a backupPolicy,
	//   and backupPolicy reference to the cluster.
	//   so it need apply the backupPolicy after the first backupPolicy created.
	// 2. create a backupJob.
	if err := policyOptions.BaseOptions.RunAsApply(inputs); err != nil {
		return err
	}
	o.BackupPolicy = policyOptions.Name

	return nil
}

func NewCreateBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateBackupOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "backup",
		Short:           "Create a backup",
		Example:         createBackupExample,
		CueTemplateName: "backupjob_template.cue",
		ResourceName:    types.ResourceBackupJobs,
		Group:           types.DPGroup,
		Version:         types.DPVersion,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Complete:        o.Complete,
		Validate:        o.Validate,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.BackupType, "backup-type", "snapshot", "Backup type")
			cmd.Flags().StringVar(&o.BackupName, "backup-name", "", "Backup name")
			cmd.Flags().StringVar(&o.Role, "role", "", "backup on cluster role")
			cmd.Flags().StringVar(&o.TTL, "ttl", "168h0m0s", "Time to live")
		},
	}
	return create.BuildCommand(inputs)
}

func NewListBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("list-backups").
		Short("List backup jobs.").
		Example(listBackupExample).
		Factory(f).
		GVR(types.BackupJobGVR()).
		IOStreams(streams).
		Build(list.Build)
}

func NewDeleteBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("delete-backup").
		Short("Delete a backup job.").
		Example(deleteBackupExample).
		GVR(types.BackupJobGVR()).
		Factory(f).
		IOStreams(streams).
		CustomComplete(completeForDeleteBackup).
		CustomFlags(customFlagsForDeleteBackup).
		Build(delete.Build)
}

func customFlagsForDeleteBackup(option builder.Options, cmd *cobra.Command) {
	var (
		o  *delete.DeleteFlags
		ok bool
	)
	if o, ok = option.(*delete.DeleteFlags); !ok {
		return
	}
	cmd.Flags().StringSliceVar(&o.ResourceNames, "name", []string{}, "Backup names")
}

// completeForDeleteBackup complete cmd for delete backup
func completeForDeleteBackup(option builder.Options, args []string) error {
	var (
		flag *delete.DeleteFlags
		ok   bool
	)
	if flag, ok = option.(*delete.DeleteFlags); !ok {
		return nil
	}

	if len(args) == 0 {
		return errors.New("Missing cluster name")
	}
	if len(args) > 1 {
		return errors.New("Only supported delete the Backup of one cluster")
	}
	if !*flag.Force && len(flag.ResourceNames) == 0 {
		return errors.New("Missing --name as backup name.")
	}
	if *flag.Force && len(flag.ResourceNames) == 0 {
		// do force action, if specified --force and not specified --name, all backups with the cluster will be deleted
		flag.ClusterName = args[0]
		// if no specify backup name and cluster name is specified. it will delete all backups with the cluster
		labelString := fmt.Sprintf("%s=%s", types.InstanceLabelKey, flag.ClusterName)
		if flag.LabelSelector == nil || len(*flag.LabelSelector) == 0 {
			flag.LabelSelector = &labelString
		} else {
			// merge label
			newLabelSelector := *flag.LabelSelector + "," + labelString
			flag.LabelSelector = &newLabelSelector
		}
	}
	return nil
}

type CreateRestoreOptions struct {
	CreateOptions
}

func (o *CreateRestoreOptions) Complete() error {
	// get backup job
	gvr := schema.GroupVersionResource{Group: types.DPGroup, Version: types.DPVersion, Resource: types.ResourceBackupJobs}
	backupJobObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Backup, metav1.GetOptions{})
	if err != nil {
		return err
	}
	srcClusterName, clusterExists, err := unstructured.NestedString(backupJobObj.Object, "metadata", "labels", "app.kubernetes.io/instance")
	if err != nil {
		return err
	}
	if !clusterExists {
		return errors.Errorf("Missing source cluster in backup '%s'.", o.Backup)
	}

	gvr = schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusters}

	clusterObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), srcClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster := dbaasv1alpha1.Cluster{}
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(clusterObj.UnstructuredContent(), &cluster)
	if err != nil {
		return err
	}

	o.AppVersionRef = cluster.Spec.AppVersionRef
	o.ClusterDefRef = cluster.Spec.ClusterDefRef
	o.TerminationPolicy = string(cluster.Spec.TerminationPolicy)
	o.PodAntiAffinity = string(cluster.Spec.Affinity.PodAntiAffinity)
	o.Monitor = cluster.Spec.Components[0].Monitor
	componentByte, err := json.Marshal(cluster.Spec.Components)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(componentByte, &o.Components); err != nil {
		return err
	}

	return o.CreateOptions.Complete()
}

func (o *CreateRestoreOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	return nil
}

func NewCreateRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateRestoreOptions{}
	o.BaseOptions = create.BaseOptions{IOStreams: streams}
	inputs := create.Inputs{
		Use:             "restore",
		Short:           "Restore a new cluster from backup",
		Example:         createRestoreExample,
		CueTemplateName: CueTemplateName,
		ResourceName:    types.ResourceClusters,
		Group:           types.Group,
		Version:         types.Version,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.Backup, "backup", "", "Backup name")
		},
	}
	return create.BuildCommand(inputs)
}

func NewListRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("list-restores").
		Short("List all restore jobs.").
		Example(listRestoreExample).
		Factory(f).
		GVR(types.RestoreJobGVR()).
		IOStreams(streams).
		Build(list.Build)
}

func NewDeleteRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("delete-restore").
		Short("Delete a restore job.").
		Example(deleteRestoreExample).
		GVR(types.RestoreJobGVR()).
		Factory(f).
		IOStreams(streams).
		CustomFlags(customFlagsForDeleteRestore).
		CustomComplete(completeForDeleteRestore).
		Build(delete.Build)
}

func customFlagsForDeleteRestore(option builder.Options, cmd *cobra.Command) {
	var (
		o  *delete.DeleteFlags
		ok bool
	)
	if o, ok = option.(*delete.DeleteFlags); !ok {
		return
	}
	cmd.Flags().StringSliceVar(&o.ResourceNames, "name", []string{}, "Restore names")
}

// completeForDeleteRestore complete cmd for delete restore
func completeForDeleteRestore(option builder.Options, args []string) error {
	var (
		flag *delete.DeleteFlags
		ok   bool
	)
	if flag, ok = option.(*delete.DeleteFlags); !ok {
		return nil
	}

	if len(args) == 0 {
		return errors.New("Missing cluster name")
	}
	if len(args) > 1 {
		return errors.New("Only supported delete the restore of one cluster")
	}
	if !*flag.Force && len(flag.ResourceNames) == 0 {
		return errors.New("Missing --name as restore name.")
	}
	if *flag.Force && len(flag.ResourceNames) == 0 {
		// do force action, if specified --force and not specified --name, all restores with the cluster will be deleted
		flag.ClusterName = args[0]
		// if no specify restore name and cluster name is specified. it will delete all restores with the cluster
		labelString := fmt.Sprintf("%s=%s", types.InstanceLabelKey, flag.ClusterName)
		if flag.LabelSelector == nil || len(*flag.LabelSelector) == 0 {
			flag.LabelSelector = &labelString
		} else {
			// merge label
			newLabelSelector := *flag.LabelSelector + "," + labelString
			flag.LabelSelector = &newLabelSelector
		}
	}
	return nil
}
