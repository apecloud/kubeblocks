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

package dataprotection

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("BackupPolicyDriver Controller test", func() {
	const (
		defaultCompName         = "test"
		defaultShardingCompName = "test-shard"
		clusterName             = "test-cluster"
		compDefName             = "test-compdef"
	)

	var (
		compDefObj *appsv1.ComponentDefinition
		clusterKey types.NamespacedName
		bpt        *dpv1alpha1.BackupPolicyTemplate
	)
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.ComponentDefinitionSignature, ml)

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupScheduleSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS)
	}

	createObjets := func() {
		By("Create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("Create a bpt obj")
		bpt = testdp.CreateBackupPolicyTpl(&testCtx, compDefObj.Name)

		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(bpt),
			func(g Gomega, bpt *dpv1alpha1.BackupPolicyTemplate) {
				g.Expect(bpt.Status.Phase).Should(Equal(dpv1alpha1.AvailablePhase))
				g.Expect(bpt.Labels[compDefObj.Name]).Should(Equal(compDefObj.Name))
			})).Should(Succeed())
	}

	BeforeEach(func() {
		cleanEnv()
		createObjets()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("expect to create a backup policy", func() {

		It("test backup policy for a general cluster", func() {
			By("creating a cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddComponent(defaultCompName, compDefObj.Name).SetReplicas(1).
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
				WithRandomName().Create(&testCtx).GetObject()

			By("checking backup policy")
			backupPolicyName := generateBackupPolicyName(clusterObj.Name, defaultCompName)
			backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterObj.Namespace}
			backupPolicy := &dpv1alpha1.BackupPolicy{}
			Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, backupPolicy, true)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
				g.Expect(policy.Spec.BackupMethods).ShouldNot(BeEmpty())
				g.Expect(policy.Spec.Targets).Should(HaveLen(0))
				g.Expect(policy.Spec.Target).ShouldNot(BeNil())
				g.Expect(policy.Spec.Target.Name).Should(BeEmpty())
			})).Should(Succeed())

			By("checking backup schedule")
			backupScheduleName := generateBackupScheduleName(clusterObj.Name, defaultCompName)
			backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterObj.Namespace}
			Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
				&dpv1alpha1.BackupSchedule{}, true)).Should(Succeed())

			By("sync from backup policy template")
			Expect(testapps.ChangeObj(&testCtx, bpt, func(template *dpv1alpha1.BackupPolicyTemplate) {
				template.Spec.Target.Strategy = dpv1alpha1.PodSelectionStrategyAll
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
				g.Expect(policy.Spec.Target.PodSelector).ShouldNot(BeNil())
				g.Expect(policy.Spec.Target.PodSelector.Strategy).Should(Equal(dpv1alpha1.PodSelectionStrategyAll))
			}))

			By("not sync from backup policy template")
			// 1. disable sync from template
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
				bp.Annotations[disableSyncFromTemplateAnnotation] = "true"
			})).Should(Succeed())
			// 2. update bpt
			Expect(testapps.ChangeObj(&testCtx, bpt, func(template *dpv1alpha1.BackupPolicyTemplate) {
				template.Spec.Target.Strategy = dpv1alpha1.PodSelectionStrategyAny
			})).Should(Succeed())
			// 3. update backup policy
			Expect(testapps.ChangeObj(&testCtx, bpt, func(template *dpv1alpha1.BackupPolicyTemplate) {
				template.Spec.BackoffLimit = pointer.Int32(int32(10))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
				g.Expect(*policy.Spec.BackoffLimit).Should(BeEquivalentTo(10))
				g.Expect(policy.Spec.Target.PodSelector.Strategy).Should(Equal(dpv1alpha1.PodSelectionStrategyAll))
			}))
		})

		It("test backup policy for a sharding cluster", func() {
			By("creating a sharding cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
				AddSharding(defaultShardingCompName, "", compDefObj.Name).SetReplicas(1).
				SetShards(3).
				WithRandomName().Create(&testCtx).GetObject()

			By("mock components")
			testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterObj.Name+"-"+defaultShardingCompName, "").
				WithRandomName().
				AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
				AddLabels(constant.AppInstanceLabelKey, clusterObj.Name).
				AddLabels(constant.KBAppShardingNameLabelKey, defaultShardingCompName).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()
			testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterObj.Name+"-"+defaultShardingCompName, "").
				WithRandomName().
				AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
				AddLabels(constant.AppInstanceLabelKey, clusterObj.Name).
				AddLabels(constant.KBAppShardingNameLabelKey, defaultShardingCompName).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()
			testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterObj.Name+"-"+defaultShardingCompName, "").
				WithRandomName().
				AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
				AddLabels(constant.AppInstanceLabelKey, clusterObj.Name).
				AddLabels(constant.KBAppShardingNameLabelKey, defaultShardingCompName).
				SetReplicas(1).
				Create(&testCtx).
				GetObject()

			By("checking backup policy")
			backupPolicyName := generateBackupPolicyName(clusterObj.Name, defaultShardingCompName)
			backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterObj.Namespace}
			backupPolicy := &dpv1alpha1.BackupPolicy{}
			Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, backupPolicy, true)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
				g.Expect(policy.Spec.BackupMethods).ShouldNot(BeEmpty())
				g.Expect(policy.Spec.Targets).Should(HaveLen(3))
				g.Expect(policy.Spec.Target).Should(BeNil())
			})).Should(Succeed())

			By("checking backup schedule")
			backupScheduleName := generateBackupScheduleName(clusterObj.Name, defaultShardingCompName)
			backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterObj.Namespace}
			Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
				&dpv1alpha1.BackupSchedule{}, true)).Should(Succeed())
		})
	})

	Context("cluster with backup", func() {
		const (
			backupRepoName = "test-backup-repo"
		)

		BeforeEach(func() {
			cleanEnv()
			createObjets()
		})

		createClusterWithBackup := func(backup *appsv1.ClusterBackup) {
			By("Creating a cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddComponent(defaultCompName, compDefObj.Name).
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
				SetBackup(backup).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)
		}

		It("Creating cluster without backup", func() {
			createClusterWithBackup(nil)
			Eventually(testapps.List(&testCtx, generics.BackupPolicySignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEmpty())
		})

		It("Creating cluster with backup", func() {
			var (
				boolTrue  = true
				boolFalse = false
				int64Ptr  = func(in int64) *int64 {
					return &in
				}
				retention = func(s string) dpv1alpha1.RetentionPeriod {
					return dpv1alpha1.RetentionPeriod(s)
				}
			)

			var testCases = []struct {
				desc   string
				backup *appsv1.ClusterBackup
			}{
				{
					desc: "backup with snapshot method",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("1d"),
						Method:                  testdp.VSBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "backup with snapshot method and specified continuous method",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("1d"),
						Method:                  testdp.VSBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						ContinuousMethod:        testdp.ContinuousMethodName1,
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "disable backup",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolFalse,
						RetentionPeriod:         retention("1d"),
						Method:                  testdp.VSBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "backup with backup tool",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("2d"),
						Method:                  testdp.BackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						RepoName:                backupRepoName,
						PITREnabled:             &boolFalse,
					},
				},
				{
					desc:   "backup is nil",
					backup: nil,
				},
			}

			for _, t := range testCases {
				By(t.desc)
				backup := t.backup
				createClusterWithBackup(backup)

				checkSchedule := func(g Gomega, schedule *dpv1alpha1.BackupSchedule) {
					var policy *dpv1alpha1.SchedulePolicy
					hasCheckPITRMethod := false
					for i := range schedule.Spec.Schedules {
						s := &schedule.Spec.Schedules[i]
						if s.BackupMethod == backup.Method {
							Expect(*s.Enabled).Should(BeEquivalentTo(*backup.Enabled))
							policy = s
							continue
						}
						if !slices.Contains([]string{testdp.ContinuousMethodName, testdp.ContinuousMethodName1}, s.BackupMethod) {
							if boolptr.IsSetToTrue(backup.Enabled) {
								// another full backup method should be disabled.
								Expect(*s.Enabled).Should(BeFalse())
							}
							continue
						}
						if len(backup.ContinuousMethod) == 0 {
							// first continuous backup method should be equal to "PITREnabled", another is disabled.
							if !hasCheckPITRMethod {
								Expect(*s.Enabled).Should(BeEquivalentTo(*backup.PITREnabled))
								hasCheckPITRMethod = true
							} else {
								Expect(*s.Enabled).Should(BeFalse())
							}
						} else {
							// specified continuous backup method should be equal to "PITREnabled", another is disabled.
							if backup.ContinuousMethod == s.BackupMethod {
								Expect(*s.Enabled).Should(BeEquivalentTo(*backup.PITREnabled))
							} else {
								Expect(*s.Enabled).Should(BeFalse())
							}
						}
					}
					if backup.Enabled != nil && *backup.Enabled {
						Expect(policy).ShouldNot(BeNil())
						Expect(policy.RetentionPeriod).Should(BeEquivalentTo(backup.RetentionPeriod))
						Expect(policy.CronExpression).Should(BeEquivalentTo(backup.CronExpression))
					}
				}

				checkPolicy := func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
					if backup != nil && backup.RepoName != "" {
						g.Expect(*policy.Spec.BackupRepoName).Should(BeEquivalentTo(backup.RepoName))
					}
					g.Expect(policy.Spec.BackupMethods).ShouldNot(BeEmpty())
					g.Expect(policy.Spec.Targets).Should(HaveLen(0))
					g.Expect(policy.Spec.Target).ShouldNot(BeNil())
					g.Expect(policy.Spec.Target.Name).Should(BeEmpty())
				}

				By("checking backup policy")
				backupPolicyName := generateBackupPolicyName(clusterKey.Name, defaultCompName)
				backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterKey.Namespace}
				backupPolicy := &dpv1alpha1.BackupPolicy{}
				Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, backupPolicy, true)).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, checkPolicy)).Should(Succeed())

				By("checking backup schedule")
				backupScheduleName := generateBackupScheduleName(clusterKey.Name, defaultCompName)
				backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterKey.Namespace}
				if backup == nil {
					Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
						&dpv1alpha1.BackupSchedule{}, true)).Should(Succeed())
					continue
				}
				Eventually(testapps.CheckObj(&testCtx, backupScheduleKey, checkSchedule)).Should(Succeed())
			}
		})
	})
})
