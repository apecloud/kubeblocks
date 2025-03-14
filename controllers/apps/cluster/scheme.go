/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func init() {
	model.AddScheme(clientgoscheme.AddToScheme)
	model.AddScheme(appsv1alpha1.AddToScheme)
	model.AddScheme(appsv1.AddToScheme)
	model.AddScheme(dpv1alpha1.AddToScheme)
	model.AddScheme(snapshotv1.AddToScheme)
	// model.AddScheme(snapshotv1beta1.AddToScheme)
	// model.AddScheme(extensionsv1alpha1.AddToScheme)
	// model.AddScheme(workloadsv1.AddToScheme)
	// model.AddScheme(batchv1.AddToScheme)
}
