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
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
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
	Role             string `json:"role,omitempty"`
	create.BaseOptions
}

type CreateVolumeSnapshotClassOptions struct {
	Driver string `json:"driver"`
	Name   string `json:"name"`
	create.BaseOptions
}

func (o *CreateVolumeSnapshotClassOptions) Complete() error {
	objs, err := o.Client.
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
	objs, err := o.Client.
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
		Version:         types.VersionV1,
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
	role, err := o.getClusterRole()
	if err != nil {
		return err
	}
	// apply backup policy
	policyOptions := CreateBackupPolicyOptions{
		TTL:              o.TTL,
		ClusterName:      o.Name,
		ConnectionSecret: connectionSecret,
		PolicyTemplate:   backupPolicyTemplate,
		Role:             role,
		BaseOptions:      o.BaseOptions,
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
			intctrlutil.AppInstanceLabelKey, o.Name,
			intctrlutil.AppManagedByLabelKey, intctrlutil.AppName),
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	secretObjs, err := o.Client.Resource(gvr).Namespace(o.Namespace).List(context.TODO(), opts)
	if err != nil {
		return "", err
	}
	if len(secretObjs.Items) == 0 {
		return "", fmt.Errorf("not found connection credential for cluster %s", o.Name)
	}
	return secretObjs.Items[0].GetName(), nil
}

func (o *CreateBackupOptions) getDefaultBackupPolicyTemplate() (string, error) {
	clusterObj, err := o.Client.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// find backupPolicyTemplate from cluster label
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s",
			intctrlutil.ClusterDefLabelKey, clusterObj.GetLabels()[intctrlutil.ClusterDefLabelKey]),
	}
	objs, err := o.Client.
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

func (o *CreateBackupOptions) getClusterRole() (string, error) {
	clusterObj, err := o.Client.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	cluster := appsv1alpha1.Cluster{}
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(clusterObj.UnstructuredContent(), &cluster)
	if err != nil {
		return "", err
	}
	clusterDefObj, err := o.Client.Resource(types.ClusterDefGVR()).Get(context.TODO(), cluster.Spec.ClusterDefRef, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterDef := appsv1alpha1.ClusterDefinition{}
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(clusterDefObj.UnstructuredContent(), &clusterDef)
	if err != nil {
		return "", err
	}
	switch clusterDef.Spec.ComponentDefs[0].WorkloadType {
	case appsv1alpha1.Replication:
		if o.BackupType == string(dpv1alpha1.BackupTypeSnapshot) {
			return "primary", nil
		}
		return "secondary", nil
	default:
		if o.BackupType == string(dpv1alpha1.BackupTypeSnapshot) {
			return "leader", nil
		}
		return "follower", nil
	}
}

func NewCreateBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateBackupOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "backup",
		Short:           "Create a backup",
		Example:         createBackupExample,
		CueTemplateName: "backup_template.cue",
		ResourceName:    types.ResourceBackups,
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
	o := list.NewListOptions(f, streams, types.BackupGVR())
	cmd := &cobra.Command{
		Use:               "list-backups",
		Short:             "List backups",
		Aliases:           []string{"ls-backups"},
		Example:           listBackupExample,
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

func NewDeleteBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.BackupGVR())
	cmd := &cobra.Command{
		Use:               "delete-backup",
		Short:             "Delete a backup",
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
	CreateOptions
}

func (o *CreateRestoreOptions) Complete() error {
	// get backup job
	gvr := schema.GroupVersionResource{Group: types.DPGroup, Version: types.DPVersion, Resource: types.ResourceBackups}
	backupObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Backup, metav1.GetOptions{})
	if err != nil {
		return err
	}
	srcClusterName, clusterExists, err := unstructured.NestedString(backupObj.Object, "metadata", "labels", "app.kubernetes.io/instance")
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
	cluster := appsv1alpha1.Cluster{}
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(clusterObj.UnstructuredContent(), &cluster)
	if err != nil {
		return err
	}

	o.ClusterVersionRef = cluster.Spec.ClusterVersionRef
	o.ClusterDefRef = cluster.Spec.ClusterDefRef
	o.TerminationPolicy = string(cluster.Spec.TerminationPolicy)

	if cluster.Spec.Affinity != nil {
		o.PodAntiAffinity = string(cluster.Spec.Affinity.PodAntiAffinity)
		o.NodeLabels = cluster.Spec.Affinity.NodeLabels
		o.TopologyKeys = cluster.Spec.Affinity.TopologyKeys
		o.Tenancy = string(cluster.Spec.Affinity.Tenancy)
	} else {
		o.PodAntiAffinity = string(appsv1alpha1.Preferred)
		o.Tenancy = string(appsv1alpha1.SharedNode)
	}
	o.Monitor = cluster.Spec.ComponentSpecs[0].Monitor
	componentByte, err := json.Marshal(cluster.Spec.ComponentSpecs)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(componentByte, &o.ComponentSpecs); err != nil {
		return err
	}

	return o.CreateOptions.Complete()
}

func (o *CreateRestoreOptions) Validate() error {
	if o.Name == "" {
		name, err := generateClusterName(o.Client, o.Namespace)
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
	o := list.NewListOptions(f, streams, types.RestoreJobGVR())
	cmd := &cobra.Command{
		Use:               "list-restores",
		Short:             "List all restore jobs",
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
		Short:             "Delete a restore job",
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
