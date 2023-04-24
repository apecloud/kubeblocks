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

package apps

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/sethvargo/go-password/password"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var ToIgnoreFinalizers []string

func init() {
	ResetToIgnoreFinalizers()
}

func ResetToIgnoreFinalizers() {
	ToIgnoreFinalizers = []string{
		"orphan",
		"kubernetes.io/pvc-protection",
		// REVIEW: adding following is a hack, if tests are running as
		// controller-runtime manager setup.
		constant.ConfigurationTemplateFinalizerName,
		"cluster.kubeblocks.io/finalizer",
	}
}

// Helper functions to change object's fields in input closure and then update it.
// Each helper is a wrapper of k8sClient.Patch.
// Example:
// Expect(ChangeObj(testCtx, obj, func() {
//		// modify input obj
// })).Should(Succeed())

func ChangeObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	pobj PT, action func(PT)) error {
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action(pobj)
	return testCtx.Cli.Patch(testCtx.Ctx, pobj, patch)
}

func ChangeObjStatus[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	pobj PT, action func()) error {
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action()
	return testCtx.Cli.Status().Patch(testCtx.Ctx, pobj, patch)
}

// Helper functions to get object, change its fields in input closure and update it.
// Each helper is a wrapper of client.Get and client.Patch.
// Each helper returns a Gomega assertion function, which should be passed into
// Eventually() or Consistently() as the first parameter.
// Example:
// Eventually(GetAndChangeObj(testCtx, key, func(fetched *appsv1alpha1.ClusterDefinition) {
//		    // modify fetched clusterDef
//      })).Should(Succeed())
// Warning: these functions should NOT be used together with Expect().
// BAD Example:
// Expect(GetAndChangeObj(testCtx, key, ...)).Should(Succeed())
// Although it compiles, and test may also pass, it makes no sense and doesn't work as you expect.

func GetAndChangeObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](
	testCtx *testutil.TestContext, namespacedName types.NamespacedName, action func(PT)) func() error {
	return func() error {
		var obj T
		pobj := PT(&obj)
		if err := testCtx.Cli.Get(testCtx.Ctx, namespacedName, pobj); err != nil {
			return err
		}
		return ChangeObj(testCtx, pobj, func(lobj PT) {
			action(lobj)
		})
	}
}

func GetAndChangeObjStatus[T intctrlutil.Object, PT intctrlutil.PObject[T]](
	testCtx *testutil.TestContext, namespacedName types.NamespacedName, action func(pobj PT)) func() error {
	return func() error {
		var obj T
		pobj := PT(&obj)
		if err := testCtx.Cli.Get(testCtx.Ctx, namespacedName, pobj); err != nil {
			return err
		}
		return ChangeObjStatus(testCtx, pobj, func() { action(pobj) })
	}
}

// Helper functions to check fields of resources when writing unit tests.
// Each helper returns a Gomega assertion function, which should be passed into
// Eventually() or Consistently() as the first parameter.
// Example:
// Eventually(CheckObj(testCtx, key, func(g Gomega, fetched *appsv1alpha1.Cluster) {
//   g.Expect(..).To(BeTrue()) // do some check
// })).Should(Succeed())
// Warning: these functions should NOT be used together with Expect().
// BAD Example:
// Expect(CheckObj(testCtx, key, ...)).Should(Succeed())
// Although it compiles, and test may also pass, it makes no sense and doesn't work as you expect.

func CheckObjExists(testCtx *testutil.TestContext, namespacedName types.NamespacedName,
	obj client.Object, expectExisted bool) func(g gomega.Gomega) {
	return func(g gomega.Gomega) {
		err := testCtx.Cli.Get(testCtx.Ctx, namespacedName, obj)
		if expectExisted {
			g.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
		} else {
			g.Expect(err).To(gomega.Satisfy(apierrors.IsNotFound))
		}
	}
}

func CheckObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	namespacedName types.NamespacedName, check func(g gomega.Gomega, pobj PT)) func(g gomega.Gomega) {
	return func(g gomega.Gomega) {
		var obj T
		pobj := PT(&obj)
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, namespacedName, pobj)).To(gomega.Succeed())
		check(g, pobj)
	}
}

// Helper functions to check fields of resource lists when writing unit tests.

func GetListLen[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	testCtx *testutil.TestContext, _ func(T, L), opt ...client.ListOption) func(gomega.Gomega) int {
	return func(g gomega.Gomega) int {
		var objList L
		g.Expect(testCtx.Cli.List(testCtx.Ctx, PL(&objList), opt...)).To(gomega.Succeed())
		items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
		return len(items)
	}
}

// Helper functions to create object from testdata files.

func CustomizeObjYAML(a ...any) func(string) string {
	return func(inputYAML string) string {
		return fmt.Sprintf(inputYAML, a...)
	}
}

func GetRandomizedKey(namespace, prefix string) types.NamespacedName {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	return types.NamespacedName{
		Name:      prefix + randomStr,
		Namespace: namespace,
	}
}

func RandomizedObjName() func(client.Object) {
	return func(obj client.Object) {
		randomStr, _ := password.Generate(6, 0, 0, true, false)
		obj.SetName(obj.GetName() + randomStr)
	}
}

func WithName(name string) func(client.Object) {
	return func(obj client.Object) {
		obj.SetName(name)
	}
}

func WithNamespace(namespace string) func(client.Object) {
	return func(obj client.Object) {
		obj.SetNamespace(namespace)
	}
}

func WithNamespacedName(resourceName, ns string) func(client.Object) {
	return func(obj client.Object) {
		obj.SetNamespace(ns)
		obj.SetName(resourceName)
	}
}

