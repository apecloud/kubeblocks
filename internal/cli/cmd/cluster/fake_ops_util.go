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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

func NewFakeOperationsOptions(ns, cName string, opsType appsv1alpha1.OpsType, objs ...runtime.Object) (*cmdtesting.TestFactory, *OperationsOptions) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	tf := cmdtesting.NewTestFactory().WithNamespace(ns)
	o := &OperationsOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: streams,
			Name:      cName,
			Namespace: ns,
		},
		TTLSecondsAfterSucceed: 30,
		OpsType:                opsType,
	}

	err := appsv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	// TODO using GroupVersionResource of FakeKubeObjectHelper
	listMapping := map[schema.GroupVersionResource]string{
		types.ClusterDefGVR():       types.KindClusterDef + "List",
		types.ClusterVersionGVR():   types.KindClusterVersion + "List",
		types.ClusterGVR():          types.KindCluster + "List",
		types.ConfigConstraintGVR(): types.KindConfigConstraint + "List",
		types.BackupGVR():           types.KindBackup + "List",
		types.RestoreJobGVR():       types.KindRestoreJob + "List",
		types.OpsGVR():              types.KindOps + "List",
	}
	o.Dynamic = dynamicfakeclient.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, listMapping, objs...)
	return tf, o
}
