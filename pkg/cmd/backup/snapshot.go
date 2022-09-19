/*
Copyright © 2022 The OpenCli Authors

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

package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/apecloud/kubeblocks/pkg/cmd/playground"

	"github.com/gosuri/uitable"
	"helm.sh/helm/v3/pkg/cli/output"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/apecloud/kubeblocks/pkg/utils"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	"github.com/apecloud/kubeblocks/pkg/types"
)

type SnapshotOptions struct {
	Namespace string
	Name      string

	Describer  func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	NewBuilder func() *resource.Builder

	BuilderArgs []string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	FilenameOptions   *resource.FilenameOptions

	client dynamic.Interface
	genericclioptions.IOStreams

	SourcePVC      string
	SourceSnapshot string
}

func NewSnapshotCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "backup snapshot command",
	}

	// add subcommands
	cmd.AddCommand(
		NewSnapCreateCmd(f, streams),
		NewSnapListCmd(f, streams),
		NewSnapRestoreCmd(f, streams),
	)

	return cmd
}

func NewSnapCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &SnapshotOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new backup volume snapshot",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.CompleteCreate(f, args))
			cmdutil.CheckErr(o.RunCreate())
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "specify backup job name.")
	cmd.Flags().StringVar(&o.SourcePVC, "source-pvc", "", "specify backup job name.")
	_ = cmd.MarkFlagRequired("source-pvc")

	return cmd
}

func (o *SnapshotOptions) CompleteCreate(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.BackupSnapSourceName}, args...)

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client
	o.NewBuilder = f.NewBuilder

	return nil
}

func (o *SnapshotOptions) RunCreate() error {
	snapshotName := o.Name
	if snapshotName == "" {
		snapshotName = "backup-snapshot-" + time.Now().Format("20060102150405")
	}
	snapshotObj := NewSnapshotInstance(o.Namespace, snapshotName, o.SourcePVC)

	gvr := schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  "v1",
		Resource: types.BackupSnapSourceName,
	}

	obj, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), snapshotObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	utils.PrintObjYaml(obj)
	return nil
}

func NewSnapshotInstance(namespace, name, sourcePVC string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "snapshot.storage.k8s.io/v1",
			"kind":       "VolumeSnapshot",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"volumeSnapshotClassName": "csi-aws-vsc",
				"source": map[string]interface{}{
					"persistentVolumeClaimName": sourcePVC,
				},
			},
		},
	}
}

func NewSnapListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &SnapshotOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all database backup snapshot.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.CompleteSnapList(f, args))
			cmdutil.CheckErr(o.RunSnapList())
		},
	}

	return cmd
}

func (o *SnapshotOptions) CompleteSnapList(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.BackupSnapSourceName}, args...)

	o.Describer = func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
		return describe.DescriberFn(f, mapping)
	}

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client
	o.NewBuilder = f.NewBuilder

	return nil
}

func (o *SnapshotOptions) RunSnapList() error {
	r := o.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		FilenameParam(o.EnforceNamespace, o.FilenameOptions).
		ResourceTypeOrNameArgs(true, o.BuilderArgs...).
		RequestChunksOf(o.DescriberSettings.ChunkSize).
		Flatten().
		Do()
	err := r.Err()
	if err != nil {
		return err
	}

	var allErrs []error
	infos, err := r.Infos()
	if err != nil {
		return err
	}

	table := uitable.New()
	table.AddRow("NAMESPACE", "NAME", "READYTOUSE", "SOURCEPVC", "RESTORESIZE", "SNAPSHOTCLASS", "CREATE_TIME")
	errs := sets.NewString()
	for _, info := range infos {
		backupSnapInfo := types.BackupSnapInfo{}

		mapping := info.ResourceMapping()
		if err != nil {
			if errs.Has(err.Error()) {
				continue
			}
			allErrs = append(allErrs, err)
			errs.Insert(err.Error())
			continue
		}

		backupSnapInfo.Namespace = info.Namespace
		backupSnapInfo.Name = info.Name
		obj, err := o.client.Resource(mapping.Resource).Namespace(o.Namespace).Get(context.TODO(), info.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		buildBackupSnapInfo(obj, &backupSnapInfo)
		table.AddRow(backupSnapInfo.Namespace, backupSnapInfo.Name, backupSnapInfo.ReadyToUse, backupSnapInfo.SourcePVC,
			backupSnapInfo.RestoreSize, backupSnapInfo.SnapshotClass, backupSnapInfo.CreationTime)
	}

	_ = output.EncodeTable(o.Out, table)
	if len(infos) == 0 && len(allErrs) == 0 {
		// if we wrote no output, and had no errors, be sure we output something.
		if o.AllNamespaces {
			_, _ = fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			_, _ = fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func buildBackupSnapInfo(obj *unstructured.Unstructured, info *types.BackupSnapInfo) {
	if obj.Object["status"] == nil {
		return
	}
	status := obj.Object["status"].(map[string]interface{})

	info.Name = obj.GetName()
	info.Namespace = obj.GetNamespace()
	if status["readyToUse"] != nil {
		info.ReadyToUse = status["readyToUse"].(bool)
	}
	if status["restoreSize"] != nil {
		info.RestoreSize = status["restoreSize"].(string)
	}
	if status["creationTime"] != nil {
		info.CreationTime = status["creationTime"].(string)
	}

	spec := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return
	}
	source := spec["source"].(map[string]interface{})
	if source == nil {
		return
	}
	info.SourcePVC = source["persistentVolumeClaimName"].(string)
	info.SnapshotClass = spec["volumeSnapshotClassName"].(string)

}

func NewSnapRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &SnapshotOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore a new database from volume snapshot",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.CompleteRestore(f, args))
			cmdutil.CheckErr(o.RunRestore())
		},
	}
	cmd.Flags().StringVar(&o.SourceSnapshot, "source-snapshot", "", "specify source snap shot name.")
	cmd.Flags().StringVar(&o.Name, "name", "", "specify database name.")
	_ = cmd.MarkFlagRequired("source-snapshot")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func (o *SnapshotOptions) CompleteRestore(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.PlaygroundSourceName}, args...)

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client
	o.NewBuilder = f.NewBuilder

	return nil
}

func (o *SnapshotOptions) RunRestore() error {
	storageCapacity, err := o.getSourceSnapshotCapacity()
	if err != nil {
		return err
	}
	snapshotObj := NewRestoreInstance(o.Namespace, o.Name, o.SourceSnapshot, storageCapacity)

	gvr := schema.GroupVersionResource{
		Group:    "mysql.oracle.com",
		Version:  "v2",
		Resource: types.PlaygroundSourceName,
	}

	obj, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), snapshotObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	utils.PrintObjYaml(obj)
	return nil
}

func (o *SnapshotOptions) getSourceSnapshotCapacity() (string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "snapshot.storage.k8s.io",
		Version:  "v1",
		Resource: "volumesnapshots",
	}

	obj, err := o.client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.SourceSnapshot, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if obj.Object["status"] == nil {
		return "", nil
	}
	status := obj.Object["status"].(map[string]interface{})

	return status["restoreSize"].(string), nil
}

func NewRestoreInstance(namespace, name, sourceSnapshot, storageCapacity string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "mysql.oracle.com/v2",
			"kind":       "InnoDBCluster",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"baseServerId": 1000,
				"datadirVolumeClaimTemplate": map[string]interface{}{
					"dataSource": map[string]interface{}{
						"name":     sourceSnapshot,
						"kind":     "VolumeSnapshot",
						"apiGroup": "snapshot.storage.k8s.io",
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": storageCapacity,
						},
					},
				},
				"imagePullPolicy":    "IfNotPresent",
				"instances":          1,
				"router":             map[string]interface{}{"instances": 0},
				"secretName":         "mycluster-cluster-secret",
				"serviceAccountName": "mycluster-sa",
				"tlsUseSelfSigned":   true,
				"version":            playground.DefaultVersion,
			},
		},
	}
}
