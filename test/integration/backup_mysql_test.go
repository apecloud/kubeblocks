/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package appstest

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("MySQL data protection function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const scriptConfigName = "test-cluster-mysql-scripts"
	const mysqlCompDefName = "replicasets"
	const mysqlCompName = "mysql"
	const backupPolicyTemplateName = "test-backup-policy-template"
	const backupPolicyName = "test-backup-policy"
	const backupRemotePVCName = "backup-remote-pvc"
	const backupName = "test-backup-job"

	// Cleanups

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupSignature, true, inNS)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ActionSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.RestoreSignature, inNS, ml)

	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	// Testcases

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
		backupKey         types.NamespacedName
	)

	createClusterObj := func() {
		By("Create configmap")
		_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-scripts.yaml", &corev1.ConfigMap{},
			testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

		By("Create a clusterDef obj")
		mode := int32(0755)
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.ConsensusMySQLComponent, mysqlCompDefName).
			AddScriptTemplate(scriptConfigName, scriptConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, &mode).
			Create(&testCtx).GetObject()

		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponentVersion(mysqlCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			Create(&testCtx).GetObject()

		By("Create a cluster obj")
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompDefName).
			SetReplicas(1).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("check cluster running")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.RunningClusterPhase))
		})).Should(Succeed())
	}

	createBackupObj := func() {
		By("By creating a actionSet")
		actionSet := testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
			&dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())

		By("By creating a backupPolicy from backupPolicyTemplate: " + backupPolicyTemplateName)
		backupPolicyObj := testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
			WithRandomName().
			SetTarget(constant.AppInstanceLabelKey, clusterKey.Name).
			SetTargetConnectionCredential(constant.GenerateDefaultConnCredential(clusterKey.Name)).
			AddBackupMethod(testdp.BackupMethodName, false, actionSet.Name).
			SetBackupMethodVolumeMounts(testapps.DataVolumeName, "/data").
			Create(&testCtx).GetObject()
		backupPolicyKey := client.ObjectKeyFromObject(backupPolicyObj)

		By("By create remove pvc")
		testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, backupRemotePVCName, clusterKey.Name,
			"none", "remote-volume").
			SetAnnotations(map[string]string{}).
			SetStorage("1Gi").
			Create(&testCtx)

		By("By check backupPolicy available")
		Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, backupPolicy *dpv1alpha1.BackupPolicy) {
			g.Expect(backupPolicy.Status.Phase).To(Equal(dpv1alpha1.BackupPolicyAvailable))
		})).Should(Succeed())

		By("By creating a backup from backupPolicy: " + backupPolicyKey.Name)
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
			WithRandomName().
			SetBackupPolicyName(backupPolicyKey.Name).
			SetBackupMethod(testdp.BackupMethodName).
			Create(&testCtx).GetObject()
		backupKey = client.ObjectKeyFromObject(backup)
	}

	Context("with MySQL full backup", func() {
		BeforeEach(func() {
			createClusterObj()
			createBackupObj()
		})

		It("should be completed", func() {
			Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dpv1alpha1.Backup) {
				g.Expect(backup.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
			})).Should(Succeed())
		})
	})
})
