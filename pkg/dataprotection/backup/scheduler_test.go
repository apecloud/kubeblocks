/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package backup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func newSchedulerTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, batchv1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))
	assert.NoError(t, opsv1alpha1.AddToScheme(scheme))
	return scheme
}

func newSchedulerForTest(t *testing.T, objs ...client.Object) *Scheduler {
	scheme := newSchedulerTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: v1.ObjectMeta{
			Name:      "schedule",
			Namespace: "ns",
			UID:       types.UID("schedule-uid"),
			Labels:    map[string]string{"custom": "label"},
		},
		Spec: dpv1alpha1.BackupScheduleSpec{BackupPolicyName: "policy"},
	}
	policy := &dpv1alpha1.BackupPolicy{
		ObjectMeta: v1.ObjectMeta{Name: "policy", Namespace: "ns"},
		Spec: dpv1alpha1.BackupPolicySpec{
			BackoffLimit: pointer.Int32(2),
			Target: &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{LabelSelector: &v1.LabelSelector{MatchLabels: map[string]string{
				constant.AppInstanceLabelKey: "cluster",
			}}}},
			BackupMethods: []dpv1alpha1.BackupMethod{{Name: "full", ActionSetName: "full-action"}, {Name: "incremental", ActionSetName: "incremental-action", CompatibleMethod: "full"}},
		},
	}
	return &Scheduler{
		RequestCtx:           ctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "ns", Name: "schedule"}}},
		Client:               cli,
		Scheme:               scheme,
		BackupSchedule:       schedule,
		BackupPolicy:         policy,
		WorkerServiceAccount: "worker",
	}
}

func TestSchedulerBuildCronJobAndPodSpec(t *testing.T) {
	oldToolsImage := viper.GetString(constant.KBToolsImage)
	defer viper.Set(constant.KBToolsImage, oldToolsImage)
	viper.Set(constant.KBToolsImage, "tools:test")

	fullActionSet := &dpv1alpha1.ActionSet{ObjectMeta: v1.ObjectMeta{Name: "full-action"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull}}
	incrementalActionSet := &dpv1alpha1.ActionSet{ObjectMeta: v1.ObjectMeta{Name: "incremental-action"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeIncremental}}
	s := newSchedulerForTest(t, fullActionSet, incrementalActionSet)
	sp := &dpv1alpha1.SchedulePolicy{
		Name: "daily", BackupMethod: "incremental", Enabled: pointer.Bool(true),
		CronExpression: "0 1 * * *", RetentionPeriod: "7d",
		Parameters: []dpv1alpha1.ParameterPair{{Name: "p", Value: "v"}},
	}
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{*sp}

	assert.Equal(t, "cluster-daily-$(date -u +'%Y%m%d%H%M%S')", s.generateBackupName(sp))

	checkCommand, err := s.buildCheckCommand(sp)
	assert.NoError(t, err)
	assert.Contains(t, checkCommand, "No completed full backups found")
	assert.Contains(t, checkCommand, "spec.backupMethod==\"full\"")

	podSpec, err := s.buildPodSpec(sp)
	assert.NoError(t, err)
	assert.Equal(t, "worker", podSpec.ServiceAccountName)
	assert.Equal(t, corev1.RestartPolicyNever, podSpec.RestartPolicy)
	assert.Equal(t, "tools:test", podSpec.Containers[0].Image)
	assert.Contains(t, podSpec.Containers[0].Args[0], "parameters:")
	assert.Contains(t, podSpec.Containers[0].Args[0], "backupMethod: incremental")

	cronJob, err := s.buildCronJob(sp, "")
	assert.NoError(t, err)
	assert.Equal(t, GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, sp.BackupMethod, sp.Name), cronJob.Name)
	assert.Equal(t, batchv1.ForbidConcurrent, cronJob.Spec.ConcurrencyPolicy)
	assert.Equal(t, "label", cronJob.Labels["custom"])
	assert.Equal(t, dptypes.AppName, cronJob.Labels[constant.AppManagedByLabelKey])
	assert.Contains(t, cronJob.Finalizers, dptypes.DataProtectionFinalizerName)
}

