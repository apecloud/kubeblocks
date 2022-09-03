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

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	"github.com/apecloud/kubeblocks/pkg/types"
)

type RestoreOptions struct {
	Namespace string

	Describer  func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	NewBuilder func() *resource.Builder

	BuilderArgs []string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	FilenameOptions   *resource.FilenameOptions

	client dynamic.Interface
	genericclioptions.IOStreams

	DbCluster     string
	BackupJobName string
}

func NewRestoreCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &RestoreOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore a new database",
		Run: func(cmd *cobra.Command, args []string) {
			dbCluster, _ := cmd.Flags().GetString("dbCluster")
			args = append(args, dbCluster)
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.DbCluster, "dbcluster", "d", "", "db cluster name.")
	_ = cmd.MarkFlagRequired("dbcluster")
	cmd.Flags().StringVarP(&o.BackupJobName, "backupname", "b", "", "restore from the backup job name.")
	_ = cmd.MarkFlagRequired("backupname")

	return cmd
}

func (o *RestoreOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.RestoreJobSourceName}, args...)

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

func (o *RestoreOptions) Run() error {
	restoreJobName := "restorejob-" + time.Now().Format("20060102150405")
	restoreJobObj := NewRestoreJobInstance(o.Namespace, restoreJobName, o.DbCluster, o.BackupJobName)
	fmt.Println(restoreJobObj)
	gvr := schema.GroupVersionResource{Group: "dataprotection.infracreate.com", Version: "v1alpha1", Resource: "restorejobs"}
	obj, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), restoreJobObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Println(obj)
	return nil
}

func NewRestoreJobInstance(namespace, name, dbCluster, backupJob string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dataprotection.infracreate.com/v1alpha1",
			"kind":       "RestoreJob",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"backupJobName": backupJob,
				"target": map[string]interface{}{
					"databaseEngine": "mysql",
					"labelsSelector": map[string]interface{}{
						"matchLabels": map[string]interface{}{"mysql.oracle.com/cluster": dbCluster},
					},
				},
				"targetVolumes": []map[string]interface{}{
					{
						"name": "mysql-restore-storage",
						"persistentVolumeClaim": map[string]interface{}{
							"claimName": "datadir-" + dbCluster + "-0",
						},
					},
				},
				"targetVolumeMounts": []map[string]interface{}{
					{
						"name":      "mysql-restore-storage",
						"mountPath": "/var/lib/mysql",
					},
				},
			},
		},
	}
}
