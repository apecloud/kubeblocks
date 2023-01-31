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
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

func NewFakeReconfiguringOptions(objs ...runtime.Object) (*cmdtesting.TestFactory, *OperationsOptions) {
	defaultClusterName := "test"
	defaultNS := "default"

	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	tf := cmdtesting.NewTestFactory().WithNamespace(defaultNS)

	o := &OperationsOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: streams,
			Name:      defaultClusterName,
		},
		OpsType:                dbaasv1alpha1.ReconfiguringType,
		TTLSecondsAfterSucceed: 30,
	}

}