func TestSchedulerReconcileCronJobCreatesPatchesAndDeletes(t *testing.T) {
	fullActionSet := &dpv1alpha1.ActionSet{ObjectMeta: v1.ObjectMeta{Name: "full-action"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull}}
	s := newSchedulerForTest(t, fullActionSet)
	sp := &dpv1alpha1.SchedulePolicy{Name: "daily", BackupMethod: "full", Enabled: pointer.Bool(true), CronExpression: "0 1 * * *", RetentionPeriod: "7d"}
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{*sp}
	deadline := int64(5)
	s.BackupSchedule.Spec.StartingDeadlineMinutes = &deadline

	assert.NoError(t, s.reconcileCronJob(sp))
	cronJobName := GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, sp.BackupMethod, sp.Name)
	cronJob := &batchv1.CronJob{}
	assert.NoError(t, s.Client.Get(context.Background(), client.ObjectKey{Name: cronJobName, Namespace: "ns"}, cronJob))
	assert.Equal(t, int64(300), *cronJob.Spec.StartingDeadlineSeconds)

	sp.CronExpression = "0 2 * * *"
	assert.NoError(t, s.reconcileCronJob(sp))
	assert.NoError(t, s.Client.Get(context.Background(), client.ObjectKey{Name: cronJobName, Namespace: "ns"}, cronJob))
	assert.True(t, strings.Contains(cronJob.Spec.Schedule, "0 2 * * *"))

	sp.Enabled = pointer.Bool(false)
	assert.NoError(t, s.reconcileCronJob(sp))
	err := s.Client.Get(context.Background(), client.ObjectKey{Name: cronJobName, Namespace: "ns"}, cronJob)
	assert.True(t, client.IgnoreNotFound(err) == nil)
}

func TestSchedulerConfigAnnotationHelpers(t *testing.T) {
	s := newSchedulerForTest(t)
	s.BackupSchedule.Annotations = map[string]string{
		constant.LastAppliedConfigAnnotationKey: `{"old":"config"}`,
		dptypes.ReConfigureGenerationKey:        "2",
	}

	s.convertLastAppliedConfigs("continuous")
	assert.Contains(t, s.BackupSchedule.Annotations, dptypes.LastAppliedConfigsAnnotationKey)
	configs, err := s.getLastAppliedConfigsMap()
	assert.NoError(t, err)
	assert.Equal(t, `{"old":"config"}`, configs["continuous"])

	generation, err := s.getReconfigureGenerationKey()
	assert.NoError(t, err)
	assert.Equal(t, 2, generation)

	s.BackupSchedule.Annotations[dptypes.ReConfigureGenerationKey] = "bad"
	_, err = s.getReconfigureGenerationKey()
	assert.Error(t, err)

	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = "{bad"
	_, err = s.getLastAppliedConfigsMap()
	assert.Error(t, err)
}

