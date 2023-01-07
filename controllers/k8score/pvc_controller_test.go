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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("PersistentVolumeClaim Controller", func() {
	var (
		ctx      = context.Background()
		timeout  = time.Second * 20
		interval = time.Second
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	createPVC := func(pvcName string) *corev1.PersistentVolumeClaim {
		By("By assure an default storageClass")
		pvcYAML := fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
     test: test
  labels:
    app.kubernetes.io/component-name: replicasets
    app.kubernetes.io/instance: wesql
    app.kubernetes.io/managed-by: kubeblocks
    app.kubernetes.io/name: state.mysql-apecloud-wesql
    vct.kubeblocks.io/name: data
  name: %s
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: "1Gi"
  storageClassName: csi-hostpath-sc
  volumeMode: Filesystem
  volumeName: pvc-e7cecbe9-524c-4071-bfb2-6e145269c245
`, pvcName)
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(yaml.Unmarshal([]byte(pvcYAML), pvc)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, pvc)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, &corev1.PersistentVolumeClaim{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return pvc
	}

	handlePersistentVolumeClaim := func(reqCtx intctrlutil.RequestCtx, cli client.Client, pvc *corev1.PersistentVolumeClaim) error {
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Annotations["kubeblocks.io/test"] = "test_pvc"
		return cli.Patch(ctx, pvc, patch)
	}

	Context("test creating PersistentVolumeClaim", func() {
		It("should handle it properly", func() {
			By("test PersistentVolumeClaim changes")
			PersistentVolumeClaimHandlerMap["pvc-controller"] = handlePersistentVolumeClaim
			pvcName := fmt.Sprintf("pvc-%s", testCtx.GetRandomStr())
			createPVC(pvcName)
			pvc := &corev1.PersistentVolumeClaim{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, pvc)).Should(Succeed())
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			Expect(k8sClient.Update(ctx, pvc))

			// wait until storageClass patched
			Eventually(func() bool {
				tmpPVC := &corev1.PersistentVolumeClaim{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, tmpPVC)
				return tmpPVC.Annotations["kubeblocks.io/test"] == "test_pvc"
			}, timeout, interval).Should(BeTrue())
		})
	})
})
