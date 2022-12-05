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

package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type WeSQL struct{}

var _ Interface = &WeSQL{}

var component = `- name: wesql-test
  type: replicasets
  monitor: false
  replicas: %d
  volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem
`

func (w *WeSQL) Install(replicas int, name string, namespace string) error {
	path, err := componentPath(replicas)
	if err != nil {
		return err
	}

	options, err := newCreateOptions(name, namespace, path)
	if err != nil {
		return err
	}

	inputs := create.Inputs{
		BaseOptionsObj:  &options.BaseOptions,
		Options:         options,
		CueTemplateName: cluster.CueTemplateName,
		ResourceName:    types.ResourceClusters,
	}

	return options.Run(inputs)
}

func componentPath(replicas int) (string, error) {
	playgroundDir, err := util.PlaygroundDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(playgroundDir, "component.yaml")
	if err = os.WriteFile(path, []byte(fmt.Sprintf(component, replicas)), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func newCreateOptions(name string, namespace string, component string) (*cluster.CreateOptions, error) {
	dynamicClient, err := util.NewFactory().DynamicClient()
	if err != nil {
		return nil, err
	}
	options := &cluster.CreateOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
			Namespace: namespace,
			Name:      name,
			Client:    dynamicClient,
		},
		ClusterDefRef:      cluster.DefaultClusterDef,
		AppVersionRef:      cluster.DefaultAppVersion,
		TerminationPolicy:  "WipeOut",
		ComponentsFilePath: component,
	}

	if err = options.Complete(); err != nil {
		return nil, err
	}
	return options, nil
}
