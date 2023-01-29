/*
Copyright ApeCloud Inc.

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

package dbaas

import (
	"reflect"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// Helper functions to change object's fields in input closure and then update it.
// Each helper is a wrapper of k8sClient.Patch.
// Example:
// Expect(ChangeObj(testCtx, obj, func() {
//		// modify input obj
// })).Should(Succeed())

func ChangeObj[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	pobj PT, action func()) error {
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action()
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
// Eventually(GetAndChangeObj(testCtx, key, func(fetched *dbaasv1alpha1.ClusterDefinition) {
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
		return ChangeObj(testCtx, pobj, func() { action(pobj) })
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
// Eventually(CheckObj(testCtx, key, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
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

// Helper functions to delete a list of resources when writing unit tests.

// ClearResources clears all resources of the given type T satisfying the input ListOptions.
func ClearResources[T intctrlutil.Object, PT intctrlutil.PObject[T],
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
	testCtx *testutil.TestContext, _ func(T, L), opts ...client.DeleteAllOfOption) {
	var (
		obj     T
		objList L
	)

	listOptions := make([]client.ListOption, 0)
	deleteOptions := make([]client.DeleteOption, 0)
	for _, opt := range opts {
		applyToDeleteFunc := reflect.ValueOf(opt).MethodByName("ApplyToDelete")
		if applyToDeleteFunc.IsValid() {
			deleteOptions = append(deleteOptions, opt.(client.DeleteOption))
		} else {
			listOptions = append(listOptions, opt.(client.ListOption))
		}
	}

	gvk, _ := apiutil.GVKForObject(PL(&objList), testCtx.Cli.Scheme())
	ginkgo.By("clear resources " + strings.TrimSuffix(gvk.Kind, "List"))

	gomega.Eventually(func() error {
		return testCtx.Cli.DeleteAllOf(testCtx.Ctx, PT(&obj), opts...)
	}).Should(gomega.Succeed())

	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, PL(&objList), listOptions...)).Should(gomega.Succeed())
		items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
		for _, obj := range items {
			pobj := PT(&obj)
			finalizers := pobj.GetFinalizers()
			if len(finalizers) > 0 {
				// PVCs are protected by the "kubernetes.io/pvc-protection" finalizer
				g.Expect(finalizers[0]).Should(gomega.BeElementOf([]string{"orphan", "kubernetes.io/pvc-protection"}))
				g.Expect(len(finalizers)).Should(gomega.Equal(1))
				g.Expect(ChangeObj(testCtx, pobj, func() {
					pobj.SetFinalizers([]string{})
				})).To(gomega.Succeed())
			}
			// it's possible that the object wasn't deleted in previous round in race conditions,
			// so it's more reliable to delete it in each loop.
			gomega.Expect(client.IgnoreNotFound(testCtx.Cli.Delete(testCtx.Ctx, PT(&obj),
				deleteOptions...))).Should(gomega.Succeed())
		}
		g.Expect(len(items)).Should(gomega.Equal(0))
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

	// mock behavior of garbage collection inside KCM
	if !testCtx.UsingExistingCluster() {
		ClearResources(testCtx, intctrlutil.StatefulSetSignature, inNS)
		ClearResources(testCtx, intctrlutil.DeploymentSignature, inNS)
		ClearResources(testCtx, intctrlutil.ConfigMapSignature, inNS)
		ClearResources(testCtx, intctrlutil.ServiceSignature, inNS)
		ClearResources(testCtx, intctrlutil.SecretSignature, inNS)
		ClearResources(testCtx, intctrlutil.PodDisruptionBudgetSignature, inNS)
		ClearResources(testCtx, intctrlutil.JobSignature, inNS)
		ClearResources(testCtx, intctrlutil.PersistentVolumeClaimSignature, inNS)
	}
}
