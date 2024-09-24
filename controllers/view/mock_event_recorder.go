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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type mockEventRecorder struct {
	store map[model.GVKNObjKey]client.Object
}

func (r *mockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	//TODO implement me
	panic("implement me")
}

func (r *mockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	//TODO implement me
	panic("implement me")
}

func (r *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	//TODO implement me
	panic("implement me")
}

func newMockEventRecorder(store map[model.GVKNObjKey]client.Object) record.EventRecorder {
	return &mockEventRecorder{store: store}
}

var _ record.EventRecorder = &mockEventRecorder{}
