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

package dbaas

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		timeout             = time.Second * 10
		consistTimeout      = time.Second * 5
		interval            = time.Second
		consensusCompName   = "consensus"
		orphanFinalizerName = "orphan"
	)

	// resourceInfo defines the number of jobs and secrets to be created per account.
	type resourceInfo struct {
		jobNum    int
		secretNum int
	}

	// testEnvFiles defines the files will be used to setup testing clusters.
	type testEnvInfo struct {
		clusterDefFile     string
		clusterVersionFile string
		resourceMap        map[dbaasv1alpha1.AccountName]resourceInfo
	}

	// testClusterStatus defines the accounts one cluster should have.
	type testClusterInfo struct {
		// clusterFile string //  cluster file to create cluster instance
		accounts []dbaasv1alpha1.AccountName // accounts this cluster should have
	}
	// sysAcctTestCase defines the info to setup test env, cluster and their expected result to verify against.
	type sysAcctTestCase struct {
		envInfo           testEnvInfo                                                                                                            // to setup test env, create ClusterDef, ClusterVersion
		clusterInfo       testClusterInfo                                                                                                        // to create cluster instance, and its expected accounts
		createClusterFunc func(cd *dbaasv1alpha1.ClusterDefinition, cv *dbaasv1alpha1.ClusterVersion, clusterName string) *dbaasv1alpha1.Cluster // how to create cluster
		patchClusterFunc  func(objectKey types.NamespacedName)                                                                                   // how to patch cluster in testing env
	}

	var (
		mysqlTestCases map[string]sysAcctTestCase
		ctx            = context.Background()
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		testdbaas.ClearClusterResources(&testCtx)

		// namespaced resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testdbaas.ClearResources(&testCtx, intctrlutil.EndpointsSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
	}

	cleanInternalCache := func() {
		// cleaning cached internal resources
		expectkeys := systemAccountReconciler.ExpectionManager.ListKeys()
		for _, key := range expectkeys {
			_ = systemAccountReconciler.ExpectionManager.deleteExpectation(key)
		}

		secretKeys := systemAccountReconciler.SecretMapStore.ListKeys()
		for _, key := range secretKeys {
			_ = systemAccountReconciler.SecretMapStore.deleteSecret(key)
		}
	}

	// TODO:@shanshan, mockEndpoint and mockBackupPolicy should be refined soon.
	//
	// start of mock functions to be refined.
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

	mockBackupPolicy := func(name string, engineName, clusterName string) *dataprotectionv1alpha1.BackupPolicy {
		ml := map[string]string{
			intctrlutil.AppInstanceLabelKey:  clusterName,
			intctrlutil.AppComponentLabelKey: engineName,
		}

		policy := &dataprotectionv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testCtx.DefaultNamespace,
			},
			Spec: dataprotectionv1alpha1.BackupPolicySpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					LabelsSelector: &metav1.LabelSelector{MatchLabels: ml},
					Secret: dataprotectionv1alpha1.BackupPolicySecret{
						Name: "mock-secret-file",
					},
				},
				Hooks: &dataprotectionv1alpha1.BackupPolicyHook{
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
	// end of mock functions to be refined

	getAccounts := func(g Gomega, cluster *dbaasv1alpha1.Cluster, ml client.MatchingLabels) dbaasv1alpha1.KBAccountType {
		secrets := &corev1.SecretList{}
		g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		jobs := &batchv1.JobList{}
		g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		return getAccountFacts(secrets, jobs)
	}

	checkOwnerReferenceToObj := func(ref metav1.OwnerReference, obj client.Object) bool {
		return ref.Name == obj.GetName() && ref.UID == obj.GetUID()
	}

	createConsensusMySQLCluster := func(cd *dbaasv1alpha1.ClusterDefinition, cv *dbaasv1alpha1.ClusterVersion, clusterName string) *dbaasv1alpha1.Cluster {
		cluster := testdbaas.CreateConsensusMysqlCluster(testCtx, cd.Name, cv.Name, clusterName, consensusCompName)
		Expect(cluster).NotTo(BeNil())
		return cluster
	}

	patchConsensusClusterReadyToServe := func(objectKey types.NamespacedName) {
		// services of type ClusterIP should have been created.
		ips := []string{"10.0.0.0", "10.0.0.1", "10.0.0.2"}
		serviceName := objectKey.Name + "-" + consensusCompName
		headlessServiceName := serviceName + "-headless"
		_ = assureEndpoint(objectKey.Namespace, serviceName, ips[0:1])
		_ = assureEndpoint(objectKey.Namespace, headlessServiceName, ips)

		By("Patching Cluster to running phase")
		Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, objectKey, func(cluster *dbaasv1alpha1.Cluster) {
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
		}), timeout, interval).Should(Succeed())
	}

	initSysAccountTests := func(testCases map[string]sysAcctTestCase) (clustersMap map[string]types.NamespacedName,
		matchingLabelsMap map[string]client.MatchingLabels) {

		clustersMap = make(map[string]types.NamespacedName)
		matchingLabelsMap = make(map[string]client.MatchingLabels)

		// create cd, cv, and cluster defined in each testcase
		for testName, testCase := range testCases {
			randomStr := testCtx.GetRandomStr()
			clusterName := "sysacc-cluster-" + randomStr
			clusterDefinitionName := "sysacc-cd-" + randomStr
			clusterVersionName := "sysacc-cv-" + randomStr

			By("Testing case: " + testName)
			envFiles := testCase.envInfo

			By("Mock test env, creating ClusterDefinition and ClusterVersion")
			clusterDef := testdbaas.CreateCustomizedObj(&testCtx, envFiles.clusterDefFile, &dbaasv1alpha1.ClusterDefinition{}, testdbaas.CustomizeObjYAML(clusterDefinitionName))
			Expect(clusterDef).NotTo(BeNil())

			clusterVersion := testdbaas.CreateCustomizedObj(&testCtx, envFiles.clusterVersionFile, &dbaasv1alpha1.ClusterVersion{}, testdbaas.CustomizeObjYAML(clusterVersionName, clusterDefinitionName))
			Expect(clusterVersion).NotTo(BeNil())

			By("Mock a cluster")
			cluster := testCase.createClusterFunc(clusterDef, clusterVersion, clusterName)

			Eventually(func(g Gomega) {
				rootSecretName := cluster.Name + "-conn-credential"
				rootSecret := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: rootSecretName}, rootSecret)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			clustersMap[testName] = client.ObjectKeyFromObject(cluster)
			matchingLabelsMap[testName] = getLabelsForSecretsAndJobs(cluster.Name, clusterDef.Spec.Type, clusterDef.Name, consensusCompName)
		}
		return
	}

	BeforeEach((func() {
		// setup test cases
		mysqlTestCases = map[string]sysAcctTestCase{
			"wesql-no-accts": {
				envInfo: testEnvInfo{
					clusterDefFile:     "consensusset/wesql_cd.yaml",
					clusterVersionFile: "consensusset/wesql_cv.yaml",
					resourceMap: map[dbaasv1alpha1.AccountName]resourceInfo{
						dbaasv1alpha1.AdminAccount: {
							jobNum:    0,
							secretNum: 0,
						},
						dbaasv1alpha1.ProbeAccount: {
							jobNum:    0,
							secretNum: 0,
						},
						dbaasv1alpha1.DataprotectionAccount: {
							jobNum:    0,
							secretNum: 0,
						},
						dbaasv1alpha1.MonitorAccount: {
							jobNum:    0,
							secretNum: 0,
						},
					},
				},
				clusterInfo: testClusterInfo{
					accounts: []dbaasv1alpha1.AccountName{},
				},
				createClusterFunc: createConsensusMySQLCluster,
				patchClusterFunc:  patchConsensusClusterReadyToServe,
			},

			"wesql-with-accts": {
				envInfo: testEnvInfo{
					clusterDefFile:     "consensusset/wesql_cd_sysacct.yaml",
					clusterVersionFile: "consensusset/wesql_cv.yaml",
					resourceMap: map[dbaasv1alpha1.AccountName]resourceInfo{
						dbaasv1alpha1.AdminAccount: {
							jobNum:    1,
							secretNum: 1,
						}, // created by stmt + AnyPod
						dbaasv1alpha1.ProbeAccount: {
							jobNum:    0,
							secretNum: 1,
						}, // created using ReferToExisting policy (by copying from specified secret)
						dbaasv1alpha1.DataprotectionAccount: {
							jobNum:    3,
							secretNum: 1,
						}, // created by stmt + AllPods
						dbaasv1alpha1.MonitorAccount: {
							jobNum:    0,
							secretNum: 0,
						}, // won't be created, not configured in ClusterDef
					},
				},
				clusterInfo: testClusterInfo{
					accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.ProbeAccount},
				},
				createClusterFunc: createConsensusMySQLCluster,
				patchClusterFunc:  patchConsensusClusterReadyToServe,
			},
		}
	}))

	Context("When Creating Cluster", func() {
		if testCtx.UsingExistingCluster() {
			Skip("Skip test if uses exsting cluster")
		}

		var (
			clustersMap       map[string]types.NamespacedName
			matchingLabelsMap map[string]client.MatchingLabels
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			clustersMap, matchingLabelsMap = initSysAccountTests(mysqlTestCases)
		})

		It("Should create jobs and cache secrets as expected for each test case", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					acctList   dbaasv1alpha1.KBAccountType
					jobsNum    int
					secretsNum int
				)

				clusterInfo := testCase.clusterInfo
				for _, acc := range clusterInfo.accounts {
					resource := testCase.envInfo.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}

				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				// patch cluster to running
				testCase.patchClusterFunc(clusterKey)

				// get latest cluster object
				cluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml, ok := matchingLabelsMap[testName]
				Expect(ok).To(BeTrue())

				if secretsNum == 0 && jobsNum == 0 {
					By("No accouts should be create for test case: " + testName)
					// verify nothing will be created or cached till timeout
					Consistently(func(g Gomega) {
						accounts := getAccounts(g, cluster, ml)
						g.Expect(accounts).To(BeEquivalentTo(acctList))
					}, consistTimeout, interval).Should(Succeed())
					continue
				}

				By("Verify accounts to be created are correct")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}, timeout*2, interval).Should(Succeed())

				By("Assure some secrets have been cached")
				Eventually(func() int {
					return len(systemAccountReconciler.SecretMapStore.ListKeys())
				}, timeout, interval).Should(BeNumerically(">", 0))

				By("Verify all jobs created have their lables set correctly")
				// get all jobs
				Eventually(func(g Gomega) {
					// all jobs matching filter `ml` should be a job for sys account.
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					for _, job := range jobs.Items {
						_, ok := job.Labels[clusterAccountLabelKey]
						g.Expect(ok).To(BeTrue())
						g.Expect(len(job.ObjectMeta.OwnerReferences)).To(BeEquivalentTo(1))
						g.Expect(checkOwnerReferenceToObj(job.OwnerReferences[0], cluster)).To(BeTrue())
					}
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}, timeout, interval).Should(Succeed())
			}
		})

		It("Should update system account expectation after BackupPolicy is created", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					randomStr        = testCtx.GetRandomStr()
					backupPolicyName = "bp-" + randomStr
					acctList         dbaasv1alpha1.KBAccountType
					jobsNum          int
					secretsNum       int
				)

				clusterInfo := testCase.clusterInfo
				for _, acc := range clusterInfo.accounts {
					resource := testCase.envInfo.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}

				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				// patch cluster to running
				testCase.patchClusterFunc(clusterKey)

				// get latest cluster object
				cluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml, ok := matchingLabelsMap[testName]
				Expect(ok).To(BeTrue())

				By("Verify accounts to be created are correct")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}, timeout*2, interval).Should(Succeed())

				By("Verify all jobs have been created")
				Eventually(func(g Gomega) {
					// all jobs matching filter `ml` should be a job for sys account.
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}, timeout, interval).Should(Succeed())

				// remember the number of cached secrets before create backup policy
				var secretsToCreate1 int
				By("Assure some secrets have been cached")
				if secretsNum > 0 || jobsNum > 0 {
					Eventually(func() int {
						secretsToCreate1 = len(systemAccountReconciler.SecretMapStore.ListKeys())
						return secretsToCreate1
					}, timeout, interval).Should(BeNumerically(">", 0))
				}

				// create backup policy, and update expected values
				By("Check the BackupPolicy creation succeeds and ExpectionManager updated")
				policy := assureBackupPolicy(backupPolicyName, consensusCompName, cluster.Name)
				policyKey := expectationKey(policy.Namespace, cluster.Name, consensusCompName)
				Eventually(func(g Gomega) {
					exp, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
					g.Expect(exists).To(BeTrue(), "ExpectionManager should have key:"+policyKey)
					g.Expect(exp.toCreate&dbaasv1alpha1.KBAccountDataprotection > 0).To(BeTrue())
				}, timeout, interval).Should(Succeed())

				resource := testCase.envInfo.resourceMap[dbaasv1alpha1.DataprotectionAccount]

				if resource.jobNum == 0 || resource.secretNum == 0 {
					// if DataprotectionAccount is not configured in ClusterDef, there should be no updates
					By("No job will be created, if account not configure")
					Consistently(func(g Gomega) {
						// no job will be created
						if resource.jobNum == 0 {
							jobs := &batchv1.JobList{}
							g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
							g.Expect(len(jobs.Items)).To(BeEquivalentTo(0))
						}

						secrets := &corev1.SecretList{}
						if resource.secretNum == 0 {
							// no secret will be created
							g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
							g.Expect(len(secrets.Items)).To(BeEquivalentTo(0))
						}
					}, consistTimeout, interval).Should(Succeed())

					By("Delete BackupPolicy")
					Eventually(func() error {
						return k8sClient.Delete(ctx, policy)
					}, timeout, interval).Should(Succeed())

					Eventually(func(g Gomega) {
						_, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
						g.Expect(exists).To(BeFalse())
					}, timeout, interval).Should(Succeed())

					continue
				}

				// if DataprotectionAccount is configured, some job should be created
				By("one or more jobs should be created, if DataProtection account is configured")
				// update expected status
				acctList |= dbaasv1alpha1.KBAccountDataprotection
				jobsNum += testCase.envInfo.resourceMap[dbaasv1alpha1.DataprotectionAccount].jobNum
				secretsNum += testCase.envInfo.resourceMap[dbaasv1alpha1.DataprotectionAccount].secretNum

				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}, timeout*2, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}, timeout, interval).Should(Succeed())

				By("Assure one more secret is cached")
				var secretsToCreate2 int
				Eventually(func() int {
					secretsToCreate2 = len(systemAccountReconciler.SecretMapStore.ListKeys())
					return secretsToCreate2
				}, timeout, interval).Should(BeEquivalentTo(1 + secretsToCreate1))

				By("Check the BackupPolicy deletion event is triggered after the policy is deleted")
				Eventually(func() error {
					return k8sClient.Delete(ctx, policy)
				}, timeout, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					_, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
					g.Expect(exists).To(BeFalse())
				}, timeout, interval).Should(Succeed())

				By("Secrets cached are not affected after BackupPolicy deletion")
				Eventually(func() int {
					secretsToCreate := len(systemAccountReconciler.SecretMapStore.ListKeys())
					return secretsToCreate
				}, timeout, interval).Should(BeEquivalentTo(secretsToCreate2))
			}
		})

		It("Cached secrets should be created when jobs succeeds", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					acctList   dbaasv1alpha1.KBAccountType
					jobsNum    int
					secretsNum int
				)

				clusterInfo := testCase.clusterInfo
				for _, acc := range clusterInfo.accounts {
					resource := testCase.envInfo.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}

				if secretsNum == 0 && jobsNum == 0 {
					continue
				}
				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				// patch cluster to running
				testCase.patchClusterFunc(clusterKey)

				// get latest cluster object
				cluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml, ok := matchingLabelsMap[testName]
				Expect(ok).To(BeTrue())

				// wait for a while till all jobs are created
				By("Mock all jobs are completed and deleted")
				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
					for _, job := range jobs.Items {
						g.Expect(testdbaas.ChangeObjStatus(&testCtx, &job, func() {
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
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					for _, job := range jobs.Items {
						g.Expect(testdbaas.ChangeObj(&testCtx, &job, func() { controllerutil.RemoveFinalizer(&job, orphanFinalizerName) })).To(Succeed())
					}
					g.Expect(len(jobs.Items)).To(Equal(0), "Verify all jobs completed and deleted")
				}, timeout, interval).Should(Succeed())

				By("Check secrets created")
				Eventually(func(g Gomega) {
					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(secrets.Items)).To(BeEquivalentTo(secretsNum))
				}, timeout, interval).Should(Succeed())

				By("Verify all secrets created have their finalizer and lables set correctly")
				// get all secrets, and check their lables and finalizer
				Eventually(func(g Gomega) {
					// get secrets matching filter
					secretsForAcct := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secretsForAcct, ml)).To(Succeed())
					for _, secret := range secretsForAcct.Items {
						// each secret has finalizer
						g.Expect(controllerutil.ContainsFinalizer(&secret, dbClusterFinalizerName)).To(BeTrue())
						g.Expect(len(secret.ObjectMeta.OwnerReferences)).To(BeEquivalentTo(1))
						g.Expect(checkOwnerReferenceToObj(secret.OwnerReferences[0], cluster)).To(BeTrue())
					}
				}, timeout, interval).Should(Succeed())
			}
			// all jobs succeeded, and there should be no cached secrets left behind.
			Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(0))
			// no backup policy is created, it should be empty.
			Expect(len(systemAccountReconciler.ExpectionManager.ListKeys())).To(BeEquivalentTo(0))
		})
	}) // end of context

	Context("When Delete Cluster", func() {
		if testCtx.UsingExistingCluster() {
			Skip("Skip test if uses exsting cluster")
		}
		var (
			clustersMap       map[string]types.NamespacedName
			matchingLabelsMap map[string]client.MatchingLabels
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			clustersMap, matchingLabelsMap = initSysAccountTests(mysqlTestCases)
		})

		It("Should clear relevant expectations and secrets after cluster deletion", func() {
			var (
				totalJobs   int
				clusterList []types.NamespacedName
			)

			for testName, testCase := range mysqlTestCases {
				var (
					acctList   dbaasv1alpha1.KBAccountType
					jobsNum    int
					secretsNum int
				)

				clusterInfo := testCase.clusterInfo
				for _, acc := range clusterInfo.accounts {
					resource := testCase.envInfo.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}
				totalJobs += jobsNum

				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				clusterList = append(clusterList, clusterKey)

				// patch cluster to running
				testCase.patchClusterFunc(clusterKey)

				// get latest cluster object
				cluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml, ok := matchingLabelsMap[testName]
				Expect(ok).To(BeTrue())

				By("Verify accounts to be created")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}, timeout*2, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}, timeout, interval).Should(Succeed())
			}

			By("Verify secrets and jobs size")
			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(totalJobs), "before delete, there are %d cached secrets", totalJobs)
			}, timeout, interval).Should(Succeed())

			By("Delete 0-th cluster from list, there should be no change jobs and cache size")
			cluster := &dbaasv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, clusterList[0], cluster)).To(Succeed())
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(totalJobs), "delete 0-th cluster, there are %d cached secrets", totalJobs)
			}, timeout, interval).Should(Succeed())

			By("Delete remaining cluster before jobs are done, all cached secrets should be removed")
			for i := 1; i < len(clusterList); i++ {
				cluster = &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterList[i], cluster)).To(Succeed())
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}

			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(0))
			}, timeout, interval).Should(Succeed())
		})
	}) // end of context

	Context("When Update Cluster", func() {
		if testCtx.UsingExistingCluster() {
			Skip("Skip test if uses exsting cluster")
		}

		var (
			clustersMap       map[string]types.NamespacedName
			matchingLabelsMap map[string]client.MatchingLabels
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			clustersMap, matchingLabelsMap = initSysAccountTests(mysqlTestCases)
		})

		It("Patch Cluster after running", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					acctList   dbaasv1alpha1.KBAccountType
					jobsNum    int
					secretsNum int
				)

				clusterInfo := testCase.clusterInfo
				for _, acc := range clusterInfo.accounts {
					resource := testCase.envInfo.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}

				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				// patch cluster to running
				testCase.patchClusterFunc(clusterKey)

				// get latest cluster object
				cluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml, ok := matchingLabelsMap[testName]
				Expect(ok).To(BeTrue())

				// wait for a while till all jobs are created
				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}, timeout, interval).Should(Succeed())

				By("Enable monitor, more jobs and secrets should be created")
				// patch cluster, enable monitor
				Eventually(testdbaas.GetAndChangeObj(&testCtx, clusterKey, func(cluster *dbaasv1alpha1.Cluster) {
					for _, comp := range cluster.Spec.Components {
						comp.Monitor = true
					}
				}), timeout, interval).Should(Succeed())

				resource := testCase.envInfo.resourceMap[dbaasv1alpha1.MonitorAccount]
				if resource.jobNum == 0 {
					continue
				}

				acctList |= dbaasv1alpha1.KBAccountMonitor
				jobsNum += resource.jobNum
				secretsNum += resource.secretNum

				jobs := &batchv1.JobList{}
				var jobSize1, secretSize1, cachedSecretSize1 int
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
					jobSize1 = len(jobs.Items)

					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(secrets.Items)).To(BeEquivalentTo(secretsNum))
					secretSize1 = len(secrets.Items)
					cachedSecretSize1 = len(systemAccountReconciler.SecretMapStore.ListKeys())
				}, timeout, interval).Should(Succeed())

				By("Mark partial jobs as completed and make sure it cannot be found")
				// mark one jobs as completed
				if jobsNum < 2 {
					continue
				}

				jobToDelete := jobs.Items[0]
				jobKey := client.ObjectKeyFromObject(&jobToDelete)
				Expect(k8sClient.Delete(ctx, &jobToDelete)).To(Succeed())
				Expect(testdbaas.ChangeObj(&testCtx, &jobToDelete, func() { controllerutil.RemoveFinalizer(&jobToDelete, orphanFinalizerName) })).To(Succeed())

				Eventually(func(g Gomega) {
					tmpJob := &batchv1.Job{}
					g.Expect(k8sClient.Get(ctx, jobKey, tmpJob)).To(Satisfy(apierrors.IsNotFound))
				}, timeout, interval).Should(Succeed())

				By("Verify jobs size decreased and secrets size increased")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					jobSize2 := len(jobs.Items)
					g.Expect(jobSize2).To(BeNumerically("<", jobSize1))

					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					secretsSize2 := len(secrets.Items)
					g.Expect(secretsSize2).To(BeNumerically("<", secretSize1))

					cachedSecretsSize2 := len(systemAccountReconciler.SecretMapStore.ListKeys())
					g.Expect(cachedSecretsSize2).To(BeNumerically("<", cachedSecretSize1))

					g.Expect(cachedSecretSize1 - cachedSecretsSize2).To(BeEquivalentTo(secretsSize2 - secretSize1))
				}, timeout, interval).Should(Succeed())
			}
		})
	})
})
