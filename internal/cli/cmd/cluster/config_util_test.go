/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

func NewFakeOperationsOptions(ns, cName string, opsType appsv1alpha1.OpsType, objs ...runtime.Object) (*cmdtesting.TestFactory, *create.CreateOptions) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	tf := cmdtesting.NewTestFactory().WithNamespace(ns)
	baseOptions := &create.CreateOptions{
		IOStreams: streams,
		Name:      cName,
		Namespace: ns,
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
	baseOptions.Dynamic = dynamicfakeclient.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, listMapping, objs...)
	return tf, baseOptions
}
