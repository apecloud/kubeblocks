/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builder

import (
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
)

// TODO: a copy of testutil.apps.base_factory, should make this as a common util both used by builder and testing
// Manipulate common attributes here to save boilerplate code

type BaseFactory[T intctrlutil.Object, PT intctrlutil.PObject[T], F any] struct {
	object          PT
	concreteFactory *F
}

func (factory *BaseFactory[T, PT, F]) init(namespace, name string, obj PT, f *F) {
	obj.SetNamespace(namespace)
	obj.SetName(name)
	if obj.GetLabels() == nil {
		obj.SetLabels(map[string]string{})
	}
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
	}
	factory.object = obj
	factory.concreteFactory = f
}

func (factory *BaseFactory[T, PT, F]) get() PT {
	return factory.object
}

func (factory *BaseFactory[T, PT, F]) SetName(name string) *F {
	factory.object.SetName(name)
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) AddLabels(keysAndValues ...string) *F {
	factory.AddLabelsInMap(WithMap(keysAndValues...))
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) AddLabelsInMap(labels map[string]string) *F {
	l := factory.object.GetLabels()
	for k, v := range labels {
		l[k] = v
	}
	factory.object.SetLabels(l)
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) AddAnnotations(keysAndValues ...string) *F {
	factory.AddAnnotationsInMap(WithMap(keysAndValues...))
	return factory.concreteFactory
}
func (factory *BaseFactory[T, PT, F]) AddAnnotationsInMap(annotations map[string]string) *F {
	a := factory.object.GetAnnotations()
	for k, v := range annotations {
		a[k] = v
	}
	factory.object.SetAnnotations(a)
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) AddControllerRevisionHashLabel(value string) *F {
	return factory.AddLabels(appsv1.ControllerRevisionHashLabelKey, value)
}

func (factory *BaseFactory[T, PT, F]) SetOwnerReferences(ownerAPIVersion string, ownerKind string, owner client.Object) *F {
	// interface object needs to determine whether the value is nil.
	// otherwise, nil pointer error may be reported.
	if owner != nil && !reflect.ValueOf(owner).IsNil() {
		t := true
		factory.object.SetOwnerReferences([]metav1.OwnerReference{
			{APIVersion: ownerAPIVersion, Kind: ownerKind, Controller: &t,
				BlockOwnerDeletion: &t, Name: owner.GetName(), UID: owner.GetUID()},
		})
	}
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) AddFinalizers(finalizers []string) *F {
	factory.object.SetFinalizers(finalizers)
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) GetObject() PT {
	return factory.object
}

func WithMap(keysAndValues ...string) map[string]string {
	// ignore mismatching for kvs
	m := make(map[string]string, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		m[keysAndValues[i]] = keysAndValues[i+1]
	}
	return m
}