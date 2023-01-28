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

package k8score

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("StorageClass Controller", func() {
	var (
		ctx      = context.Background()
		timeout  = time.Second * 10
		interval = time.Second
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start, otherwise :
		// - if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		// - worse, if an async DeleteAll call is issued here, it maybe executed later by the
		// K8s API server, by which time the testcase may have already created some new test objects,
		// which shall be accidentally deleted.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createStorageClassObj := func(storageClassName string, allowVolumeExpansion bool) *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: %t
`, storageClassName, allowVolumeExpansion)
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, sc)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: storageClassName}, &storagev1.StorageClass{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return sc
	}

	handleStorageClass := func(reqCtx intctrlutil.RequestCtx, cli client.Client, storageClass *storagev1.StorageClass) error {
		patch := client.MergeFrom(storageClass.DeepCopy())
		storageClass.Annotations["kubeblocks.io/test"] = "test"
		return cli.Patch(ctx, storageClass, patch)
	}

	Context("When test creating storageClass", func() {
		It("should handle it properly", func() {
			By("test storageClass changes")
			StorageClassHandlerMap["test-controller"] = handleStorageClass
			storageClassName := fmt.Sprintf("standard-%s", testCtx.GetRandomStr())
			createStorageClassObj(storageClassName, true)
			storageClass := &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion := true
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(k8sClient.Update(ctx, storageClass))

			// wait until storageClass patched
			Eventually(func() bool {
				tempStorageClass := &storagev1.StorageClass{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: storageClass.Name}, tempStorageClass)
				return tempStorageClass.Annotations["kubeblocks.io/test"] == "test"
			}, timeout, interval).Should(BeTrue())
		})
	})
})
