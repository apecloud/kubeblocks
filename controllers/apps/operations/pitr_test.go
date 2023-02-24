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

package operations

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("PITR Functions", func() {
	const defaultTTL = "168h0m0s"
	const backupName = "test-backup-job"
	const sourceCluster = "source-cluster"

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-pitr-" + randomStr
		clusterVersionName    = "clusterversion-for-pitr-" + randomStr
		clusterName           = "cluster-for-pitr-" + randomStr
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test PITR", func() {
		var pitrClient PointInTimeRecoveryManager
		var cluster *appsv1alpha1.Cluster

		BeforeEach(func() {
			By("init operations resources ")
			_, _, cluster = initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			pitrClient = PointInTimeRecoveryManager{
				Client:  k8sClient,
				Cluster: cluster,
				Ctx:     ctx,
			}

			By("By creating backup policyTemplate: ")
			backupTplLabels := map[string]string{
				intctrlutil.ClusterDefLabelKey: clusterDefinitionName,
			}
			_ = testapps.NewBackupPolicyTemplateFactory("backup-policy-template").
				WithRandomName().SetLabels(backupTplLabels).
				SetBackupToolName("backup-tool-name").
				SetSchedule("0 * * * *").
				SetTTL(defaultTTL).
				SetCredentialKeyword("username", "password").
				AddHookPreCommand("touch /data/mysql/.restore;sync").
				AddHookPostCommand("rm -f /data/mysql/.restore;sync").
				SetPointInTimeRecovery(&corev1.Container{Image: "111", Args: []string{"111"}}, map[string]string{"111": "222"}).
				Create(&testCtx).GetObject()

			By("By creating earlier backup: ")
			backupLabels := map[string]string{
				intctrlutil.AppNameLabelKey: sourceCluster,
			}
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetTTL(defaultTTL).
				SetBackupPolicyName("test-fake").
				SetBackupType(dpv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()
			now := metav1.Now()
			backupStatus := dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupCompleted,
				StartTimestamp:      &now,
				CompletionTimestamp: &now,
			}
			backupStatus.CompletionTimestamp = &metav1.Time{Time: now.Add(-time.Hour * 2)}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backup))

			By("By creating latest backup: ")
			backupNext := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetTTL(defaultTTL).
				SetBackupPolicyName("test-fake").
				SetBackupType(dpv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()
			backupStatus.CompletionTimestamp = &metav1.Time{Time: now.Add(time.Hour * 2)}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupNext))
		})

		It("Test PITR prepare", func() {
			cluster.SetAnnotations(map[string]string{
				"restore-from-time":    metav1.Now().Format(time.RFC3339),
				"restore-from-cluster": sourceCluster,
			})
			Expect(pitrClient.DoPrepare(cluster)).Should(Succeed())
			/*
				By("mock cluster annotations for start opsRequest")
				// mock snapshot annotation for cluster
				componentReplicasMap := map[string]int32{}
				for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
					componentReplicasMap[v.Name] = v.Replicas
				}
				componentReplicasSnapshot, _ := json.Marshal(componentReplicasMap)
				opsRes.Cluster.Annotations = map[string]string{
					intctrlutil.SnapShotForStartAnnotationKey: string(componentReplicasSnapshot),
				}
				By("create Start opsRequest")
				ops := testapps.NewOpsRequestObj("start-ops-"+randomStr, testCtx.DefaultNamespace,
					clusterName, appsv1alpha1.StartType)
				opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)

				By("test start action and reconcile function")
				startHandler := StartOpsHandler{}
				oldComponentReplicasMap, _ := startHandler.getComponentReplicasSnapshot(opsRes.Cluster.Annotations)
				// do action
				Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
				opsRes.Cluster.Status.Phase = appsv1alpha1.StoppedPhase
				Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
				for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
					oldReplicas := oldComponentReplicasMap[v.Name]
					Expect(oldReplicas == v.Replicas).Should(BeTrue())
				}
				_, err := GetOpsManager().Reconcile(opsRes)
				Expect(err == nil).Should(BeTrue())
			*/
		})

	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}
