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

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/create"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/list"
	"github.com/apecloud/kubeblocks/internal/dbctl/delete"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
)

var (
	listBackupExample = templates.Examples(`
		# list all backup
		dbctl cluster list-backup
	`)
	deleteBackupExample = templates.Examples(`
		# delete a backup named backup-name
		dbctl cluster delete-backup backup-name
	`)
	listRestoreExample = templates.Examples(`
		# list all restore
		dbctl cluster list-restore
	`)
	deleteRestoreExample = templates.Examples(`
		# delete a restore named restore-name
		dbctl cluster delete-restore restore-name
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
		o.BackupName = strings.Join([]string{o.Name, o.Namespace, "backup", time.Now().Format("20060102150405")}, "-")
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
	if err := policyOptions.BaseOptions.RunAsApply(inputs); err != nil {
		return err
	}

	return nil
}

func NewCreateBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateBackupOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "backup",
		Short:           "Create a database backup",
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
		Use("list-backup").
		Short("List all backup jobs.").
		Example(listBackupExample).
		Factory(f).
		GroupKind(types.BackupJobGK()).
		IOStreams(streams).
		Build(list.Build)
}

func NewDeleteBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("delete-backup").
		Short("Delete a backup job.").
		Example(deleteBackupExample).
		GroupKind(types.BackupJobGK()).
		Factory(f).
		IOStreams(streams).
		Build(delete.Build)
}

type CreateRestoreOptions struct {
	BackupJob string
	CreateOptions
	create.BaseOptions
}

func (o *CreateRestoreOptions) CompleteRestore() error {
	// get backup job
	gvr := schema.GroupVersionResource{Group: types.DPGroup, Version: types.DPVersion, Resource: types.ResourceBackupJobs}
	obj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	srcClusterName, _, err := unstructured.NestedString(obj.Object, "metadata", "labels", "clusters.dbaas.kubeblocks.io/name")
	if err != nil {
		return err
	}

	gvr = schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusters}
	obj, err = o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), srcClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	o.AppVersionRef, _, _ = unstructured.NestedString(obj.Object, "spec", "appVersionRef")
	o.ClusterDefRef, _, _ = unstructured.NestedString(obj.Object, "spec", "clusterDefinitionRef")
	o.TerminationPolicy, _, _ = unstructured.NestedString(obj.Object, "spec", "terminationPolicy")
	// components, _, _ := unstructured.NestedSlice(obj.Object, "spec", "components")
	// component, _, _ := unstructured.NestedMap(components[0], "spec", "components")

	return nil
}

func (o *CreateRestoreOptions) ValidateRestore() error {
	return nil
}

func NewCreateRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateRestoreOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "restore",
		Short:           "Restore a database from backup",
		CueTemplateName: CueTemplateName,
		ResourceName:    types.ResourceClusters,
		Group:           types.DPGroup,
		Version:         types.DPVersion,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.ValidateRestore,
		Complete:        o.CompleteRestore,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.BackupJob, "backup", "", "Backup name")
		},
	}
	return create.BuildCommand(inputs)
}

func NewListRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("list-restore").
		Short("List all restore jobs.").
		Example(listRestoreExample).
		Factory(f).
		GroupKind(types.RestoreJobGK()).
		IOStreams(streams).
		Build(list.Build)
}

func NewDeleteRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("delete-restore").
		Short("Delete a restore job.").
		Example(deleteRestoreExample).
		GroupKind(types.RestoreJobGK()).
		Factory(f).
		IOStreams(streams).
		Build(delete.Build)
}
