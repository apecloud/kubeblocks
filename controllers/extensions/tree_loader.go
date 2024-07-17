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

package extensions

import (
	"context"

	extensions "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type treeLoader struct{}

func (t *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	tree, err := kubebuilderx.ReadObjectTree[*extensions.Addon](ctx, reader, req, nil)
	if err != nil {
		return nil, err
	}
	root := tree.GetRoot()
	if root == nil {
		return tree, nil
	}

	tree.EventRecorder = recorder
	tree.Logger = logger

	return tree, nil
}

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

var _ kubebuilderx.TreeLoader = &treeLoader{}
