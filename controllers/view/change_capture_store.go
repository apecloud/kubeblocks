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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type ChangeCaptureStore interface {
	Insert(object client.Object, capture bool) error
	GetChanges() []viewv1.ObjectChange
}

type changeCaptureStore struct {
	scheme        *runtime.Scheme
	i18nResources *corev1.ConfigMap
	store         map[model.GVKNObjKey]client.Object
	changes       []viewv1.ObjectChange
}

func (s *changeCaptureStore) Insert(object client.Object, capture bool) error {
	//TODO implement me
	panic("implement me")
}

func (s *changeCaptureStore) GetChanges() []viewv1.ObjectChange {
	//TODO implement me
	panic("implement me")
}

func newChangeCaptureStore(scheme *runtime.Scheme, resource *corev1.ConfigMap) ChangeCaptureStore {
	return &changeCaptureStore{
		scheme:        scheme,
		i18nResources: resource,
	}
}

var _ ChangeCaptureStore = &changeCaptureStore{}
