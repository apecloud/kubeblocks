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
	"time"

	"github.com/apecloud/kubeblocks/pkg/utils"

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

type CreateOptions struct {
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
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create a new backup job",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.Name, "name", "", "specify backup job name.")

	return cmd
}

func (o *CreateOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.BackupJobSourceName}, args...)

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

func (o *CreateOptions) Run() error {
	backupJobName := o.Name
	if backupJobName == "" {
		backupJobName = "backupjob-" + time.Now().Format("20060102150405")
	}
	backupJobObj := NewBackupJobInstance(o.Namespace, backupJobName)
	gvr := schema.GroupVersionResource{Group: "dataprotection.infracreate.com", Version: "v1alpha1", Resource: "backupjobs"}
	obj, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), backupJobObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	utils.PrintObjYaml(obj)
	return nil
}

func NewBackupJobInstance(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dataprotection.infracreate.com/v1alpha1",
			"kind":       "BackupJob",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"backupPolicyName": "backup-policy-1",
				"backupType":       "full",
				"ttl":              "168h0m0s",
			},
		},
	}
}
