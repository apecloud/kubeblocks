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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	It("creates ClusterRestore and does not create Cluster", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		restoreSpec := opsRequest.Spec.GetRestore()
		restoreSpec.DeferPostReadyUntilClusterRunning = true
		backup := newRestoreOpsBackup(backupName, nil)
		cli := newRestoreOpsFakeClient(opsRequest, backup)
		opsRes := &OpsResource{OpsRequest: opsRequest}

		Expect(restoreHandler.Action(reqCtx, cli, opsRes)).Should(Succeed())

		clusterRestore := &dpv1alpha1.ClusterRestore{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreOpsName, Namespace: opsRequest.Namespace}, clusterRestore)).Should(Succeed())
		Expect(clusterRestore.Spec.TargetClusterName).Should(Equal(restoreClusterName))
		Expect(clusterRestore.Spec.BackupRef.Name).Should(Equal(backupName))
		Expect(clusterRestore.Spec.BackupRef.Namespace).Should(Equal(opsRequest.Namespace))
		Expect(clusterRestore.Spec.RestoreTime).Should(Equal("2026-05-04T08:00:00Z"))
		Expect(clusterRestore.Spec.VolumeRestorePolicy).Should(Equal(dpv1alpha1.VolumeClaimRestorePolicySerial))
		Expect(clusterRestore.Spec.DeferPostReadyUntilClusterRunning).Should(BeTrue())
		Expect(clusterRestore.Spec.Env).Should(Equal([]corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}}))
		Expect(clusterRestore.Spec.Parameters).Should(Equal([]dpv1alpha1.ParameterPair{{Name: "restore-param", Value: "restore-value"}}))
		Expect(clusterRestore.OwnerReferences).Should(HaveLen(1))
		Expect(clusterRestore.OwnerReferences[0].UID).Should(Equal(opsRequest.UID))

		cluster := &appsv1.Cluster{}
		err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreClusterName, Namespace: opsRequest.Namespace}, cluster)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
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

		clusterRestore := &dpv1alpha1.ClusterRestore{}
		Expect(cli.Get(reqCtx.Ctx, client.ObjectKey{Name: restoreOpsName, Namespace: opsRequest.Namespace}, clusterRestore)).Should(Succeed())
		Expect(clusterRestore.Spec.RestoreTime).Should(Equal("2026-05-04T08:00:00Z"))
	})

	It("keeps running while ClusterRestore is not completed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseRestoring)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
	})

	It("fails when ClusterRestore failed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseFailed)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).Should(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
	})

	It("succeeds after ClusterRestore completed and target Cluster is running", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseCompleted)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.RunningClusterPhase)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore, targetCluster)
		opsRes := &OpsResource{OpsRequest: opsRequest}

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, opsRes)

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		Expect(opsRes.Cluster.Name).Should(Equal(restoreClusterName))
	})

	It("keeps running after ClusterRestore completed while target Cluster is not running", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseCompleted)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.CreatingClusterPhase)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore, targetCluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
	})

	It("fails after ClusterRestore completed when target Cluster failed", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseCompleted)
		targetCluster := newRestoreTargetCluster(opsRequest.Namespace, restoreClusterName, appsv1.FailedClusterPhase)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore, targetCluster)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).Should(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
	})

	It("keeps running after ClusterRestore completed when target Cluster is not visible yet", func() {
		opsRequest := createRestoreOpsObj(restoreClusterName, restoreOpsName, backupName)
		clusterRestore := newClusterRestore(opsRequest, dpv1alpha1.ClusterRestorePhaseCompleted)
		cli := newRestoreOpsFakeClient(opsRequest, clusterRestore)

		phase, _, err := restoreHandler.ReconcileAction(reqCtx, cli, &OpsResource{OpsRequest: opsRequest})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
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
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testCtx.DefaultNamespace,
			Labels:    labels,
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
		},
	}
}

func newClusterRestore(opsRequest *opsv1alpha1.OpsRequest, phase dpv1alpha1.ClusterRestorePhase) *dpv1alpha1.ClusterRestore {
	return &dpv1alpha1.ClusterRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRestoreName(opsRequest),
			Namespace: opsRequest.Namespace,
		},
		Spec: dpv1alpha1.ClusterRestoreSpec{
			TargetClusterName: opsRequest.Spec.GetClusterName(),
			BackupRef: dpv1alpha1.ClusterRestoreBackupRef{
				Name:      opsRequest.Spec.GetRestore().BackupName,
				Namespace: opsRequest.Namespace,
			},
		},
		Status: dpv1alpha1.ClusterRestoreStatus{
			Phase: phase,
			TargetClusterRef: &dpv1alpha1.ClusterRestoreTargetClusterRef{
				Name:      opsRequest.Spec.GetClusterName(),
				Namespace: opsRequest.Namespace,
			},
		},
	}
}

func newRestoreTargetCluster(namespace, name string, phase appsv1.ClusterPhase) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: appsv1.ClusterStatus{
			Phase: phase,
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
