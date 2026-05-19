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

package operations

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

var _ = Describe("Restore OpsRequest", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		restoreClusterName = "restore-cluster-" + randomStr
		backupName         = "backup-for-ops-" + randomStr
		restoreOpsName     = "restore-ops-" + randomStr
		reqCtx             intctrlutil.RequestCtx
		restoreHandler     = RestoreOpsHandler{}
	)

	BeforeEach(func() {
		reqCtx = intctrlutil.RequestCtx{Ctx: ctx}
	})

	It("creates Cluster with spec.restore", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		restoreSpec := opsRequest.Spec.GetRestore()
		restoreSpec.DeferPostReadyUntilClusterRunning = true
		backup := newRestoreOpsBackup(backupName, nil)
		cli := newRestoreOpsFakeClient(opsRequest, backup)
		opsRes := &OpsResource{OpsRequest: opsRequest}

		Expect(restoreHandler.Action(reqCtx, cli, opsRes)).Should(Succeed())

		cluster := &appsv1.Cluster{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreClusterName, Namespace: opsRequest.Namespace}, cluster)).Should(Succeed())
		Expect(cluster.Spec.Restore).ShouldNot(BeNil())
		Expect(cluster.Spec.Restore.Source.Name).Should(Equal(backupName))
		Expect(cluster.Spec.Restore.Source.Namespace).Should(Equal(opsRequest.Namespace))
		Expect(cluster.Spec.Restore.PITR).Should(Equal("2026-05-04T08:00:00Z"))
		Expect(cluster.Spec.Restore.Parameters).Should(HaveKeyWithValue("restore-param", "restore-value"))
		Expect(cluster.Spec.Restore.Parameters).Should(HaveKeyWithValue(dptypes.VolumeRestorePolicyParameterKey, string(dpv1alpha1.VolumeClaimRestorePolicySerial)))
		Expect(cluster.Spec.Restore.Parameters).Should(HaveKeyWithValue(dptypes.DeferPostReadyUntilClusterRunningParameterKey, "true"))
		var restoreEnv []corev1.EnvVar
		Expect(json.Unmarshal([]byte(cluster.Spec.Restore.Parameters[dptypes.RestoreEnvParameterKey]), &restoreEnv)).Should(Succeed())
		Expect(restoreEnv).Should(ContainElement(corev1.EnvVar{Name: "RESTORE_ENV", Value: "true"}))
		Expect(cluster.Annotations).ShouldNot(HaveKey("kubeblocks.io/restore-from-backup"))
		Expect(cluster.Labels).Should(HaveKeyWithValue(constant.OpsRequestNameLabelKey, opsRequest.Name))
		Expect(cluster.Labels).Should(HaveKeyWithValue(constant.OpsRequestNamespaceLabelKey, opsRequest.Namespace))
		Expect(cluster.Labels).Should(HaveKeyWithValue(constant.OpsRequestTypeLabelKey, string(opsv1alpha1.RestoreType)))
		Expect(cluster.OwnerReferences).Should(BeEmpty())
	})

	It("re-enters when target Cluster already belongs to the same restore OpsRequest", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		opsRequest.Labels = nil
		backup := newRestoreOpsBackup(backupName, nil)
		existing, err := restoreHandler.getClusterObjFromBackup(backup, opsRequest)
		Expect(err).ShouldNot(HaveOccurred())
		markRestoreClusterWithOps(existing, opsRequest)
		cli := newRestoreOpsFakeClient(opsRequest, backup, existing)
		opsRes := &OpsResource{OpsRequest: opsRequest}

		Expect(restoreHandler.Action(reqCtx, cli, opsRes)).Should(Succeed())

		Expect(opsRes.Cluster.Name).Should(Equal(restoreClusterName))
		Expect(opsRequest.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, restoreClusterName))
		Expect(opsRequest.Labels).Should(HaveKeyWithValue(constant.OpsRequestTypeLabelKey, string(opsv1alpha1.RestoreType)))
	})

	It("fails when target Cluster already exists and is not created by this restore OpsRequest", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		backup := newRestoreOpsBackup(backupName, nil)
		existing := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      restoreClusterName,
				Namespace: opsRequest.Namespace,
			},
		}
		cli := newRestoreOpsFakeClient(opsRequest, backup, existing)

		Expect(restoreHandler.Action(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})).ShouldNot(Succeed())
	})

	It("formats and validates continuous backup restore time", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		opsRequest.Spec.GetRestore().RestorePointInTime = "May 04,2026 16:00:00 UTC+0800"
		backup := newRestoreOpsBackup(backupName, map[string]string{
			dptypes.BackupTypeLabelKey:   string(dpv1alpha1.BackupTypeContinuous),
			constant.AppInstanceLabelKey: restoreClusterName,
		})
		backup.Status.TimeRange = &dpv1alpha1.BackupTimeRange{
			Start: &metav1.Time{Time: time.Date(2026, 5, 4, 7, 0, 0, 0, time.UTC)},
			End:   &metav1.Time{Time: time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)},
		}
		cli := newRestoreOpsFakeClient(opsRequest, backup)

		Expect(restoreHandler.Action(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})).Should(Succeed())

		cluster := &appsv1.Cluster{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreClusterName, Namespace: opsRequest.Namespace}, cluster)).Should(Succeed())
		Expect(cluster.Spec.Restore.PITR).Should(Equal("2026-05-04T08:00:00Z"))
	})

	It("keeps running while restore condition is not completed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		cluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.CreatingClusterPhase, metav1.ConditionUnknown)
		cli := newRestoreOpsFakeClient(opsRequest, cluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
	})

	It("fails when restore condition failed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		cluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.CreatingClusterPhase, metav1.ConditionFalse)
		cli := newRestoreOpsFakeClient(opsRequest, cluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).Should(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
	})

	It("succeeds after restore completed and target Cluster is running", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.RunningClusterPhase, metav1.ConditionTrue)
		cli := newRestoreOpsFakeClient(opsRequest, targetCluster)
		opsRes := &OpsResource{OpsRequest: opsRequest}

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, opsRes)

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		Expect(opsRes.Cluster.Name).Should(Equal(restoreClusterName))
	})

	It("keeps running after restore completed while target Cluster is not running", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.CreatingClusterPhase, metav1.ConditionTrue)
		cli := newRestoreOpsFakeClient(opsRequest, targetCluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
	})

	It("fails after restore completed when target Cluster failed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.FailedClusterPhase, metav1.ConditionTrue)
		cli := newRestoreOpsFakeClient(opsRequest, targetCluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).Should(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
	})

	It("fails when target Cluster is not visible", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		cli := newRestoreOpsFakeClient(opsRequest)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).Should(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
	})
})

