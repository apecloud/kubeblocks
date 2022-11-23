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
	"context"
	"fmt"
	"time"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("Event Controller", func() {
	var (
		timeout     = time.Second * 20
		interval    = time.Second
		clusterName = "wesql-for-storageclass"
		ctx         = context.Background()
	)

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &storagev1.StorageClass{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

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
		Expect(testCtx.CheckedCreateObj(ctx, sc)).Should(Succeed())
		return sc
	}

	createCluster := func(storageClassName string) *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
       kubeblocks.io/ops-request: |
          {"Updating":"wesql-restart-test"}
       kubeblocks.io/storage-class: csi-hostpath-sc,standard
  labels:
    appversion.kubeblocks.io/name: app-version-for-storageclass
    clusterdefinition.kubeblocks.io/name: cluster-definition-for-storageclass
  name: %s
  namespace: default
spec:
  appVersionRef: app-version-for-storageclass
  clusterDefinitionRef: cluster-definition-for-storageclass
  components:
  - monitor: false
    name: wesql-test
    replicas: 3
    type: replicasets
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem
    - name: log
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem  
        storageClassName: %s
  terminationPolicy: WipeOut`, clusterName, storageClassName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), cluster)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, &dbaasv1alpha1.Cluster{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return cluster
	}

	createPvc := func(pvcName, storageClassName string) {
		pvcYaml := fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: hostpath.csi.k8s.io
  labels:
    app.kubernetes.io/component-name: wesql-test
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: %s
  volumeMode: Filesystem
  volumeName: pvc-185c837d-16ea-4bef-b5f0-e3853722407b
`, clusterName, pvcName, storageClassName)
		pvc := &corev1.PersistentVolumeClaim{}
		Expect(yaml.Unmarshal([]byte(pvcYaml), pvc)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), pvc)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: pvcName, Namespace: testCtx.DefaultNamespace}, &corev1.PersistentVolumeClaim{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	Context("When receiving role changed event", func() {
		It("should handle it properly", func() {
			By("init resources")
			vctName1 := "data"
			defaultStorageClassName := "standard"
			storageClassName := "csi-hostpath-sc"
			createCluster(storageClassName)
			cluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Operations = &dbaasv1alpha1.Operations{}
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.ObservedGeneration = 1
			Expect(k8sClient.Status().Patch(ctx, cluster, patch))
			reqCtx := intctrlutil.RequestCtx{
				Ctx: context.Background(),
			}
			defaultStorageClassObj := createStorageClassObj(defaultStorageClassName, true)
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, defaultStorageClassObj)).Should(Succeed())
			storageClassObj := createStorageClassObj(storageClassName, false)
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, storageClassObj)).Should(Succeed())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				return len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data,log]")
			createPvc(fmt.Sprintf("log-%s-wesql-test", clusterName), storageClassName)
			createPvc(fmt.Sprintf("data-%s-wesql-test", clusterName), defaultStorageClassName)
			// set storageClass allowVolumeExpansion to true
			storageClass := &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion := true
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, storageClass)).Should(Succeed())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				componentVolumeExpandable := newCluster.Status.Operations.VolumeExpandable[0]
				return len(componentVolumeExpandable.VolumeClaimTemplateNames) > 1 && componentVolumeExpandable.VolumeClaimTemplateNames[1] == "log"
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			// set storageClass allowVolumeExpansion to false
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion = false
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, storageClass)).Should(Succeed())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				componentVolumeExpandable := newCluster.Status.Operations.VolumeExpandable[0]
				return len(componentVolumeExpandable.VolumeClaimTemplateNames) == 1 && componentVolumeExpandable.VolumeClaimTemplateNames[0] == vctName1
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component not support volume expansion")
			// set defaultStorageClass allowVolumeExpansion to false
			defaultStorageClass := &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClassName}, defaultStorageClass)).Should(Succeed())
			allowVolumeExpansion = false
			defaultStorageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, defaultStorageClass)).Should(Succeed())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				return len(newCluster.Status.Operations.VolumeExpandable) == 0
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			// set defaultStorageClass allowVolumeExpansion to true
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClassName}, defaultStorageClass)).Should(Succeed())
			allowVolumeExpansion = true
			defaultStorageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(handleClusterVolumeExpansion(reqCtx, k8sClient, defaultStorageClass)).Should(Succeed())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				return len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1
			}, timeout, interval).Should(BeTrue())
		})
	})
})
