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

package builder

import (
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

func Test(t *testing.T) {
	cmd := NewCmdBuilder().
		IOStreams(genericclioptions.NewTestIOStreamsDiscard()).
		Factory(nil).
		Use("test").
		Short("test command short description").
		Example("test command example").
		GroupKind(types.ClusterGK()).Cmd()

	if cmd == nil {
		t.Errorf("cmd is nil")
	}
}
