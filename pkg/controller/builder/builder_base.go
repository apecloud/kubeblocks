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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
)

// TODO(free6om): a copy(and updated) of testutil.apps.base_factory, should make this as a common util both used by builder and testing
// Manipulate common attributes here to save boilerplate code

type BaseBuilder[T intctrlutil.Object, PT intctrlutil.PObject[T], B any] struct {
	object          PT
	concreteBuilder *B
}

func (builder *BaseBuilder[T, PT, B]) init(namespace, name string, obj PT, b *B) {
	obj.SetNamespace(namespace)
	obj.SetName(name)
	builder.object = obj
	builder.concreteBuilder = b
}

func (builder *BaseBuilder[T, PT, B]) get() PT {
	return builder.object
}

func (builder *BaseBuilder[T, PT, B]) SetName(name string) *B {
	builder.object.SetName(name)
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) SetUID(uid types.UID) *B {
	builder.object.SetUID(uid)
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) AddLabels(keysAndValues ...string) *B {
	builder.AddLabelsInMap(WithMap(keysAndValues...))
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) AddLabelsInMap(labels map[string]string) *B {
	if len(labels) == 0 {
		return builder.concreteBuilder
	}
	l := builder.object.GetLabels()
	if l == nil {
		l = make(map[string]string, 0)
	}
	for k, v := range labels {
		l[k] = v
	}
	builder.object.SetLabels(l)
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) AddAnnotations(keysAndValues ...string) *B {
	builder.AddAnnotationsInMap(WithMap(keysAndValues...))
	return builder.concreteBuilder
}
func (builder *BaseBuilder[T, PT, B]) AddAnnotationsInMap(annotations map[string]string) *B {
	if len(annotations) == 0 {
		return builder.concreteBuilder
	}
	a := builder.object.GetAnnotations()
	if a == nil {
		a = make(map[string]string, 0)
	}
	for k, v := range annotations {
		a[k] = v
	}
	builder.object.SetAnnotations(a)
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) AddControllerRevisionHashLabel(value string) *B {
	return builder.AddLabels(appsv1.ControllerRevisionHashLabelKey, value)
}

func (builder *BaseBuilder[T, PT, B]) SetOwnerReferences(ownerAPIVersion string, ownerKind string, owner client.Object) *B {
	// interface object needs to determine whether the value is nil.
	// otherwise, nil pointer error may be reported.
	if owner != nil && !reflect.ValueOf(owner).IsNil() {
		t := true
		builder.object.SetOwnerReferences([]metav1.OwnerReference{
			{APIVersion: ownerAPIVersion, Kind: ownerKind, Controller: &t,
				BlockOwnerDeletion: &t, Name: owner.GetName(), UID: owner.GetUID()},
		})
	}
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) AddFinalizers(finalizers []string) *B {
	builder.object.SetFinalizers(finalizers)
	return builder.concreteBuilder
}

func (builder *BaseBuilder[T, PT, B]) GetObject() PT {
	return builder.object
}

func WithMap(keysAndValues ...string) map[string]string {
	// ignore mismatching for kvs
	m := make(map[string]string, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		m[keysAndValues[i]] = keysAndValues[i+1]
	}
	return m
}