func createRestoreOpsObj(clusterName, restoreOpsName, backupName string) *opsv1alpha1.OpsRequest {
	return &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreOpsName,
			Namespace: testCtx.DefaultNamespace,
			UID:       types.UID(restoreOpsName + "-uid"),
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.OpsRequestTypeLabelKey: string(opsv1alpha1.RestoreType),
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName: clusterName,
			Type:        opsv1alpha1.RestoreType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				Restore: &opsv1alpha1.Restore{
					BackupName:          backupName,
					RestorePointInTime:  "2026-05-04T08:00:00Z",
					VolumeRestorePolicy: string(dpv1alpha1.VolumeClaimRestorePolicySerial),
					Env:                 []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}},
					Parameters:          []dpv1alpha1.ParameterPair{{Name: "restore-param", Value: "restore-value"}},
				},
			},
		},
	}
}

func newRestoreOpsBackup(name string, labels map[string]string) *dpv1alpha1.Backup {
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-cluster",
			Namespace: testCtx.DefaultNamespace,
		},
		Spec: appsv1.ClusterSpec{
			TerminationPolicy: appsv1.Delete,
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:         "mysql",
				ComponentDef: "mysql",
				Replicas:     1,
			}},
		},
	}
	snapshot, err := json.Marshal(cluster)
	Expect(err).ShouldNot(HaveOccurred())
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testCtx.DefaultNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				constant.ClusterSnapshotAnnotationKey: string(snapshot),
			},
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
		},
	}
}

func newRestoreTargetCluster(namespace, name string, phase appsv1.ClusterPhase, restoreStatus metav1.ConditionStatus) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: appsv1.ClusterStatus{
			Phase: phase,
			Conditions: []metav1.Condition{{
				Type:    appsv1.ConditionTypeRestore,
				Status:  restoreStatus,
				Reason:  "test",
				Message: "test",
			}},
		},
	}
}

func newRestoreOpsFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	Expect(dpv1alpha1.AddToScheme(scheme)).Should(Succeed())
	Expect(opsv1alpha1.AddToScheme(scheme)).Should(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}
