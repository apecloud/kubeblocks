/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Restore OpsRequest", func() {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		restoreClusterName    = "restore-cluster"
		backupName            = "backup-for-ops-" + randomStr
		nodePort              = int32(31212)
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest for Restore", func() {
		var (
			opsRes   *OpsResource
			reqCtx   intctrlutil.RequestCtx
			backup   *dpv1alpha1.Backup
			account  string
			password []byte
		)
		BeforeEach(func() {
			By("init operations resources ")
			opsRes, _, _ = initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			reqCtx = intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("create Backup")
			backup = testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(testdp.BackupPolicyName).
				SetBackupMethod(testdp.VSBackupMethodName).
				Create(&testCtx).GetObject()

			By("set system account")
			account = "test"
			password, _ = json.Marshal("testPwd")

			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			})).Should(Succeed())
		})

		It("", func() {

			By("create Restore OpsRequest")
			opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-"+randomStr, backupName)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase

			By("mock restore OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restore action")
			restoreHandler := RestoreOpsHandler{}
			_ = restoreHandler.Action(reqCtx, k8sClient, opsRes)

			By("test restore reconcile function")
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})
		It("test restore when cluster has system Accounts", func() {
			By("mock opsRequest.cluster with secret")
			Expect(testapps.ChangeObj(&testCtx, backup, func(backup *dpv1alpha1.Backup) {
				opsRes.Cluster.ResourceVersion = ""
				passwordConfig := &appsv1alpha1.PasswordConfig{
					Length: 29,
				}
				opsRes.Cluster.Spec.ComponentSpecs[0].SystemAccounts = []appsv1alpha1.ComponentSystemAccount{
					{
						Name:           account,
						PasswordConfig: passwordConfig,
						SecretRef:      nil,
					},
				}
			})).Should(Succeed())

			By("mock backup annotations and labels")
			Expect(testapps.ChangeObj(&testCtx, backup, func(backup *dpv1alpha1.Backup) {
				backup.Labels = map[string]string{
					dptypes.BackupTypeLabelKey:      string(dpv1alpha1.BackupTypeFull),
					constant.KBAppComponentLabelKey: consensusComp,
				}
				opsRes.Cluster.ResourceVersion = ""
				secretName := constant.GenerateAccountSecretName(opsRes.Cluster.Name, consensusComp, account)
				labels := constant.GetComponentWellKnownLabels(opsRes.Cluster.Name, consensusComp)
				secret := builder.NewSecretBuilder(opsRes.Cluster.Namespace, secretName).
					AddLabelsInMap(labels).
					AddLabels(constant.AppManagedByLabelKey, constant.AppName).
					AddLabels(constant.AppInstanceLabelKey, opsRes.Cluster.Name).
					AddLabels(constant.KBAppComponentLabelKey, consensusComp).
					AddLabels(constant.ClusterAccountLabelKey, account).
					PutData(constant.AccountNameForSecret, []byte(account)).
					PutData(constant.AccountPasswdForSecret, password).
					SetImmutable(true).
					GetObject()
				secretList := &corev1.SecretList{Items: []corev1.Secret{
					*secret,
				}}
				clusterBytes, _ := json.Marshal(opsRes.Cluster)
				secretListBytes, _ := json.Marshal(secretList)

				backup.Annotations = map[string]string{
					constant.ClusterSnapshotAnnotationKey:  string(clusterBytes),
					constant.SecretsSnapshotAnnotationsKey: string(secretListBytes),
				}
			})).Should(Succeed())

			By("create Restore OpsRequest")
			opsRes.OpsRequest = createRestoreOpsObj(restoreClusterName, "restore-ops-"+randomStr, backupName)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase

			By("mock restore OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restore action")
			restoreHandler := RestoreOpsHandler{}
			_ = restoreHandler.Action(reqCtx, k8sClient, opsRes)

			By("the Secret should be restored correctly")
			restoredSecretName := constant.GenerateAccountSecretName(restoreClusterName, consensusComp, account)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: restoredSecretName, Namespace: opsRes.OpsRequest.Namespace}, func(g Gomega, restoreSecret *corev1.Secret) {
				Expect(restoreSecret.Data[constant.AccountNameForSecret]).Should(Equal([]byte(account)))
				Expect(restoreSecret.Data[constant.AccountPasswdForSecret]).Should(Equal(password))
			})).Should(Succeed())

		})
		It("test if source cluster exists services", func() {
			By("mock backup annotations and labels")
			opsRes.Cluster.Spec.Services = []appsv1alpha1.ClusterService{
				{
					ComponentSelector: consensusComp,
					Service: appsv1alpha1.Service{
						Name: "svc",
						Spec: corev1.ServiceSpec{
							Ports: []corev1.ServicePort{
								{Name: "port", Port: 3306, NodePort: nodePort},
							},
						},
					},
				},
				{
					ComponentSelector: consensusComp,
					Service: appsv1alpha1.Service{
						Name:        "svc-2",
						ServiceName: "svc-2",
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{Name: "port", Port: 3306, NodePort: nodePort},
							},
						},
					},
				}}
			Expect(testapps.ChangeObj(&testCtx, backup, func(backup *dpv1alpha1.Backup) {
				backup.Labels = map[string]string{
					dptypes.BackupTypeLabelKey:      string(dpv1alpha1.BackupTypeFull),
					constant.KBAppComponentLabelKey: consensusComp,
				}
				opsRes.Cluster.ResourceVersion = ""
				clusterBytes, _ := json.Marshal(opsRes.Cluster)
				backup.Annotations = map[string]string{
					constant.ClusterSnapshotAnnotationKey: string(clusterBytes),
				}
			})).Should(Succeed())

			By("create Restore OpsRequest")
			opsRes.OpsRequest = createRestoreOpsObj(restoreClusterName, "restore-ops-"+randomStr, backupName)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase

			By("mock restore OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restore action")
			restoreHandler := RestoreOpsHandler{}
			_ = restoreHandler.Action(reqCtx, k8sClient, opsRes)

			By("the loadBalancer should be reset")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: restoreClusterName, Namespace: opsRes.OpsRequest.Namespace}, func(g Gomega, restoreCluster *appsv1alpha1.Cluster) {
				Expect(restoreCluster.Spec.Services).Should(HaveLen(1))
			})).Should(Succeed())
		})

	})
})

func createRestoreOpsObj(clusterName, restoreOpsName, backupName string) *appsv1alpha1.OpsRequest {
	ops := &appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreOpsName,
			Namespace: testCtx.DefaultNamespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.OpsRequestTypeLabelKey: string(appsv1alpha1.RestoreType),
			},
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			ClusterName: clusterName,
			Type:        appsv1alpha1.RestoreType,
			SpecificOpsRequest: appsv1alpha1.SpecificOpsRequest{
				Restore: &appsv1alpha1.Restore{
					BackupName: backupName,
				},
			},
		},
	}
	return testapps.CreateOpsRequest(ctx, testCtx, ops)
}
