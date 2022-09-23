/*
Copyright © 2022 The dbctl Authors

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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

type commandOptions struct {
	Namespace string

	Describer  func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	NewBuilder func() *resource.Builder

	BuilderArgs []string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	FilenameOptions   *resource.FilenameOptions

	client ctrlcli.Client
	genericclioptions.IOStreams
}

func (o *commandOptions) setup(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.PlaygroundSourceName}, args...)

	o.Describer = func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
		return describe.DescriberFn(f, mapping)
	}

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	c, err := ctrlcli.New(config, ctrlcli.Options{})
	if err != nil {
		return err
	}
	o.client = c
	o.NewBuilder = f.NewBuilder

	return nil
}

// dbClusterHandler is iterator handlers function for dbClusters
func (o *commandOptions) run(dbClusterHandler func(*types.DBClusterInfo), postHandler func() error, opts ...ctrlcli.ListOption) error {
	ctx := context.Background()
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.GroupVersionKind{})

	// TODO: need to apply  MatchingLabels
	// ml := ctrlcli.MatchingLabels()
	if err := o.client.List(ctx, ul); err != nil {
		return err
	}

	for _, dbCluster := range ul.Items {
		clusterInfo := buildClusterInfo(&dbCluster)
		dbClusterHandler(clusterInfo)
	}
	if err := postHandler(); err != nil {
		return err
	}

	if len(ul.Items) == 0 {
		// if we wrote no output, and had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}
	return nil
}
