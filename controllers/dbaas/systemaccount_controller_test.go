/*
Copyright ApeCloud Inc.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("SystemAccount Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Second
	)

	var (
		acctJobsMap = map[dbaasv1alpha1.AccountName]int{
			dbaasv1alpha1.AdminAccount:          1, // created by stmt + AnyPod
			dbaasv1alpha1.DataprotectionAccount: 3, // created by stmt + AllPods
			dbaasv1alpha1.ProbeAccount:          0, // created using ReferToExisting policy (by copying from specified secret)
		}
		expectedAcctList       = []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.ProbeAccount}
		expectedAccounts       dbaasv1alpha1.KBAccountType
		expectedJobsPerCluster int
		expectedAcctNum        int
		ctx                    = context.Background()
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, the existence of old ones shall be found, which causes
		// new objects fail to create.
		By("clean resources")

		testdbaas.ClearClusterResources(&testCtx)

		// namespaced resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testdbaas.ClearResources(&testCtx, intctrlutil.EndpointsSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)

		// clear internal states
		for _, key := range systemAccountReconciler.SecretMapStore.ListKeys() {
			Expect(systemAccountReconciler.SecretMapStore.deleteSecret(key)).Should(Succeed())
		}
		for _, key := range systemAccountReconciler.ExpectionManager.ListKeys() {
			Expect(systemAccountReconciler.ExpectionManager.deleteExpectation(key)).Should(Succeed())
		}
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	mockBackupPolicy := func(name string, engineName, clusterName string) *dataprotectionv1alpha1.BackupPolicy {
		ml := map[string]string{
			intctrlutil.AppInstanceLabelKey: clusterName,
		}

		policy := &dataprotectionv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testCtx.DefaultNamespace,
			},
			Spec: dataprotectionv1alpha1.BackupPolicySpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					DatabaseEngine: engineName,
					LabelsSelector: &metav1.LabelSelector{MatchLabels: ml},
					Secret: dataprotectionv1alpha1.BackupPolicySecret{
						Name: "mock-secret-file",
					},
				},
				Hooks: dataprotectionv1alpha1.BackupPolicyHook{
					PreCommands:  []string{"mock-precommand"},
					PostCommands: []string{"mock-postcommand"},
				},
				RemoteVolume: corev1.Volume{
					Name: "mock-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "mock-pvc"},
					},
				},
			},
		}
		return policy
	}

	patchCluster := func(key types.NamespacedName) {
		By("Patching Cluster to trigger reconcile")
		Eventually(func() error {
			return testdbaas.ChangeSpec(&testCtx, key, func(cluster *dbaasv1alpha1.Cluster) {
				if cluster.Annotations == nil {
					cluster.Annotations = make(map[string]string)
				}
				cluster.Annotations["mockmode"] = "testing"
			})
		}, timeout, interval).Should(Succeed())
	}

	assureBackupPolicy := func(policyName, engineName, clusterName string) *dataprotectionv1alpha1.BackupPolicy {
		By("Creating Backup Policy")
		policy := mockBackupPolicy(policyName, engineName, clusterName)
		Expect(testCtx.CheckedCreateObj(ctx, policy)).Should(Succeed())

		createdPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: policy.Name, Namespace: policy.Namespace}, createdPolicy)
		}, timeout, interval).Should(Succeed())
		return createdPolicy
	}

	mockEndpoint := func(namespace, endpointName string, ips []string) *corev1.Endpoints {
		mockAddresses := func(ip, podName string) corev1.EndpointAddress {
			return corev1.EndpointAddress{
				IP:       ip,
				NodeName: nil,
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: testCtx.DefaultNamespace,
					Name:      podName,
				},
			}
		}

		addresses := make([]corev1.EndpointAddress, 0)
		for i := 0; i < len(ips); i++ {
			podName := "pod-" + testCtx.GetRandomStr()
			addresses = append(addresses, mockAddresses(ips[i], podName))
		}

		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      endpointName,
			},
		}
		ep.Subsets = []corev1.EndpointSubset{
			{
				Addresses: addresses,
			},
		}
		return ep
	}

	assureEndpoint := func(namespace, epname string, ips []string) *corev1.Endpoints {
		ep := mockEndpoint(namespace, epname, ips)
		Expect(testCtx.CheckedCreateObj(ctx, ep)).Should(Succeed())
		// assure cluster def is ready
		createdEP := &corev1.Endpoints{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: epname, Namespace: namespace}, createdEP)
		}, timeout, interval).Should(Succeed())
		return createdEP
	}

	getAccounts := func(g Gomega, cluster *dbaasv1alpha1.Cluster, ml client.MatchingLabels) dbaasv1alpha1.KBAccountType {
		secrets := &corev1.SecretList{}
		g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		jobs := &batchv1.JobList{}
		g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		return getAccountFacts(secrets, jobs)
	}

	setupMockEnv := func(clusterDefinitionName, clusterVersionName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion) {
		// setup expected values
		expectedAcctNum = len(expectedAcctList)
		expectedAccounts = dbaasv1alpha1.KBAccountInvalid
		expectedJobsPerCluster = 0
		for _, acct := range expectedAcctList {
			expectedJobsPerCluster += acctJobsMap[acct]
			expectedAccounts |= acct.GetAccountID()
		}
		// mock clusterDefinition and clusterVersion objects
		clusterDef := testdbaas.MockClusterDefinition(ctx, testCtx, clusterDefinitionName, "consensusset/wesql_cd_sysacct.yaml")
		Expect(clusterDef).ShouldNot(BeNil())
		clusterVersion := testdbaas.CreateConsensusMysqlClusterVersion(ctx, testCtx, clusterDefinitionName, clusterVersionName)
		Expect(clusterVersion).ShouldNot(BeNil())
		return clusterDef, clusterVersion
	}

	Context("When Creating Cluster", func() {
		It("Should update system account expectation", func() {
			if testEnv.UseExistingCluster != nil && *testEnv.UseExistingCluster {
				Skip("Mocked Cluster is not fully implemented to run in real cluster.")
			}

			var (
				randomStr             = testCtx.GetRandomStr()
				clusterName           = "cluster-" + randomStr
				backupPolicyName      = "backup-policy-demo"
				databaseEngine        = "mysql"
				clusterDefinitionName = "cluster-definition-" + randomStr
				clusterVersionName    = "clusterversion-" + randomStr
				consensusCompName     = "consensus"
				ips                   = []string{"10.0.0.0", "10.0.0.1", "10.0.0.2"}
			)

			By("Setup mock env")
			clusterDef, clusterVersion := setupMockEnv(clusterDefinitionName, clusterVersionName)

			By("Mock Cluster")
			cluster := testdbaas.CreateConsensusMysqlCluster(ctx, testCtx, clusterDef.Name, clusterVersion.Name, clusterName, consensusCompName)
			Expect(cluster).ShouldNot(BeNil())

			// services of type ClusterIP should have been created.
			serviceName := clusterName + "-" + consensusCompName
			headlessServiceName := serviceName + "-headless"

			_ = assureEndpoint(cluster.Namespace, serviceName, ips[0:1])
			_ = assureEndpoint(cluster.Namespace, headlessServiceName, ips)

			patchCluster(intctrlutil.GetNamespacedName(cluster))

			ml := getLabelsForSecretsAndJobs(cluster.Name, clusterDef.Spec.Type, clusterDef.Name, consensusCompName)
			By("Verify two accounts to be created")
			Eventually(func(g Gomega) {
				accounts := getAccounts(g, cluster, ml)
				g.Expect(accounts).To(BeEquivalentTo(expectedAccounts))
			}, timeout, interval).Should(Succeed())

			By("Assure some Secrets creation are cached")
			secretsToCreate1 := 0
			Eventually(func() int {
				secretsToCreate1 = len(systemAccountReconciler.SecretMapStore.ListKeys())
				return secretsToCreate1
			}, timeout, interval).Should(And(BeNumerically(">", 0), BeNumerically("<=", expectedAcctNum)))

			// create backup policy, and update expected values
			policy := assureBackupPolicy(backupPolicyName, databaseEngine, clusterName)
			expectedJobsPerCluster += acctJobsMap[dbaasv1alpha1.DataprotectionAccount]
			expectedAccounts |= dbaasv1alpha1.KBAccountDataprotection
			expectedAcctNum += 1

			By("Check the BackupPolicy creation filters run")
			policyKey := expectationKey(policy.Namespace, clusterName, databaseEngine)
			Eventually(func(g Gomega) {
				exp, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
				g.Expect(exists).To(BeTrue())
				g.Expect(exp.toCreate&dbaasv1alpha1.KBAccountDataprotection > 0).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Verify three accounts have been created in total")
			Eventually(func(g Gomega) {
				accounts := getAccounts(g, cluster, ml)
				g.Expect(accounts).To(BeEquivalentTo(expectedAccounts))
			}, timeout, interval).Should(Succeed())

			By("Assure more Secrets creation are cached")
			secretsToCreate2 := 0
			Eventually(func() int {
				secretsToCreate2 = len(systemAccountReconciler.SecretMapStore.ListKeys())
				return secretsToCreate2
			}, timeout, interval).Should(BeNumerically(">", secretsToCreate1))

			secretsCreated := 0
			secrets := &corev1.SecretList{}
			Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
			secretsCreated = len(secrets.Items)

			By("Mock all jobs completed and deleted")
			Eventually(func(g Gomega) {
				jobs := &batchv1.JobList{}
				g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				g.Expect(len(jobs.Items)).To(BeEquivalentTo(expectedJobsPerCluster), "there should be 5 jobs created")
				for _, job := range jobs.Items {
					g.Expect(testdbaas.ChangeStatus(&testCtx, intctrlutil.GetNamespacedName(&job), func(job *batchv1.Job) {
						job.Status.Conditions = []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}}
					})).To(Succeed())
					g.Expect(k8sClient.Delete(ctx, &job)).To(Succeed())
				}
			}, timeout, interval).Should(Succeed())

			// remove 'orphan' finalizers to make sure all jobs can be deleted.
			Eventually(func(g Gomega) {
				finalizerName := "orphan"
				jobs := &batchv1.JobList{}
				g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				for _, job := range jobs.Items {
					g.Expect(testdbaas.ChangeSpec(&testCtx, intctrlutil.GetNamespacedName(&job), func(job *batchv1.Job) {
						controllerutil.RemoveFinalizer(job, finalizerName)
					})).To(Succeed())
				}
				g.Expect(len(jobs.Items)).To(Equal(0))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				jobs := &batchv1.JobList{}
				g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				g.Expect(len(jobs.Items)).To(Equal(0))
			}, timeout, interval).Should(Succeed())

			By("Check all secrets creation are completed")
			Eventually(func(g Gomega) {
				secrets := &corev1.SecretList{}
				g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				g.Expect(len(secrets.Items)).To(Equal(secretsCreated + secretsToCreate2))
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(0))
			}, 2*timeout, interval).Should(Succeed())

			By("Check the BackupPolicy deletion filter triggered after the Cluster is deleted")
			Eventually(func() error {
				return k8sClient.Delete(ctx, policy)
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				_, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
				g.Expect(exists).To(BeFalse())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When Delete Cluster", func() {
		It("Should clear relevant expectations and secrets after cluster deletion", func() {
			if testEnv.UseExistingCluster != nil && *testEnv.UseExistingCluster {
				Skip("Mocked Cluster is not fully implemented to run in real cluster.")
			}
			var (
				randomStr             = testCtx.GetRandomStr()
				clusterDefinitionName = "cluster-definition-" + randomStr
				clusterVersionName    = "clusterversion-" + randomStr
				consensusCompName     = "consensus"
				ips                   = []string{"10.0.0.0", "10.0.0.1", "10.0.0.2"}
			)

			By("set up mock env")
			clusterDef, clusterVersion := setupMockEnv(clusterDefinitionName, clusterVersionName)

			clusterNameList := []string{"cluster-alpha", "cluster-beta"}
			clusterList := make([]types.NamespacedName, 0)

			for _, clusterName := range clusterNameList {
				cluster := testdbaas.CreateConsensusMysqlCluster(ctx, testCtx, clusterDef.Name, clusterVersion.Name, clusterName, consensusCompName)
				Expect(cluster).ShouldNot(BeNil())

				// services of type ClusterIP should have been created.
				serviceName := cluster.Name + "-" + consensusCompName
				headlessServiceName := serviceName + "-headless"
				_ = assureEndpoint(cluster.Namespace, serviceName, ips[0:1])
				_ = assureEndpoint(cluster.Namespace, headlessServiceName, ips)
				patchCluster(intctrlutil.GetNamespacedName(cluster))

				clusterList = append(clusterList, intctrlutil.GetNamespacedName(cluster))

				Eventually(func(g Gomega) {
					rootSecretName := clusterName + "-conn-credential"
					rootSecret := &corev1.Secret{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: rootSecretName}, rootSecret)).To(Succeed())
				}, timeout, interval).Should(Succeed())

				ml := getLabelsForSecretsAndJobs(cluster.Name, clusterDef.Spec.Type, clusterDef.Name, consensusCompName)
				By("Verify two accounts to be created")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(expectedAccounts))
				}, timeout, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(secrets.Items)).To(BeEquivalentTo(1))
				}, timeout, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(expectedJobsPerCluster))
				}, timeout, interval).Should(Succeed())
			}

			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(expectedJobsPerCluster * expectedAcctNum))
			}, timeout, interval).Should(Succeed())

			By("Delete Cluster and check SecretMapStore")
			Eventually(func(g Gomega) {
				cluster := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterList[0], cluster)).To(Succeed())
				g.Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(expectedJobsPerCluster))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				cluster := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterList[1], cluster)).To(Succeed())
				g.Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(expectedJobsPerCluster))
			}, timeout, interval).Should(Succeed())
		})
	})
})
