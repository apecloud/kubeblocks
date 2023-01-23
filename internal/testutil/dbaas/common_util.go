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
	gomega "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// Helper functions to change fields in the desired state and status of resources.
// Each helper is a wrapper of k8sClient.Patch.
// Example:
// changeSpec(testCtx, key, func(clusterDef *dbaasv1alpha1.ClusterDefinition) {
//		// modify clusterDef
// })

func ChangeSpec[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	namespacedName types.NamespacedName, action func(PT)) error {
	var obj T
	pobj := PT(&obj)
	if err := testCtx.Cli.Get(testCtx.Ctx, namespacedName, pobj); err != nil {
		return err
	}
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action(pobj)
	if err := testCtx.Cli.Patch(testCtx.Ctx, pobj, patch); err != nil {
		return err
	}
	return nil
}

func ChangeStatus[T intctrlutil.Object, PT intctrlutil.PObject[T]](testCtx *testutil.TestContext,
	namespacedName types.NamespacedName, action func(pobj PT)) error {
	var obj T
	pobj := PT(&obj)
	if err := testCtx.Cli.Get(testCtx.Ctx, namespacedName, pobj); err != nil {
		return err
	}
	patch := client.MergeFrom(PT(pobj.DeepCopy()))
	action(pobj)
	if err := testCtx.Cli.Status().Patch(testCtx.Ctx, pobj, patch); err != nil {
		return err
	}
	return nil
}

// Helper functions to check fields of resources when writing unit tests.
// Each helper returns a Gomega assertion function, which should be passed into
// Eventually() or Consistently() as the first parameter.
// Example:
// Eventually(testCtx, checkObj(key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
//   g.Expect(..).To(BeTrue()) // do some check
// })).Should(Succeed())

func CheckExists(testCtx *testutil.TestContext, namespacedName types.NamespacedName,
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
	L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L], Traits intctrlutil.ObjListTraits[T, L]](
	testCtx *testutil.TestContext, _ func(T, L, Traits), opts ...client.ListOption) {
	var (
		objList L
		traits  Traits
	)

	gomega.Eventually(func() error {
		return testCtx.Cli.List(testCtx.Ctx, PL(&objList), opts...)
	}, testCtx.DefaultTimeout, testCtx.DefaultInterval).Should(gomega.Succeed())
	for _, obj := range traits.GetItems(&objList) {
		// it's possible deletions are initiated in testcases code but cache is not updated
		gomega.Expect(client.IgnoreNotFound(testCtx.Cli.Delete(testCtx.Ctx, PT(&obj)))).Should(gomega.Succeed())
	}

	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(testCtx.Cli.List(testCtx.Ctx, PL(&objList), opts...)).Should(gomega.Succeed())
		for _, obj := range traits.GetItems(&objList) {
			pobj := PT(&obj)
			finalizers := pobj.GetFinalizers()
			if len(finalizers) > 0 {
				// PVCs are protected by the "kubernetes.io/pvc-protection" finalizer
				g.Expect(finalizers[0]).Should(gomega.BeElementOf([]string{"orphan", "kubernetes.io/pvc-protection"}))
				g.Expect(len(finalizers)).Should(gomega.Equal(1))
				pobj.SetFinalizers([]string{})
				g.Expect(testCtx.Cli.Update(testCtx.Ctx, pobj)).Should(gomega.Succeed())
			}
		}
		g.Expect(len(traits.GetItems(&objList))).Should(gomega.Equal(0))
	}, testCtx.ClearResourceTimeout, testCtx.ClearResourceInterval).Should(gomega.Succeed())
}

// ClearClusterResources clears all dependent resources belonging existing clusters.
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
	if !(testCtx.TestEnv.UseExistingCluster != nil && *testCtx.TestEnv.UseExistingCluster) {
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
