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

package preflight

import (
	"os"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/preflight/util"
)

// LoadPreflightSpec loads content of preflightSpec and hostPreflightSpec against yamlFiles from args
func LoadPreflightSpec(checkFileList []string, checkYamlData [][]byte) (*preflightv1beta2.Preflight, *preflightv1beta2.HostPreflight, string, error) {
	var (
		preflightSpec     *preflightv1beta2.Preflight
		hostPreflightSpec *preflightv1beta2.HostPreflight
		preflightContent  []byte
		preflightName     string
		err               error
	)
	for _, fileName := range checkFileList {
		// support to load yaml from stdin, local file and URI
		if preflightContent, err = cluster.MultipleSourceComponents(fileName, os.Stdin); err != nil {
			return preflightSpec, hostPreflightSpec, preflightName, err
		}
		checkYamlData = append(checkYamlData, preflightContent)
	}
	for _, yamlData := range checkYamlData {
		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(yamlData, nil, nil)
		if err != nil {
			return preflightSpec, hostPreflightSpec, preflightName, errors.Wrapf(err, "failed to parse %s", string(yamlData))
		}
		if spec, ok := obj.(*preflightv1beta2.Preflight); ok {
			preflightSpec = ConcatPreflightSpec(preflightSpec, spec)
			preflightName = preflightSpec.Name
		} else if spec, ok := obj.(*preflightv1beta2.HostPreflight); ok {
			hostPreflightSpec = ConcatHostPreflightSpec(hostPreflightSpec, spec)
			preflightName = hostPreflightSpec.Name
		}
	}
	return preflightSpec, hostPreflightSpec, preflightName, nil
}

func init() {
	// register the scheme of troubleshoot API and decode function
	if err := util.AddToScheme(scheme.Scheme); err != nil {
		return
	}
}
