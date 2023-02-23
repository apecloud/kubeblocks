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

package apps

import (
	"context"
	"reflect"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

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
	factory.object = obj
	factory.concreteFactory = f
}

func (factory *BaseFactory[T, PT, F]) get() PT {
	return factory.object
}

func (factory *BaseFactory[T, PT, F]) WithRandomName() *F {
	key := GetRandomizedKey("", factory.object.GetName())
	factory.object.SetName(key.Name)
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

func (factory *BaseFactory[T, PT, F]) CheckedCreate(testCtx *testutil.TestContext) *F {
	gomega.Expect(testCtx.CheckedCreateObj(testCtx.Ctx, factory.get())).Should(gomega.Succeed())
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) Create(testCtx *testutil.TestContext) *F {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.get())).Should(gomega.Succeed())
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) CreateCli(ctx context.Context, cli client.Client) *F {
	gomega.Expect(cli.Create(ctx, factory.get())).Should(gomega.Succeed())
	return factory.concreteFactory
}

func (factory *BaseFactory[T, PT, F]) GetObject() PT {
	return factory.object
}
