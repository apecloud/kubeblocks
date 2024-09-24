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

package view

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
)

type ReconcilerTree interface {
	Run() error
}

type reconcilerTree struct {

}

func (r *reconcilerTree) Run() error {
	//TODO implement me
	panic("implement me")
}

func newReconcilerTree(rules []viewv1.OwnershipRule, mClient client.Client, recorder record.EventRecorder) (ReconcilerTree, error) {
	return &reconcilerTree{}, nil
}

var _ ReconcilerTree = &reconcilerTree{}