func WithMap(keysAndValues ...string) map[string]string {
	// ignore mismatching for kvs
	m := make(map[string]string, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		m[keysAndValues[i]] = keysAndValues[i+1]
	}
	return m
}

func WithLabels(keysAndValues ...string) func(client.Object) {
	return func(obj client.Object) {
		obj.SetLabels(WithMap(keysAndValues...))
	}
}

func WithAnnotations(keysAndValues ...string) func(client.Object) {
	return func(obj client.Object) {
		obj.SetAnnotations(WithMap(keysAndValues...))
	}
}

// CreateObj calls CreateCustomizedObj with CustomizeObjYAML wrapper for any optional modify actions.
func CreateObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	filePath string, pobj PT, actions ...any) PT {
	return CreateCustomizedObj(testCtx, filePath, pobj, CustomizeObjYAML(actions...))
}

func NewCustomizedObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](
	filePath string, pobj PT, actions ...any) PT {
	objBytes, err := testdata.GetTestDataFileContent(filePath)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	objYAML := string(objBytes)
	for _, action := range actions {
		if action == nil {
			continue
		}
		switch f := action.(type) {
		case func(string) string:
			objYAML = f(objYAML)
		default:
		}
	}
	gomega.Expect(yaml.Unmarshal([]byte(objYAML), pobj)).Should(gomega.Succeed())
	for _, action := range actions {
		if action == nil {
			continue
		}
		switch f := action.(type) {
		case func(client.Object):
			f(pobj)
		case func(PT):
			f(pobj)
		}
	}
	return pobj
}

func CreateCustomizedObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	filePath string, pobj PT, actions ...any) PT {
	pobj = NewCustomizedObj(filePath, pobj, actions...)
	return CreateK8sResource(*testCtx, pobj).(PT)
}

func CheckedCreateCustomizedObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	filePath string, pobj PT, actions ...any) PT {
	pobj = NewCustomizedObj(filePath, pobj, actions...)
	return CheckedCreateK8sResource(*testCtx, pobj).(PT)
}

// Helper functions to delete object.

func DeleteObject[T intctrlutil.Object, PT intctrlutil.PObject[T]](
	testCtx *testutil.TestContext, key types.NamespacedName, pobj PT) {
	gomega.Expect(func() error {
		if err := testCtx.Cli.Get(testCtx.Ctx, key, pobj); err != nil {
			return client.IgnoreNotFound(err)
		}
		return testCtx.Cli.Delete(testCtx.Ctx, pobj)
	}()).Should(gomega.Succeed())
}

// Helper functions to delete a list of resources when writing unit tests.

// ClearResources clears all resources of the given type T satisfying the input ListOptions.
func ClearResources[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	testCtx *testutil.TestContext, funcSig func(T, L), opts ...client.DeleteAllOfOption) {
	ClearResourcesWithRemoveFinalizerOption[T, PT, L, PL](testCtx, funcSig, false, opts...)
}

// ClearResourcesWithRemoveFinalizerOption clears all resources of the given type T with
// removeFinalizer specifier, and satisfying the input ListOptions.
func ClearResourcesWithRemoveFinalizerOption[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	testCtx *testutil.TestContext, _ func(T, L), removeFinalizer bool, opts ...client.DeleteAllOfOption) {
	var (
		obj     T
		objList L
	)

	listOptions := make([]client.ListOption, 0)
	for _, opt := range opts {
		applyToListFunc := reflect.ValueOf(opt).MethodByName("ApplyToList")
		if applyToListFunc.IsValid() {
			listOptions = append(listOptions, opt.(client.ListOption))
		}
	}

	gvk, _ := apiutil.GVKForObject(PL(&objList), testCtx.Cli.Scheme())
	ginkgo.By("clear resources " + strings.TrimSuffix(gvk.Kind, "List"))
	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.DeleteAllOf(testCtx.Ctx, PT(&obj), opts...)).ShouldNot(gomega.HaveOccurred())
		g.Expect(testCtx.Cli.List(testCtx.Ctx, PL(&objList), listOptions...)).Should(gomega.Succeed())
		items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
		for _, obj := range items {
			pobj := PT(&obj)
			if pobj.GetDeletionTimestamp().IsZero() {
				panic("expected DeletionTimestamp is not nil")
			}
			finalizers := pobj.GetFinalizers()
			if len(finalizers) > 0 {
				if removeFinalizer {
					g.Expect(ChangeObj(testCtx, pobj, func(lobj PT) {
						pobj.SetFinalizers([]string{})
					})).To(gomega.Succeed())
				} else {
					g.Expect(finalizers).Should(gomega.BeEmpty())
				}
			}
		}
		g.Expect(items).Should(gomega.BeEmpty())
	}, testCtx.ClearResourceTimeout, testCtx.ClearResourcePollingInterval).Should(gomega.Succeed())
}

// ClearClusterResources clears all dependent resources belonging to existing clusters.
// The function is intended to be called to clean resources created by cluster controller in envtest
// environment without UseExistingCluster set, where garbage collection lacks.
func ClearClusterResources(testCtx *testutil.TestContext) {
	inNS := client.InNamespace(testCtx.DefaultNamespace)
	ClearResources(testCtx, intctrlutil.ClusterSignature, inNS,
		client.HasLabels{testCtx.TestObjLabelKey})
	// finalizer of ConfigMap are deleted in ClusterDef&ClusterVersion controller
	ClearResources(testCtx, intctrlutil.ClusterVersionSignature,
		client.HasLabels{testCtx.TestObjLabelKey})
	ClearResources(testCtx, intctrlutil.ClusterDefinitionSignature,
		client.HasLabels{testCtx.TestObjLabelKey})
}
