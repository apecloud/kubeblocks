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

package dbaas

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Event Controller", func() {
	var (
		timeout            = time.Second * 10
		interval           = time.Second
		clusterDefName     = "cluster-def-" + testCtx.GetRandomStr()
		clusterVersionName = "app-versoion-" + testCtx.GetRandomStr()
		clusterName        = "mysql-for-storageclass-" + testCtx.GetRandomStr()
		ctx                = context.Background()
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// non-namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.StorageClassSignature, client.HasLabels{testCtx.TestObjLabelKey})
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createClusterDef := func() {
		clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     %s
spec:
  type: state.mysql
  components:
  - typeName: consensus
    componentType: Consensus
    defaultReplicas: 3
    consensusSpec:
      leader:
        name: "leader"
        accessMode: ReadWrite
      followers:
      - name: "follower"
        accessMode: Readonly
    podSpec:
      containers:
      - name: mysql
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 13306
          name: paxos
          protocol: TCP
`, clusterDefName)
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
	}

	createClusterVersion := func() {
		clusterVersionYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:     %s
spec:
  clusterDefinitionRef: %s
  components:
  - type: consensus
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
`, clusterVersionName, clusterDefName)
		clusterVersion := &dbaasv1alpha1.ClusterVersion{}
		Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
	}

	createStorageClassObj := func(storageClassName string, allowVolumeExpansion bool) {
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
	}

	createCluster := func(defaultStorageClassName, storageClassName string) *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  annotations:
       kubeblocks.io/ops-request: |
          {"Updating":"wesql-restart-test"}
       kubeblocks.io/storage-class: %s,%s
  labels:
    clusterversion.kubeblocks.io/name: %s
    clusterdefinition.kubeblocks.io/name: %s
  name: %s
  namespace: default
spec:
  clusterVersionRef: %s
  clusterDefinitionRef: %s
  components:
  - monitor: false
    name: wesql-test
    replicas: 3
    type: consensus
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem
        storageClassName: %s
    - name: log
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
        volumeMode: Filesystem  
        storageClassName: %s
  terminationPolicy: WipeOut`, defaultStorageClassName, storageClassName,
			clusterVersionName, clusterDefName, clusterName, clusterVersionName,
			clusterDefName, defaultStorageClassName, storageClassName)
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

	createPVC := func(pvcName, storageClassName string) {
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
			defaultStorageClassName := "standard-" + testCtx.GetRandomStr()
			storageClassName := "csi-hostpath-sc-" + testCtx.GetRandomStr()
			createClusterDef()
			createClusterVersion()
			createCluster(defaultStorageClassName, storageClassName)
			cluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Operations = &dbaasv1alpha1.Operations{}
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.ObservedGeneration = 1
			Expect(k8sClient.Status().Patch(ctx, cluster, patch))
			Eventually(func() bool {
				tmpCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, tmpCluster)
				return tmpCluster.Status.Operations != nil
			}, timeout, interval).Should(BeTrue())
			createStorageClassObj(defaultStorageClassName, true)

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				return len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1
			}, timeout, interval).Should(BeTrue())

			createStorageClassObj(storageClassName, false)
			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data,log]")
			createPVC(fmt.Sprintf("log-%s-wesql-test", clusterName), storageClassName)
			createPVC(fmt.Sprintf("data-%s-wesql-test", clusterName), defaultStorageClassName)
			// set storageClass allowVolumeExpansion to true
			storageClass := &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion := true
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(k8sClient.Update(ctx, storageClass)).Should(Succeed())
			Eventually(func() bool {
				tmpSc := &storagev1.StorageClass{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, tmpSc)
				allowExpansion := tmpSc.AllowVolumeExpansion
				return *allowExpansion == true
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				return len(volumeExpandable) > 0 && len(volumeExpandable[0].VolumeClaimTemplateNames) > 1 && volumeExpandable[0].VolumeClaimTemplateNames[1] == "log"
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			// set storageClass allowVolumeExpansion to false
			storageClass = &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion = false
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(k8sClient.Update(ctx, storageClass)).Should(Succeed())
			Eventually(func() bool {
				tmpSc := &storagev1.StorageClass{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, tmpSc)
				allowExpansion := tmpSc.AllowVolumeExpansion
				return *allowExpansion == false
			}, timeout, interval).Should(BeTrue())
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
			Expect(k8sClient.Update(ctx, defaultStorageClass)).Should(Succeed())
			Eventually(func() bool {
				tmpSc := &storagev1.StorageClass{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClassName}, tmpSc)
				allowExpansion := tmpSc.AllowVolumeExpansion
				return *allowExpansion == false
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				return len(newCluster.Status.Operations.VolumeExpandable) == 0
			}, timeout, interval).Should(BeTrue())

			By("expect wesql-test component support volume expansion and volumeClaimTemplateNames is [data]")
			// set defaultStorageClass allowVolumeExpansion to true
			defaultStorageClass = &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClassName}, defaultStorageClass)).Should(Succeed())
			allowVolumeExpansion = true
			defaultStorageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(k8sClient.Update(ctx, defaultStorageClass)).Should(Succeed())
			Eventually(func() bool {
				tmpSc := &storagev1.StorageClass{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: defaultStorageClassName}, tmpSc)
				allowExpansion := tmpSc.AllowVolumeExpansion
				return *allowExpansion == true
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				newCluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, newCluster)
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				return len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1
			}, timeout, interval).Should(BeTrue())
		})
	})
})
