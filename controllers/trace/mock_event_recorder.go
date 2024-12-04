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

package trace

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type mockEventRecorder struct {
	store  ChangeCaptureStore
	logger logr.Logger
}

func (r *mockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.emitEvent(object, nil, eventtype, reason, message)
}

func (r *mockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	message := fmt.Sprintf(messageFmt, args...)
	r.emitEvent(object, nil, eventtype, reason, message)
}

func (r *mockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	message := fmt.Sprintf(messageFmt, args...)
	r.emitEvent(object, annotations, eventtype, reason, message)
}

func (r *mockEventRecorder) emitEvent(object runtime.Object, annotations map[string]string, eventtype, reason, message string) {
	metaObj, err := meta.Accessor(object)
	if err != nil {
		r.logger.Error(err, "Error accessing object metadata")
		return
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s.%s", metaObj.GetName(), string(uuid.NewUUID())),
			Namespace:   metaObj.GetNamespace(),
			Annotations: annotations,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      object.GetObjectKind().GroupVersionKind().Kind,
			Namespace: metaObj.GetNamespace(),
			Name:      metaObj.GetName(),
			UID:       metaObj.GetUID(),
		},
		Type:    eventtype,
		Reason:  reason,
		Message: message,
	}
	_ = r.store.Insert(event)
}

func newMockEventRecorder(store ChangeCaptureStore) record.EventRecorder {
	logger := log.FromContext(context.Background()).WithName("MockEventRecorder")
	return &mockEventRecorder{
		store:  store,
		logger: logger,
	}
}

var _ record.EventRecorder = &mockEventRecorder{}
