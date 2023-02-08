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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("Reconcile StorageClass", func() {
	var (
		clusterDefName     = "cluster-def-" + testCtx.GetRandomStr()
		clusterVersionName = "app-versoion-" + testCtx.GetRandomStr()
		clusterName        = "mysql-for-storageclass-" + testCtx.GetRandomStr()
		consensusCompName  = "consensus"
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

	createStorageClassObj := func(storageClassName string, allowVolumeExpansion bool) {
		By("By assure an default storageClass")
		testdbaas.CreateCustomizedObj(&testCtx, "operations/storageclass.yaml",
			&storagev1.StorageClass{}, testdbaas.CustomizeObjYAML(storageClassName, allowVolumeExpansion))
	}

	createCluster := func(defaultStorageClassName, storageClassName string) *dbaasv1alpha1.Cluster {
		objBytes, err := testdata.GetTestDataFileContent("consensusset/wesql.yaml")
		Expect(err).Should(Succeed())
		clusterString := fmt.Sprintf(string(objBytes), clusterVersionName, clusterDefName, clusterName,
			clusterVersionName, clusterDefName, consensusCompName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterString), cluster)).Should(Succeed())
		volumeClaimTemplates := cluster.Spec.Components[0].VolumeClaimTemplates
		volumeClaimTemplates[0].Spec.StorageClassName = &defaultStorageClassName
		volumeClaimTemplates = append(volumeClaimTemplates, dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
			Name: "log",
			Spec: &corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("2Gi"),
					},
				},
			},
		})
		cluster.Spec.Components[0].VolumeClaimTemplates = volumeClaimTemplates
		cluster.Annotations = map[string]string{
			intctrlutil.StorageClassAnnotationKey: defaultStorageClassName + "," + storageClassName,
		}
		Expect(testCtx.CreateObj(context.Background(), cluster)).Should(Succeed())
		return cluster
	}

	createPVC := func(pvcName, storageClassName string) {
		testdbaas.CreateCustomizedObj(&testCtx, "operations/volume_expansion_pvc.yaml",
			&corev1.PersistentVolumeClaim{}, testdbaas.CustomizeObjYAML(consensusCompName, clusterName, pvcName, pvcName, storageClassName))
	}

	updateStorageClassAllowVolumeExpansion := func(storageClassName string, allowVolumeExpansion bool) {
		Expect(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKey{Name: storageClassName}, func(tmpSc *storagev1.StorageClass) {
			tmpSc.AllowVolumeExpansion = &allowVolumeExpansion
		})()).Should(Succeed())

		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKey{Name: storageClassName}, func(g Gomega, tmpSc *storagev1.StorageClass) {
			g.Expect(*tmpSc.AllowVolumeExpansion).Should(Equal(allowVolumeExpansion))
		})).Should(Succeed())
	}

	Context("reconcile the Cluster.status.operation.volumeExpandable when StorageClass and PVC changed", func() {
		It("should handle it properly", func() {
			By("init resources")
			vctName1 := "data"
			defaultStorageClassName := "standard-" + testCtx.GetRandomStr()
			storageClassName := "csi-hostpath-sc-" + testCtx.GetRandomStr()
			testdbaas.CreateConsensusMysqlClusterDef(testCtx, clusterDefName)
			testdbaas.CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName)
			cluster := createCluster(defaultStorageClassName, storageClassName)
			Expect(testdbaas.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(newCluster *dbaasv1alpha1.Cluster) {
				newCluster.Status.Operations = &dbaasv1alpha1.Operations{}
				newCluster.Status.Phase = dbaasv1alpha1.RunningPhase
				newCluster.Status.ObservedGeneration = 1
			})()).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				g.Expect(newCluster.Status.Operations != nil).Should(BeTrue())
			})).Should(Succeed())

			By("test without pvc")
			createStorageClassObj(defaultStorageClassName, true)

			By("expect consensus component support volume expansion and volumeClaimTemplateNames is [data]")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				g.Expect(len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1).Should(BeTrue())
			})).Should(Succeed())

			By("test with pvc")
			createStorageClassObj(storageClassName, false)
			createPVC(fmt.Sprintf("log-%s-%s", clusterName, consensusCompName), storageClassName)
			createPVC(fmt.Sprintf("data-%s-%s", clusterName, consensusCompName), defaultStorageClassName)

			By("expect consensus component support volume expansion and volumeClaimTemplateNames is [data,log]")
			// set storageClass allowVolumeExpansion to true
			updateStorageClassAllowVolumeExpansion(storageClassName, true)

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				g.Expect(len(volumeExpandable) > 0 && len(volumeExpandable[0].VolumeClaimTemplateNames) > 1 && volumeExpandable[0].VolumeClaimTemplateNames[1] == "log").Should(BeTrue())
			})).Should(Succeed())

			By("expect consensus component support volume expansion and volumeClaimTemplateNames is [data]")
			// set storageClass allowVolumeExpansion to false
			updateStorageClassAllowVolumeExpansion(storageClassName, false)
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				componentVolumeExpandable := newCluster.Status.Operations.VolumeExpandable[0]
				g.Expect(len(componentVolumeExpandable.VolumeClaimTemplateNames) == 1 && componentVolumeExpandable.VolumeClaimTemplateNames[0] == vctName1).Should(BeTrue())
			})).Should(Succeed())

			By("expect consensus component not support volume expansion")
			// set defaultStorageClass allowVolumeExpansion to false
			updateStorageClassAllowVolumeExpansion(defaultStorageClassName, false)
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				g.Expect(len(newCluster.Status.Operations.VolumeExpandable) == 0).Should(BeTrue())
			})).Should(Succeed())

			By("expect consensus component support volume expansion and volumeClaimTemplateNames is [data]")
			// set defaultStorageClass allowVolumeExpansion to true
			updateStorageClassAllowVolumeExpansion(defaultStorageClassName, true)
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, newCluster *dbaasv1alpha1.Cluster) {
				volumeExpandable := newCluster.Status.Operations.VolumeExpandable
				g.Expect(len(volumeExpandable) > 0 && volumeExpandable[0].VolumeClaimTemplateNames[0] == vctName1).Should(BeTrue())
			})).Should(Succeed())
		})
	})
})
