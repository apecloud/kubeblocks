/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("component restore test", func() {
	const (
		compDefName     = "test-compdef"
		clusterName     = "test-cluster"
		defaultCompName = "default"
	)

	var (
		compDefObj *kbappsv1.ComponentDefinition
		clusterObj *kbappsv1.Cluster
		clusterKey types.NamespacedName
		settings   map[string]interface{}
	)

	resetTestContext := func() {
		compDefObj = nil
		if settings != nil {
			Expect(viper.MergeConfigMap(settings)).ShouldNot(HaveOccurred())
			settings = nil
		}
	}

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete components (and all dependent sub-resources), and component definitions & versions
		testapps.ClearComponentResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		resetTestContext()
	}

	BeforeEach(func() {
		cleanEnv()
		settings = viper.AllSettings()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createDefinitionObjects := func() {
		By("create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true").
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()
	}

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.ClusterReconciled(&testCtx, clusterKey)).Should(BeTrue())
		cluster := &kbappsv1.Cluster{}
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, cluster, true)).Should(Succeed())
		for _, compName := range compNames {
			compPhase := kbappsv1.CreatingComponentPhase
			for _, spec := range cluster.Spec.ComponentSpecs {
				if spec.Name == compName && spec.Replicas == 0 {
					compPhase = kbappsv1.StoppedComponentPhase
				}
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(compPhase))
		}
	}

	getPVCName := func(vctName, compAndTPLName string, i int) string {
		return fmt.Sprintf("%s-%s-%s-%d", vctName, clusterKey.Name, compAndTPLName, i)
	}

	createPVC := func(clusterName, pvcName, compName, storageSize, storageClassName string) {
		if storageSize == "" {
			storageSize = "1Gi"
		}
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
			compName, testapps.DataVolumeName).
			AddLabelsInMap(map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
				constant.AppManagedByLabelKey:   constant.AppName,
			}).
			SetStorage(storageSize).
			SetStorageClass(storageClassName).
			CheckedCreate(&testCtx)
	}

	mockComponentPVCsAndBound := func(comp *kbappsv1.ClusterComponentSpec, replicas int, create bool, storageClassName string) {
		for i := 0; i < replicas; i++ {
			for _, vct := range comp.VolumeClaimTemplates {
				pvcKey := types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      getPVCName(vct.Name, comp.Name, i),
				}
				if create {
					createPVC(clusterKey.Name, pvcKey.Name, comp.Name, vct.Spec.Resources.Requests.Storage().String(), storageClassName)
				}
				Eventually(testapps.CheckObjExists(&testCtx, pvcKey,
					&corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
					if pvc.Status.Capacity == nil {
						pvc.Status.Capacity = corev1.ResourceList{}
					}
					pvc.Status.Capacity[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				})).Should(Succeed())
			}
		}
	}

	testRestoreComponentFromBackup := func(compName, compDefName string) {
		By("mock backup tool object")
		backupPolicyName := "test-backup-policy"
		backupName := "test-backup"
		_ = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml", &dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())

		By("creating backup")
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
			SetBackupPolicyName(backupPolicyName).
			SetBackupMethod(testdp.BackupMethodName).
			Create(&testCtx).
			GetObject()

		By("mocking backup status completed, we don't need backup reconcile here")
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(backup), func(backup *dpv1alpha1.Backup) {
			backup.Status.PersistentVolumeClaimName = "backup-pvc"
			backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			testdp.MockBackupStatusMethod(backup, testdp.BackupMethodName, testapps.DataVolumeName, testdp.ActionSetName)
		})).Should(Succeed())

		By("creating cluster with backup")
		restoreFromBackup := fmt.Sprintf(`{"%s":{"name":"%s"}}`, compName, backupName)
		pvcSpec := testapps.NewPVCSpec("1Gi")
		replicas := 3
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetReplicas(int32(replicas)).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddAnnotations(constant.RestoreFromBackupAnnotationKey, restoreFromBackup).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		// mock pvcs have restored
		mockComponentPVCsAndBound(clusterObj.Spec.GetComponentByName(compName), replicas, true, testk8s.DefaultStorageClassName)

		By("wait for restore created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterKey.Name,
			constant.KBAppComponentLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.RestoreSignature,
			ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(1))

		By("Mocking restore phase to Completed")
		// mock prepareData restore completed
		testdp.MockRestoreCompleted(&testCtx, ml)

		By("Waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName)

		itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
		its := &itsList.Items[0]
		By("mock pod are available and wait for component enter running phase")
		mockPods := testapps.MockInstanceSetPods(&testCtx, its, clusterObj, compName)
		Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
			testk8s.MockInstanceSetReady(its, mockPods...)
		})).ShouldNot(HaveOccurred())
		Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(kbappsv1.RunningComponentPhase))

		By("clean up annotations after cluster running")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *kbappsv1.Cluster) {
			g.Expect(tmpCluster.Status.Phase).Should(Equal(kbappsv1.RunningClusterPhase))
			// mock postReady restore completed
			testdp.MockRestoreCompleted(&testCtx, ml)
			g.Expect(tmpCluster.Annotations[constant.RestoreFromBackupAnnotationKey]).Should(BeEmpty())
		})).Should(Succeed())
	}

	// TODO: We no longer have the cluster controller running for testing, so the restore cases should be rewritten or the implementation should be refactored.
	PContext("restore", func() {
		BeforeEach(func() {
			createDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("restore component from backup", func() {
			testRestoreComponentFromBackup(defaultCompName, compDefObj.Name)
		})
	})
})
