/*
Copyright 2022 The KubeBlocks Authors

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

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type DeleteOptions struct {
	Namespace string
	Name      string

	client dynamic.Interface
	genericclioptions.IOStreams
}

func NewDeleteCmd(f cmdutil.Factory) *cobra.Command {
	o := &DeleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *DeleteOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing cluster name")
	}
	return nil
}

func (o *DeleteOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
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

	return nil
}

func (o *DeleteOptions) Run() error {
	gvr := schema.GroupVersionResource{Group: "dbaas.infracreate.com", Version: "v1alpha1", Resource: "clusters"}
	err := o.client.Resource(gvr).Namespace(o.Namespace).Delete(context.TODO(), o.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