func TestSchedulerReconfigureEarlyReturns(t *testing.T) {
	s := newSchedulerForTest(t)
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full", Enabled: pointer.Bool(false)}
	assert.NoError(t, s.reconfigure(sp))

	value := "v"
	refBytes, err := json.Marshal(backupReconfigureRef{Disable: parameterPairs{"full": []opsv1alpha1.ParameterPair{{Key: "p", Value: &value}}}})
	assert.NoError(t, err)
	s.BackupSchedule.Annotations = map[string]string{dptypes.ReconfigureRefAnnotationKey: string(refBytes)}
	assert.NoError(t, s.reconfigure(sp))

	sp.Enabled = pointer.Bool(true)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"[{\"name\":\"p\",\"value\":\"v\"}]"}`
	assert.NoError(t, s.reconfigure(sp))
}

func TestSchedulerReconcileReconfigureOpsStates(t *testing.T) {
	failedOps := &opsv1alpha1.OpsRequest{
		ObjectMeta: v1.ObjectMeta{
			Name: "failed", Namespace: "ns",
			CreationTimestamp: v1.Now(),
			Labels:            map[string]string{dptypes.BackupScheduleLabelKey: "schedule"},
		},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsFailedPhase},
	}
	s := newSchedulerForTest(t, failedOps)
	err := s.reconcileReconfigure(s.BackupSchedule)
	assert.Error(t, err)

	runningOps := failedOps.DeepCopy()
	runningOps.Name = "running"
	runningOps.Status.Phase = opsv1alpha1.OpsRunningPhase
	s = newSchedulerForTest(t, runningOps)
	err = s.reconcileReconfigure(s.BackupSchedule)
	assert.Error(t, err)

	succeedOps := failedOps.DeepCopy()
	succeedOps.Name = "succeed"
	succeedOps.Status.Phase = opsv1alpha1.OpsSucceedPhase
	s = newSchedulerForTest(t, succeedOps)
	assert.NoError(t, s.reconcileReconfigure(s.BackupSchedule))
}

var _ = Describe("Scheduler Test", func() {
	buildScheduler := func() *Scheduler {
		return &Scheduler{
			RequestCtx: ctrlutil.RequestCtx{
				Log:      logger,
				Ctx:      testCtx.Ctx,
				Recorder: recorder,
			},
			Client: testCtx.Cli,
		}
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, ml, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupScheduleSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		var (
			backupPolicy   *dpv1alpha1.BackupPolicy
			backupSchedule *dpv1alpha1.BackupSchedule
			actionSet      *dpv1alpha1.ActionSet
			scheduler      *Scheduler
			cluster        *appsv1.Cluster
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			clusterInfo := testdp.NewFakeCluster(&testCtx)
			cluster = clusterInfo.Cluster
			backupPolicy = testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, testdp.BackupPolicyName).
				SetBackupRepoName(testdp.BackupRepoName).
				SetPathPrefix(testdp.BackupPathPrefix).
				SetTarget(constant.AppInstanceLabelKey, testdp.ClusterName,
					constant.KBAppComponentLabelKey, testdp.ComponentName,
					constant.RoleLabelKey, testapps.Leader).
				AddBackupMethod(testdp.BackupMethodName, false, actionSet.Name).
				AddBackupMethod(testdp.VSBackupMethodName, true, "").
				Create(&testCtx).GetObject()
			backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
				schedule.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: appsv1.APIVersion,
						Kind:       "Cluster",
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				}
			})

			scheduler = buildScheduler()
		})

		Context("test Schedule", func() {
			It("should schedule", func() {
				scheduler.BackupSchedule = backupSchedule
				scheduler.BackupPolicy = backupPolicy
				Expect(scheduler.Schedule()).Should(Succeed())
			})

			It("schedule should fail if invalid backup policy", func() {
				scheduler.BackupSchedule = backupSchedule
				scheduler.BackupPolicy = backupPolicy
				for i := range scheduler.BackupPolicy.Spec.BackupMethods {
					scheduler.BackupPolicy.Spec.BackupMethods[i].Name = "not-exist"
				}
				Expect(scheduler.Schedule()).ShouldNot(Succeed())
			})

			It("test schedule for continuous backup", func() {
				By("set actionSet type to Continuous")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(as *dpv1alpha1.ActionSet) {
					actionSet.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
				})).Should(Succeed())

				By("enable backupMethod and do scheduling")
				Expect(testapps.ChangeObj(&testCtx, backupSchedule, func(schedule *dpv1alpha1.BackupSchedule) {
					for i, v := range backupSchedule.Spec.Schedules {
						if v.BackupMethod == testdp.BackupMethodName {
							backupSchedule.Spec.Schedules[i].Enabled = pointer.Bool(true)
							break
						}
					}
				})).Should(Succeed())

				scheduler.BackupPolicy = backupPolicy
				scheduler.BackupSchedule = backupSchedule
				Expect(scheduler.Schedule()).Should(Succeed())

				By("check the continuous backup created")
				backupName := GenerateCRNameByBackupSchedule(backupSchedule, testdp.BackupMethodName)
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: backupName, Namespace: testCtx.DefaultNamespace},
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("re-create backupSchedule and enable the method")
				Expect(k8sClient.Delete(ctx, backupSchedule)).Should(Succeed())
				backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
					schedule.OwnerReferences = []v1.OwnerReference{
						{
							APIVersion: appsv1.APIVersion,
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
						},
					}
					for i, v := range backupSchedule.Spec.Schedules {
						if v.BackupMethod == testdp.BackupMethodName {
							backupSchedule.Spec.Schedules[i].Enabled = pointer.Bool(true)
							break
						}
					}
				})
				By("Expect only one continuous backup to exist")
				scheduler.BackupSchedule = backupSchedule
				Expect(scheduler.Schedule()).Should(Succeed())
				Eventually(testapps.List(&testCtx, generics.BackupSignature, client.MatchingLabels{
					dptypes.BackupTypeLabelKey:     string(dpv1alpha1.BackupTypeContinuous),
					dptypes.BackupScheduleLabelKey: backupSchedule.Name,
				}, client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(1))
			})
		})
	})
})
