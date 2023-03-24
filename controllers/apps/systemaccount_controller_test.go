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

package apps

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("SystemAccount Controller", func() {

	const (
		clusterDefName         = "test-clusterdef"
		clusterVersionName     = "test-clusterversion"
		clusterNamePrefix      = "test-cluster"
		mysqlCompType          = "replicasets"
		mysqlCompTypeWOSysAcct = "wo-sysacct"
		mysqlCompName          = "mysql"
		mysqlCompNameWOSysAcct = "wo-sysacct"
		orphanFinalizerName    = "orphan"
		clusterEndPointsSize   = 3
	)

	/**
		* To test the behavior of system accounts controller, we conduct following tests:
		* 1. construct two components, one with all accounts set, and one with none.
		* 2. create two clusters, one cluster for each component, and verify
	  * a) the number of secrets, jobs, and cached secrets are as expected
		* b) secret will be created, once corresponding job succeeds.
		* c) secrets, deleted accidentially, will be re-created during next cluster reconciliation round.
		*
		* Each test case, used in following IT(integration test), consists of two parts:
		* a) how to build the test cluster, and
		* b) what does this cluster expect
	**/

	// sysAcctResourceInfo defines the number of jobs and secrets to be created per account.
	type sysAcctResourceInfo struct {
		jobNum           int
		secretNum        int
		cachedSecretsNum int
	}
	// sysAcctTestCase defines the info to setup test env, cluster and their expected result to verify against.
	type sysAcctTestCase struct {
		componentName   string
		componentDefRef string
		resourceMap     map[appsv1alpha1.AccountName]sysAcctResourceInfo
		accounts        []appsv1alpha1.AccountName // accounts this cluster should have
	}

	var (
		ctx               = context.Background()
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		testapps.ClearClusterResources(&testCtx)

		// namespaced resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, intctrlutil.EndpointsSignature, inNS, ml)
	}

	cleanInternalCache := func() {
		secretKeys := systemAccountReconciler.SecretMapStore.ListKeys()
		for _, key := range secretKeys {
			_ = systemAccountReconciler.SecretMapStore.deleteSecret(key)
		}
	}

	/**
	 * Start of mock functions.
	 **/
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
		}).Should(Succeed())
		return createdEP
	}
	/*
	 * end of mock functions to be refined
	 */

	/*
	 * Start of helper functions
	 */
	getAccounts := func(g Gomega, cluster *appsv1alpha1.Cluster, ml client.MatchingLabels) appsv1alpha1.KBAccountType {
		secrets := &corev1.SecretList{}
		g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace))).To(Succeed())
		g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		jobs := &batchv1.JobList{}
		g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		return getAccountFacts(secrets, jobs)
	}

	checkOwnerReferenceToObj := func(ref metav1.OwnerReference, obj client.Object) bool {
		return ref.Name == obj.GetName() && ref.UID == obj.GetUID()
	}

	patchClusterToRunning := func(objectKey types.NamespacedName, compName string) {
		// services of type ClusterIP should have been created.
		ips := []string{"10.0.0.0", "10.0.0.1", "10.0.0.2"}
		serviceName := objectKey.Name + "-" + compName
		headlessServiceName := serviceName + "-headless"
		_ = assureEndpoint(objectKey.Namespace, serviceName, ips[0:1])
		_ = assureEndpoint(objectKey.Namespace, headlessServiceName, ips[0:clusterEndPointsSize])

		By("Patching Cluster to running phase")
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, objectKey, func(cluster *appsv1alpha1.Cluster) {
			cluster.Status.Phase = appsv1alpha1.RunningPhase
		})).Should(Succeed())
	}

	initSysAccountTestsAndCluster := func(testCases map[string]*sysAcctTestCase) (clustersMap map[string]types.NamespacedName) {
		// create clusterdef and cluster verions, but not clusters
		By("Create a clusterDefinition obj")
		systemAccount := mockSystemAccountsSpec()
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
			AddSystemAccountSpec(systemAccount).
			AddComponent(testapps.StatefulMySQLComponent, mysqlCompTypeWOSysAcct).
			Create(&testCtx).GetObject()

		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponent(mysqlCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponent(mysqlCompNameWOSysAcct).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			Create(&testCtx).GetObject()

		Expect(clusterDefObj).NotTo(BeNil())

		Expect(len(testCases)).To(BeNumerically(">", 0))
		// fill the number of secrets, jobs, and cached secrets
		for _, testCase := range testCases {
			compDef := clusterDefObj.GetComponentDefByName(testCase.componentDefRef)
			Expect(compDef).NotTo(BeNil())
			if compDef.SystemAccounts == nil {
				continue
			}
			if testCase.resourceMap == nil {
				testCase.resourceMap = make(map[appsv1alpha1.AccountName]sysAcctResourceInfo)
			}
			var jobNum, secretNum, cachedSecretNum int
			for _, account := range compDef.SystemAccounts.Accounts {
				name := account.Name
				policy := account.ProvisionPolicy
				switch policy.Type {
				case appsv1alpha1.CreateByStmt:
					secretNum = 0
					cachedSecretNum = 1
					if policy.Scope == appsv1alpha1.AnyPods {
						jobNum = 1
					} else {
						jobNum = clusterEndPointsSize
					}
				case appsv1alpha1.ReferToExisting:
					jobNum = 0
					cachedSecretNum = 0
					secretNum = 1
				}
				testCase.resourceMap[name] = sysAcctResourceInfo{
					jobNum:           jobNum,
					cachedSecretsNum: cachedSecretNum,
					secretNum:        secretNum,
				}
			}
		}

		clustersMap = make(map[string]types.NamespacedName)

		// create cluster defined in each testcase
		for testName, testCase := range testCases {
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testCase.componentName, testCase.componentDefRef).
				SetReplicas(1).
				Create(&testCtx).GetObject()
			clusterKey := client.ObjectKeyFromObject(clusterObj)
			clustersMap[testName] = clusterKey

			By("Make sure cluster root conn credential is ready.")
			Eventually(func(g Gomega) {
				rootSecretName := component.GenerateConnCredential(clusterKey.Name)
				rootSecret := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterKey.Namespace,
					Name:      rootSecretName}, rootSecret)).To(Succeed())
			}).Should(Succeed())
		}
		return
	}
	/*
	 * end of helper functions
	 */

	// scenario 1: create cluster and check secrets and jobs are created
	Context("When Creating Cluster", func() {
		var (
			clustersMap    map[string]types.NamespacedName
			mysqlTestCases map[string]*sysAcctTestCase
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			// setup testcase
			mysqlTestCases = map[string]*sysAcctTestCase{
				"wesql-no-accts": {
					componentName:   mysqlCompNameWOSysAcct,
					componentDefRef: mysqlCompTypeWOSysAcct,
					accounts:        []appsv1alpha1.AccountName{},
				},
				"wesql-with-accts": {
					componentName:   mysqlCompName,
					componentDefRef: mysqlCompType,
					accounts:        getAllSysAccounts(),
				},
			}
			clustersMap = initSysAccountTestsAndCluster(mysqlTestCases)
		})

		It("Should create jobs and cache secrets as expected for each test case", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					acctList        appsv1alpha1.KBAccountType
					jobsNum         int
					secretsNum      int
					cachedSecretNum int
				)

				for _, acc := range testCase.accounts {
					resource := testCase.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
					cachedSecretNum += resource.cachedSecretsNum
				}

				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())

				// get latest cluster object
				cluster := &appsv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())
				// patch cluster to running
				patchClusterToRunning(clusterKey, testCase.componentName)

				ml := getLabelsForSecretsAndJobs(componentUniqueKey{namespace: cluster.Namespace, clusterName: cluster.Name, componentName: testCase.componentName})

				if secretsNum == 0 && jobsNum == 0 && cachedSecretNum == 0 {
					By("No accouts should be create for test case: " + testName)
					// verify nothing will be created or cached till timeout
					Consistently(func(g Gomega) {
						accounts := getAccounts(g, cluster, ml)
						g.Expect(accounts).To(BeEquivalentTo(acctList))
					}).Should(Succeed())
					continue
				}

				By("Verify accounts to be created are correct")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}).Should(Succeed())

				By("Assure some secrets have been cached")
				Eventually(func() int {
					return len(systemAccountReconciler.SecretMapStore.ListKeys())
				}).Should(BeEquivalentTo(cachedSecretNum))

				By("Verify all jobs created have their lables set correctly")
				// get all jobs
				Eventually(func(g Gomega) {
					// all jobs matching filter `ml` should be a job for sys account.
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					for _, job := range jobs.Items {
						_, ok := job.Labels[constant.ClusterAccountLabelKey]
						g.Expect(ok).To(BeTrue())
						g.Expect(len(job.ObjectMeta.OwnerReferences)).To(BeEquivalentTo(1))
						g.Expect(checkOwnerReferenceToObj(job.OwnerReferences[0], cluster)).To(BeTrue())
					}
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}).Should(Succeed())
			}
		})

		// It("Cached secrets should be created when jobs succeeds", func() {
		// 	for testName, testCase := range mysqlTestCases {
		// 		var (
		// 			acctList        appsv1alpha1.KBAccountType
		// 			jobsNum         int
		// 			secretsNum      int
		// 			cachedSecretNum int
		// 		)

		// 		for _, acc := range testCase.accounts {
		// 			resource := testCase.resourceMap[acc]
		// 			acctList |= acc.GetAccountID()
		// 			jobsNum += resource.jobNum
		// 			secretsNum += resource.secretNum
		// 			cachedSecretNum += resource.cachedSecretsNum
		// 		}

		// 		if secretsNum == 0 && jobsNum == 0 && cachedSecretNum == 0 {
		// 			continue
		// 		}
		// 		// get a cluster instance from map, created during preparation
		// 		clusterKey, ok := clustersMap[testName]
		// 		Expect(ok).To(BeTrue())
		// 		// patch cluster to running
		// 		patchClusterToRunning(clusterKey, testCase.componentName)

		// 		// get cluster object
		// 		cluster := &appsv1alpha1.Cluster{}
		// 		Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

		// 		ml := getLabelsForSecretsAndJobs(componentUniqueKey{
		// 			namespace:     cluster.Namespace,
		// 			clusterName:   cluster.Name,
		// 			componentName: testCase.componentName})

		// 		By("Verify accounts to be created are correct")
		// 		Eventually(func(g Gomega) {
		// 			accounts := getAccounts(g, cluster, ml)
		// 			g.Expect(accounts).To(BeEquivalentTo(acctList))
		// 		}).Should(Succeed())

		// 		// REVIEW: need to revise this test case, caught intermittent error:
		// 		//  [FAILED] Expected
		// 		//   <int>: 3
		// 		//   to be equivalent to
		// 		// 	 <int>: 4
		// 		// [FAILED] Timed out after 10.000s.
		// 		//   Expected
		// 		// 	 <int>: 2
		// 		//   to be equivalent to
		// 		// 	 <int>: 3
		// 		// [FAILED] Timed out after 10.000s.
		// 		// Expected
		// 		// 	<int>: 3
		// 		// to be equivalent to
		// 		// 	<int>: 4
		// 		By("Verify secrets cached are correct")
		// 		Eventually(len(systemAccountReconciler.SecretMapStore.ListKeys())).Should(BeEquivalentTo(cachedSecretNum))

		// 		// wait for a while till all jobs are created
		// 		By("Mock all jobs are completed and deleted")
		// 		Eventually(func(g Gomega) {
		// 			jobs := &batchv1.JobList{}
		// 			g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		// 			g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
		// 			for _, job := range jobs.Items {
		// 				g.Expect(testapps.ChangeObjStatus(&testCtx, &job, func() {
		// 					job.Status.Conditions = []batchv1.JobCondition{{
		// 						Type:   batchv1.JobComplete,
		// 						Status: corev1.ConditionTrue,
		// 					}}
		// 				})).To(Succeed())
		// 				g.Expect(k8sClient.Delete(ctx, &job)).To(Succeed())
		// 			}
		// 		}).Should(Succeed())

		// 		// remove 'orphan' finalizers to make sure all jobs can be deleted.
		// 		Eventually(func(g Gomega) {
		// 			jobs := &batchv1.JobList{}
		// 			g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		// 			for _, job := range jobs.Items {
		// 				g.Expect(testapps.ChangeObj(&testCtx, &job, func() { controllerutil.RemoveFinalizer(&job, orphanFinalizerName) })).To(Succeed())
		// 			}
		// 			g.Expect(len(jobs.Items)).To(Equal(0), "Verify all jobs completed and deleted")
		// 		}).Should(Succeed())

		// 		By("Check secrets created")
		// 		Eventually(func(g Gomega) {
		// 			secrets := &corev1.SecretList{}
		// 			g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
		// 			g.Expect(len(secrets.Items)).To(BeEquivalentTo(secretsNum + cachedSecretNum))
		// 		}).Should(Succeed())

		// 		By("Verify all secrets created have their finalizer and lables set correctly")
		// 		// get all secrets, and check their lables and finalizer
		// 		Eventually(func(g Gomega) {
		// 			// get secrets matching filter
		// 			secretsForAcct := &corev1.SecretList{}
		// 			g.Expect(k8sClient.List(ctx, secretsForAcct, ml)).To(Succeed())
		// 			for _, secret := range secretsForAcct.Items {
		// 				// each secret has finalizer
		// 				g.Expect(controllerutil.ContainsFinalizer(&secret, DBClusterFinalizerName)).To(BeTrue())
		// 				g.Expect(len(secret.ObjectMeta.OwnerReferences)).To(BeEquivalentTo(1))
		// 				g.Expect(checkOwnerReferenceToObj(secret.OwnerReferences[0], cluster)).To(BeTrue())
		// 			}
		// 		}).Should(Succeed())
		// 	}
		// 	// all jobs succeeded, and there should be no cached secrets left behind.
		// 	Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(0))
		// })
	}) // end of context

	Context("When Delete Cluster", func() {
		var (
			clustersMap    map[string]types.NamespacedName
			mysqlTestCases map[string]*sysAcctTestCase
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			// setup testcase
			mysqlTestCases = map[string]*sysAcctTestCase{
				"wesql-with-accts": {
					componentName:   mysqlCompName,
					componentDefRef: mysqlCompType,
					accounts:        getAllSysAccounts(),
				},
				"wesql-with-accts-dup": {
					componentName:   mysqlCompName,
					componentDefRef: mysqlCompType,
					accounts:        getAllSysAccounts(),
				},
			}

			clustersMap = initSysAccountTestsAndCluster(mysqlTestCases)
		})

		It("Should clear relevant expectations and secrets after cluster deletion", func() {
			var totalJobs, totalSecrets, totalCachedSecrets int
			for testName, testCase := range mysqlTestCases {
				var (
					acctList        appsv1alpha1.KBAccountType
					jobsNum         int
					secretsNum      int
					cachedSecretNum int
				)

				for _, acc := range testCase.accounts {
					resource := testCase.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
					cachedSecretNum += resource.cachedSecretsNum
				}
				totalJobs += jobsNum
				totalSecrets += secretsNum
				totalCachedSecrets += cachedSecretNum

				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())

				// patch cluster to running
				patchClusterToRunning(clusterKey, testCase.componentName)

				// get latest cluster object
				cluster := &appsv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())
				ml := getLabelsForSecretsAndJobs(componentUniqueKey{namespace: cluster.Namespace, clusterName: cluster.Name, componentName: testCase.componentName})

				By("Verify accounts to be created")
				Eventually(func(g Gomega) {
					accounts := getAccounts(g, cluster, ml)
					g.Expect(accounts).To(BeEquivalentTo(acctList))
				}).Should(Succeed())

				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
				}).Should(Succeed())
			}

			By("Verify secrets and jobs size")
			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(totalCachedSecrets), "before delete, there are %d cached secrets", totalCachedSecrets)
			}).Should(Succeed())

			clusterKeys := make([]types.NamespacedName, 0, len(clustersMap))
			for _, v := range clustersMap {
				clusterKeys = append(clusterKeys, v)
			}

			By("Delete 0-th cluster from list, there should be no change in cached secrets size")
			cluster := &appsv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, clusterKeys[0], cluster)).To(Succeed())
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())

			By("Delete remaining cluster before jobs are done, all cached secrets should be removed")
			for i := 1; i < len(clusterKeys); i++ {
				cluster = &appsv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKeys[i], cluster)).To(Succeed())
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}

			Eventually(func(g Gomega) {
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys())).To(BeEquivalentTo(0))
			}).Should(Succeed())
		})
	}) // end of context

	Context("When Update Cluster", func() {
		var (
			clustersMap    map[string]types.NamespacedName
			mysqlTestCases map[string]*sysAcctTestCase
		)

		BeforeEach(func() {
			cleanEnv()
			DeferCleanup(cleanEnv)

			cleanInternalCache()
			DeferCleanup(cleanInternalCache)

			// setup testcase
			mysqlTestCases = map[string]*sysAcctTestCase{
				"wesql-with-accts": {
					componentName:   mysqlCompName,
					componentDefRef: mysqlCompType,
					accounts:        getAllSysAccounts(),
				},
				"wesql-with-accts-dup": {
					componentName:   mysqlCompName,
					componentDefRef: mysqlCompType,
					accounts:        getAllSysAccounts(),
				},
			}
			clustersMap = initSysAccountTestsAndCluster(mysqlTestCases)
		})

		It("Patch Cluster after running", func() {
			for testName, testCase := range mysqlTestCases {
				var (
					acctList   appsv1alpha1.KBAccountType
					jobsNum    int
					secretsNum int
				)

				for _, acc := range testCase.accounts {
					resource := testCase.resourceMap[acc]
					acctList |= acc.GetAccountID()
					jobsNum += resource.jobNum
					secretsNum += resource.secretNum
				}

				// get a cluster instance from map, created during preparation
				clusterKey, ok := clustersMap[testName]
				Expect(ok).To(BeTrue())
				// patch cluster to running
				patchClusterToRunning(clusterKey, testCase.componentName)

				// get latest cluster object
				cluster := &appsv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, clusterKey, cluster)).Should(Succeed())

				ml := getLabelsForSecretsAndJobs(componentUniqueKey{namespace: cluster.Namespace, clusterName: cluster.Name, componentName: testCase.componentName})

				// wait for a while till all jobs are created
				Eventually(func(g Gomega) {
					jobs := &batchv1.JobList{}
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))

					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(secrets.Items)).To(BeEquivalentTo(secretsNum))
				}).Should(Succeed())

				By("Enable monitor, no more jobs or secrets should be created")
				// patch cluster, flip comp.Monitor
				Eventually(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1alpha1.Cluster) {
					for _, comp := range cluster.Spec.ComponentSpecs {
						comp.Monitor = !comp.Monitor
					}
				})).Should(Succeed())

				jobs := &batchv1.JobList{}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(jobs.Items)).To(BeEquivalentTo(jobsNum))
					// nothing changed since last time updates
					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					g.Expect(len(secrets.Items)).To(BeEquivalentTo(secretsNum))
				}).Should(Succeed())

				cachedSecretNum := len(systemAccountReconciler.SecretMapStore.ListKeys())
				By("Mark partial jobs as completed and make sure it cannot be found")
				// mark one jobs as completed
				if jobsNum < 2 {
					continue
				}
				// delete one job, but the job IS NOT completed.
				By("Delete one job directly, the system should not create new secrets.")
				jobToDelete := jobs.Items[0]
				jobKey := client.ObjectKeyFromObject(&jobToDelete)
				Expect(k8sClient.Delete(ctx, &jobToDelete)).To(Succeed())

				Eventually(func(g Gomega) {
					tmpJob := &batchv1.Job{}
					g.Expect(k8sClient.Get(ctx, jobKey, tmpJob)).To(Succeed())
					g.Expect(len(tmpJob.ObjectMeta.Finalizers)).To(BeEquivalentTo(1))
					g.Expect(testapps.ChangeObj(&testCtx, tmpJob, func() { controllerutil.RemoveFinalizer(tmpJob, orphanFinalizerName) })).To(Succeed())
				}).Should(Succeed())

				By("Verify jobs size decreased and secrets size increased")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					jobSize2 := len(jobs.Items)
					g.Expect(jobSize2).To(BeNumerically("<", jobsNum))

					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					secretsSize2 := len(secrets.Items)
					g.Expect(secretsSize2).To(BeEquivalentTo(secretsNum))

					cachedSecretsSize2 := len(systemAccountReconciler.SecretMapStore.ListKeys())
					g.Expect(cachedSecretsSize2).To(BeEquivalentTo(cachedSecretNum))
				}).Should(Succeed())

				// delete one job directly, but the job is completed.
				By("Delete one job directly, the system should not create new secrets.")
				jobKey = client.ObjectKeyFromObject(&jobs.Items[0])
				Eventually(func(g Gomega) {
					tmpJob := &batchv1.Job{}
					g.Expect(k8sClient.Get(ctx, jobKey, tmpJob)).To(Succeed())
					g.Expect(testapps.ChangeObjStatus(&testCtx, tmpJob, func() {
						tmpJob.Status.Conditions = []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}}
					})).To(Succeed())
					g.Expect(k8sClient.Delete(ctx, tmpJob)).To(Succeed())
				}).Should(Succeed())

				Eventually(func(g Gomega) {
					tmpJob := &batchv1.Job{}
					err := k8sClient.Get(ctx, jobKey, tmpJob)
					g.Expect(err).To(Succeed())
					g.Expect(len(tmpJob.ObjectMeta.Finalizers)).To(BeEquivalentTo(1))
					g.Expect(testapps.ChangeObj(&testCtx, tmpJob, func() { controllerutil.RemoveFinalizer(tmpJob, orphanFinalizerName) })).To(Succeed())
				}).Should(Succeed())

				By("Verify jobs size decreased and secrets size increased")
				Eventually(func(g Gomega) {
					secrets := &corev1.SecretList{}
					g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
					secretsSize2 := len(secrets.Items)
					g.Expect(secretsSize2).To(BeNumerically(">", secretsNum))

					cachedSecretsSize2 := len(systemAccountReconciler.SecretMapStore.ListKeys())
					g.Expect(cachedSecretsSize2).To(BeNumerically("<", cachedSecretNum))
				}).Should(Succeed())
			}
		})
	})
})
