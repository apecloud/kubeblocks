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

package instance

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

type treeLoader struct{}

var _ kubebuilderx.TreeLoader = &treeLoader{}

func (r *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	ml := getMatchLabels(req.Name)
	kinds := ownedKinds()
	tree, err := kubebuilderx.ReadObjectTree[*workloads.Instance](ctx, reader, req, ml, kinds...)
	if err != nil {
		return nil, err
	}

	tree.Context = ctx
	tree.EventRecorder = recorder
	tree.Logger = logger

	tree.SetFinalizer(finalizer)

	return tree, err
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.PodList{},
		&corev1.PersistentVolumeClaimList{},
		&corev1.ServiceList{},   // headless
		&corev1.ConfigMapList{}, // config & script, env
		&corev1.SecretList{},    // account, tls
		&corev1.ServiceAccountList{},
		&rbacv1.RoleBindingList{},
	}
}
