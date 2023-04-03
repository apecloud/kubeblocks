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

package plan

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("PITR Functions", func() {
	const defaultTTL = "168h0m0s"
	const backupName = "test-backup-job"
	const sourceCluster = "source-cluster"

	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-for-pitr-" + randomStr
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.CronJobSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		//
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test PITR", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			mysqlCompType      = "replicasets"
			mysqlCompName      = "mysql"
			nginxCompType      = "proxy"
		)

		var (
			clusterDef           *appsv1alpha1.ClusterDefinition
			clusterVersion       *appsv1alpha1.ClusterVersion
			cluster              *appsv1alpha1.Cluster
			synthesizedComponent *component.SynthesizedComponent
			pvc                  *corev1.PersistentVolumeClaim
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				Create(&testCtx).GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompType).
				AddInitContainerShort("nginx-init", testapps.NginxImage).
				AddContainerShort("nginx", testapps.NginxImage).
				Create(&testCtx).GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				AddRestorePointInTime(metav1.Time{Time: metav1.Now().Time}, sourceCluster).
				Create(&testCtx).GetObject()

			By("By mocking a pvc")
			pvc = testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, "data-"+clusterName+"-"+mysqlCompName+"-0", clusterName, mysqlCompName, "data").
				SetStorage("1Gi").
				Create(&testCtx).GetObject()

			By("By creating backup policyTemplate: ")
			backupTplLabels := map[string]string{
				constant.ClusterDefLabelKey: clusterDefName,
			}
			_ = testapps.NewBackupPolicyTemplateFactory("backup-policy-template").
				WithRandomName().SetLabels(backupTplLabels).
				SetBackupToolName("backup-tool-name").
				SetSchedule("0 * * * *").
				SetTTL(defaultTTL).
				SetCredentialKeyword("username", "password").
				AddHookPreCommand("touch /data/mysql/.restore;sync").
				AddHookPostCommand("rm -f /data/mysql/.restore;sync").
				SetPointInTimeRecovery(&dpv1alpha1.ScriptSpec{
					Command: []string{"sh", "-c"}, Args: []string{"cp /pitr/log /data/log"}},
					map[string]string{"pg.conf": "recovery-to-time='$KB_RECOVERY_TIME'"}).
				Create(&testCtx).GetObject()

			clusterCompDefObj := clusterDef.Spec.ComponentDefs[0]
			synthesizedComponent = &component.SynthesizedComponent{
				PodSpec:               clusterCompDefObj.PodSpec,
				Probes:                clusterCompDefObj.Probes,
				LogConfigs:            clusterCompDefObj.LogConfigs,
				HorizontalScalePolicy: clusterCompDefObj.HorizontalScalePolicy,
				VolumeClaimTemplates:  cluster.Spec.ComponentSpecs[0].ToVolumeClaimTemplates(),
			}

			By("By creating earlier backup: ")
			now := metav1.Now()
			backupLabels := map[string]string{
				constant.AppInstanceLabelKey: sourceCluster,
			}
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetTTL(defaultTTL).
				SetBackupPolicyName("test-fake").
				SetBackupType(dpv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()
			earlierStartTime := &metav1.Time{Time: now.Add(-time.Hour * 3)}
			earlierStopTime := &metav1.Time{Time: now.Add(-time.Hour * 2)}
			backupStatus := dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupCompleted,
				StartTimestamp:      earlierStartTime,
				CompletionTimestamp: earlierStopTime,
				Manifests: &dpv1alpha1.ManifestsStatus{
					BackupLog: &dpv1alpha1.BackupLogStatus{
						StartTime: earlierStartTime,
						StopTime:  earlierStopTime,
					},
				},
			}
			backupStatus.CompletionTimestamp = &metav1.Time{Time: now.Add(-time.Hour * 2)}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backup))

			By("By creating latest backup: ")
			latestStartTime := &metav1.Time{Time: now.Add(-time.Hour * 3)}
			latestStopTime := &metav1.Time{Time: now.Add(time.Hour * 2)}
			backupNext := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				WithRandomName().SetLabels(backupLabels).
				SetTTL(defaultTTL).
				SetBackupPolicyName("test-fake").
				SetBackupType(dpv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()
			backupStatus = dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupCompleted,
				StartTimestamp:      latestStartTime,
				CompletionTimestamp: latestStopTime,
				Manifests: &dpv1alpha1.ManifestsStatus{
					BackupLog: &dpv1alpha1.BackupLogStatus{
						StartTime: latestStartTime,
						StopTime:  latestStopTime,
					},
				},
			}
			patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupNext))
		})

		It("Test PITR prepare", func() {
			Expect(DoPITRPrepare(ctx, testCtx.Cli, cluster, synthesizedComponent)).Should(Succeed())
			Expect(synthesizedComponent.PodSpec.InitContainers).ShouldNot(BeEmpty())
		})
		It("Test Merge pitr config", func() {
			pitrMgr := PointInTimeRecoveryManager{
				Cluster: cluster,
				Client:  testCtx.Cli,
				Ctx:     ctx,
			}
			configMap := corev1.ConfigMap{
				Data: map[string]string{"pg.conf": "key=value"},
			}
			Expect(pitrMgr.MergeConfigMap(&configMap)).Should(Succeed())
			Expect(configMap.Data).ShouldNot(Equal(map[string]string{"pg.conf": "key=value"}))
		})
		It("Test PITR job run and cleanup", func() {
			By("when data pvc is pending")
			shouldRequeue, err := DoPITRIfNeed(ctx, testCtx.Cli, cluster)
			Expect(err).Should(Succeed())
			Expect(shouldRequeue).Should(BeTrue())
			By("when data pvc is bound")
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(fetched *corev1.PersistentVolumeClaim) {
				fetched.Status.Phase = corev1.ClaimBound
			})).Should(Succeed())
			shouldRequeue, err = DoPITRIfNeed(ctx, testCtx.Cli, cluster)
			Expect(err).Should(Succeed())
			By("when job is completed")
			jobKey := types.NamespacedName{Namespace: cluster.Namespace, Name: "pitr-prepare-" + clusterName}
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, jobKey, func(fetched *batchv1.Job) {
				fetched.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete}}
			})).Should(Succeed())
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(fetched *appsv1alpha1.Cluster) {
				fetched.Status.Phase = appsv1alpha1.RunningClusterPhase
			})).Should(Succeed())
			shouldRequeue, err = DoPITRIfNeed(ctx, testCtx.Cli, cluster)
			Expect(err).Should(Succeed())
			By("cleanup pitr job")
			Expect(DoPITRCleanup(ctx, testCtx.Cli, cluster)).Should(Succeed())
		})
	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}
