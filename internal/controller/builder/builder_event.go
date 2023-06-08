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

package builder

import (
	corev1 "k8s.io/api/core/v1"
)

type EventBuilder struct {
	BaseBuilder[corev1.Event, *corev1.Event, EventBuilder]
}

func NewEventBuilder(namespace, name string) *EventBuilder {
	builder := &EventBuilder{}
	builder.init(namespace, name, &corev1.Event{}, builder)
	return builder
}

func (builder *EventBuilder) SetInvolvedObject(objectRef corev1.ObjectReference) *EventBuilder {
	builder.get().InvolvedObject = objectRef
	return builder
}
