/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package experimental

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type treeLoader struct{}

func (t *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	tree, err := kubebuilderx.ReadObjectTree[*experimental.NodeCountScaler](ctx, reader, req, nil)
	if err != nil {
		return nil, err
	}
	root := tree.GetRoot()
	if root == nil {
		return tree, nil
	}
	scaler, _ := root.(*experimental.NodeCountScaler)
	key := types.NamespacedName{Namespace: scaler.Namespace, Name: scaler.Spec.TargetClusterName}
	cluster := &appsv1.Cluster{}
	if err = reader.Get(ctx, key, cluster); err != nil {
		return nil, err
	}
	if err = tree.Add(cluster); err != nil {
		return nil, err
	}
	for _, compName := range scaler.Spec.TargetComponentNames {
		name := constant.GenerateClusterComponentName(scaler.Spec.TargetClusterName, compName)
		key = types.NamespacedName{Namespace: scaler.Namespace, Name: name}
		its := &workloads.InstanceSet{}
		if err = reader.Get(ctx, key, its); err != nil {
			return nil, err
		}
		if err = tree.Add(its); err != nil {
			return nil, err
		}
	}
	nodeList := &corev1.NodeList{}
	if err = reader.List(ctx, nodeList); err != nil {
		return nil, err
	}
	for i := range nodeList.Items {
		if err = tree.Add(&nodeList.Items[i]); err != nil {
			return nil, err
		}
	}

	tree.EventRecorder = recorder
	tree.Logger = logger

	return tree, nil
}

func objectTree() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

var _ kubebuilderx.TreeLoader = &treeLoader{}
