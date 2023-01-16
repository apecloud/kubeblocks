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

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("SystemAccount Controller", func() {
	var (
		timeout            = time.Second * 10
		interval           = time.Second
		clusterName        = "cluster-sysaccount"
		clusterDefName     = "def-sysaccount"
		typeName           = "mycomponent"
		clusterEngineType  = "state.mysql-8"
		ClusterVersionName = "app-version-sysaccount"
		backupPolicyName   = "backup-policy-demo"
		databaseEngine     = "mysql"
	)
	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Service{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Endpoints{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &batchv1.Job{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dataprotectionv1alpha1.BackupPolicy{}, client.InNamespace(testCtx.DefaultNamespace))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
	})

	const (
		leader   = "leader"
		follower = "follower"
	)

	mockContainer := func() corev1.Container {
		container := corev1.Container{
			Name:            "mysql",
			Image:           "apecloud/wesql-server:8.0.30",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/bin/bash", "-c"},
			Env: []corev1.EnvVar{
				{Name: "MYSQL_ROOT_USER", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "$(CONN_CREDENTIAL_SECRET_NAME)"},
						Key:                  "username",
					},
				}},
				{Name: "MYSQL_ROOT_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "$(CONN_CREDENTIAL_SECRET_NAME)"},
						Key:                  "password",
					},
				}},
			},
			Ports: []corev1.ContainerPort{
				{ContainerPort: 3306, Name: "mysql", Protocol: corev1.ProtocolTCP},
				{ContainerPort: 13306, Name: "paxos", Protocol: corev1.ProtocolTCP},
			},
		}
		return container
	}

	mockClusterDefinition := func(clusterDefName string, clusterEngineType string, typeName string) *dbaasv1alpha1.ClusterDefinition {
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterDefName,
				Namespace: "default",
			},
			Spec: dbaasv1alpha1.ClusterDefinitionSpec{
				Type: clusterEngineType,
				Components: []dbaasv1alpha1.ClusterDefinitionComponent{
					{
						TypeName:        typeName,
						DefaultReplicas: 3,
						MinReplicas:     0,
						ComponentType:   dbaasv1alpha1.Consensus,
						ConsensusSpec: &dbaasv1alpha1.ConsensusSetSpec{
							Leader:    dbaasv1alpha1.ConsensusMember{Name: leader, AccessMode: dbaasv1alpha1.ReadWrite},
							Followers: []dbaasv1alpha1.ConsensusMember{{Name: follower, AccessMode: dbaasv1alpha1.Readonly}},
						},
						Service: &corev1.ServiceSpec{Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 3306}}},
						Probes: &dbaasv1alpha1.ClusterDefinitionProbes{
							RoleChangedProbe: &dbaasv1alpha1.ClusterDefinitionProbe{PeriodSeconds: 1},
						},
						PodSpec: &corev1.PodSpec{
							Containers: []corev1.Container{mockContainer()},
						},
						SystemAccounts: &dbaasv1alpha1.SystemAccountSpec{
							CmdExecutorConfig: &dbaasv1alpha1.CmdExecutorConfig{
								Image:   "mysql-8.0.30",
								Command: []string{"mysql", "-e", "$(KB_ACCOUNT_STATEMENT)"},
							},
							PasswordConfig: dbaasv1alpha1.PasswordConfig{
								Length:     10,
								NumDigits:  10,
								NumSymbols: 0,
								LetterCase: dbaasv1alpha1.MixedCases,
							},
							Accounts: []dbaasv1alpha1.SystemAccountConfig{
								{
									Name: dbaasv1alpha1.AdminAccount,
									ProvisionPolicy: dbaasv1alpha1.ProvisionPolicy{
										Type:  dbaasv1alpha1.CreateByStmt,
										Scope: dbaasv1alpha1.AnyPods,
										Statements: &dbaasv1alpha1.ProvisionStatements{
											CreationStatement: `CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY "$(PASSWD)"; GRANT ALL PRIVILEGES ON *.* TO $(USERNAME);`,
											DeletionStatement: `DROP USER IF EXISTS $(USERNAME);`},
									},
								},
								{
									Name: dbaasv1alpha1.DataprotectionAccount,
									ProvisionPolicy: dbaasv1alpha1.ProvisionPolicy{
										Type:  dbaasv1alpha1.CreateByStmt,
										Scope: dbaasv1alpha1.AllPods,
										Statements: &dbaasv1alpha1.ProvisionStatements{
											CreationStatement: `CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY "$(PASSWD)"; GRANT ALL PRIVILEGES ON *.* TO $(USERNAME);`,
											DeletionStatement: `DROP USER IF EXISTS $(USERNAME);`},
									},
								},
								{
									Name: dbaasv1alpha1.ProbeAccount,
									ProvisionPolicy: dbaasv1alpha1.ProvisionPolicy{
										Type: dbaasv1alpha1.ReferToExisting,
										SecretRef: &dbaasv1alpha1.ProvisionSecretRef{
											Name:      "$(CONN_CREDENTIAL_SECRET_NAME)",
											Namespace: "default",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		return clusterDefinition
	}

	mockClusterVersion := func(appverName string, clusterDefName string, typeName string) *dbaasv1alpha1.ClusterVersion {
		ClusterVersion := &dbaasv1alpha1.ClusterVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name: appverName,
			},
			Spec: dbaasv1alpha1.ClusterVersionSpec{
				ClusterDefinitionRef: clusterDefName,
				Components: []dbaasv1alpha1.ClusterVersionComponent{
					{
						Type: typeName,
						PodSpec: &corev1.PodSpec{
							Containers: []corev1.Container{mockContainer()},
						},
					},
				},
			},
		}
		return ClusterVersion
	}

	mockCluster := func(clusterDefName, appVerName, typeName, clusterName string, replicas int32) *dbaasv1alpha1.Cluster {
		clusterObj := &dbaasv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      clusterName,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterVersionRef: appVerName,
				ClusterDefRef:     clusterDefName,
				Components: []dbaasv1alpha1.ClusterComponent{
					{
						Name:     typeName,
						Type:     typeName,
						Replicas: &replicas,
					},
				},
				TerminationPolicy: dbaasv1alpha1.WipeOut,
			},
		}
		return clusterObj
	}

	mockBackupPolicy := func(name string, engineName, clusterName string) *dataprotectionv1alpha1.BackupPolicy {
		ml := map[string]string{
			intctrlutil.AppInstanceLabelKey: clusterName,
		}

		policy := &dataprotectionv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
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
					Name: "mock-volumn",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "mock-pvc"},
					},
				},
			},
		}
		return policy
	}

	assureClusterDef := func() *dbaasv1alpha1.ClusterDefinition {
		By("Creating cluster definition")
		// create cluster def
		clusterDef := mockClusterDefinition(clusterDefName, clusterEngineType, typeName)
		Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())
		// assure cluster def is ready
		createdClusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: clusterDefName}, createdClusterDef)
		}, timeout, interval).Should(Succeed())
		return createdClusterDef
	}

	assureClusterVersion := func() *dbaasv1alpha1.ClusterVersion {
		// create app version
		ClusterVersion := mockClusterVersion(ClusterVersionName, clusterDefName, typeName)
		Expect(testCtx.CheckedCreateObj(ctx, ClusterVersion)).Should(Succeed())
		// assure cluster def is ready
		createdClusterVersion := &dbaasv1alpha1.ClusterVersion{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: ClusterVersionName}, createdClusterVersion)
		}, timeout, interval).Should(Succeed())
		return createdClusterVersion
	}

	assureCluster := func(replicas int32) *dbaasv1alpha1.Cluster {
		By("Creating cluster")
		cluster := mockCluster(clusterDefName, ClusterVersionName, typeName, clusterName, replicas)
		Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())
		createdCluster := &dbaasv1alpha1.Cluster{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Name, Namespace: cluster.Namespace}, createdCluster)
		}, timeout, interval).Should(Succeed())
		return createdCluster
	}

	patchCluster := func(key types.NamespacedName) {
		By("Patching Cluster to trigger reconcile")
		Eventually(changeSpec(key, func(cluster *dbaasv1alpha1.Cluster) {
			if cluster.Annotations == nil {
				cluster.Annotations = make(map[string]string)
			}
			cluster.Annotations["mockmode"] = "testing"
		}), timeout, interval).Should(Succeed())
	}

	assureBackupPolicy := func(policyName, engintName, clusterName string) *dataprotectionv1alpha1.BackupPolicy {
		By("Creating Backup Policy")
		policy := mockBackupPolicy(policyName, engintName, clusterName)
		Expect(testCtx.CheckedCreateObj(ctx, policy)).Should(Succeed())

		createdPolicy := &dataprotectionv1alpha1.BackupPolicy{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: policy.Name, Namespace: policy.Namespace}, createdPolicy)
		}, timeout, interval).Should(Succeed())
		return createdPolicy
	}

	mockEndpoint := func(namespace, endpointName string) *corev1.Endpoints {
		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      endpointName,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{
							IP:       "10.0.0.0",
							NodeName: nil,
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: namespace,
								Name:      "pod0",
							},
						},
					},
				},
			},
		}
		return ep
	}

	mockHeadlessEndpoint := func(namespace, endpointName string) *corev1.Endpoints {
		ep := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      endpointName,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{
							IP:       "10.0.0.0",
							NodeName: nil,
							Hostname: "pod0",
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: namespace,
								Name:      "pod0",
							},
						},
						{
							IP:       "10.0.0.1",
							NodeName: nil,
							Hostname: "pod1",
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: namespace,
								Name:      "pod1",
							},
						},
						{
							IP:       "10.0.0.2",
							NodeName: nil,
							Hostname: "pod2",
							TargetRef: &corev1.ObjectReference{
								Kind:      "Pod",
								Namespace: namespace,
								Name:      "pod2",
							},
						},
					},
				},
			},
		}
		return ep
	}

	assureEndpont := func(namespace, epname string) *corev1.Endpoints {
		ep := mockEndpoint(namespace, epname)
		Expect(testCtx.CheckedCreateObj(ctx, ep)).Should(Succeed())
		// assure cluster def is ready
		createdEP := &corev1.Endpoints{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: epname, Namespace: namespace}, createdEP)
		}, timeout, interval).Should(Succeed())
		return createdEP
	}

	assureHeadlessEndpont := func(namespace, epname string) *corev1.Endpoints {
		ep := mockHeadlessEndpoint(namespace, epname)
		Expect(testCtx.CheckedCreateObj(ctx, ep)).Should(Succeed())
		// assure cluster def is ready
		createdEP := &corev1.Endpoints{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: epname, Namespace: namespace}, createdEP)
		}, timeout, interval).Should(Succeed())
		return createdEP
	}

	Context("When Creating Consensus Cluster", func() {
		It("Should update system account expectation", func() {
			if testEnv.UseExistingCluster != nil && *testEnv.UseExistingCluster {
				Skip("Mocked Cluster is not fully implemented to run in real cluster.")
			}
			createdClusterDef := assureClusterDef()
			ClusterVersion := assureClusterVersion()

			By("Assuring ClusterDef and ClusterVersion are Available")
			Eventually(func() bool {
				tmpClusterDef := &dbaasv1alpha1.ClusterDefinition{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: createdClusterDef.Name, Namespace: createdClusterDef.Namespace}, tmpClusterDef)
				if err != nil {
					return false
				}
				return tmpClusterDef.Status.Phase == dbaasv1alpha1.AvailablePhase
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				tmpClusterVersion := &dbaasv1alpha1.ClusterVersion{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: ClusterVersion.Name, Namespace: ClusterVersion.Namespace}, tmpClusterVersion)
				if err != nil {
					return false
				}
				return tmpClusterVersion.Status.Phase == dbaasv1alpha1.AvailablePhase
			}, timeout, interval).Should(BeTrue())

			// make sure cluster is under creation
			cluster := assureCluster(3)
			compName := cluster.Spec.Components[0].Name
			// services of type ClusterIP should have been created.
			serviceName := cluster.Name + "-" + compName
			headlessServiceName := serviceName + "-headless"
			_ = assureEndpont(cluster.Namespace, serviceName)
			_ = assureHeadlessEndpont(cluster.Namespace, headlessServiceName)

			patchCluster(intctrlutil.GetNamespacedName(cluster))

			ml := getLabelsForSecretsAndJobs(cluster.Name, createdClusterDef.Spec.Type, createdClusterDef.Name, compName)

			getAccounts := func(g Gomega) dbaasv1alpha1.KBAccountType {
				secrets := &corev1.SecretList{}
				g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				jobs := &batchv1.JobList{}
				g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				return getAccountFacts(secrets, jobs)
			}

			By("Verify two accounts to be created")
			Eventually(func(g Gomega) {
				accounts := getAccounts(g)
				g.Expect(accounts).To(BeEquivalentTo(dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountProbe))
			}, timeout, interval).Should(Succeed())

			By("Assure some Secrets creation are cached")
			secretsToCreate1 := 0
			Eventually(func() bool {
				secretsToCreate1 = len(systemAccountReconciler.SecretMapStore.ListKeys())
				return secretsToCreate1 > 0
			}, timeout, interval).Should(BeTrue())

			By("Create a Backup Policy")
			policy := assureBackupPolicy(backupPolicyName, databaseEngine, clusterName)

			By("Check the BackupPolicy creation filters run")
			policyKey := expectationKey(policy.Namespace, clusterName, databaseEngine)
			Eventually(func(g Gomega) {
				exp, exists, _ := systemAccountReconciler.ExpectionManager.getExpectation(policyKey)
				g.Expect(exists).To(BeTrue())
				g.Expect(exp.toCreate&dbaasv1alpha1.KBAccountDataprotection > 0).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Verify three accounts have been created in total")
			Eventually(func(g Gomega) {
				accounts := getAccounts(g)
				g.Expect(accounts).To(BeEquivalentTo(dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection | dbaasv1alpha1.KBAccountProbe))
			}, timeout, interval).Should(Succeed())

			By("Assure more Secrets creation are cached")
			secretsToCreate2 := 0
			Eventually(func() bool {
				secretsToCreate2 = len(systemAccountReconciler.SecretMapStore.ListKeys())
				return secretsToCreate2 > secretsToCreate1
			}, timeout, interval).Should(BeTrue())

			secretsCreated := 0
			Eventually(func(g Gomega) {
				secrets := &corev1.SecretList{}
				g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				secretsCreated = len(secrets.Items)
			}, timeout, interval).Should(Succeed())

			By("Mock all jobs completed and deleted")
			Eventually(func(g Gomega) {
				jobs := &batchv1.JobList{}
				g.Expect(k8sClient.List(ctx, jobs, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				for _, job := range jobs.Items {
					g.Expect(changeStatus(intctrlutil.GetNamespacedName(&job), func(job *batchv1.Job) {
						job.Status.Conditions = []batchv1.JobCondition{{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						}}
					})).To(Succeed())
					g.Expect(k8sClient.Delete(ctx, &job)).To(Succeed())
					g.Expect(changeSpec(intctrlutil.GetNamespacedName(&job), func(job *batchv1.Job) {
						job.SetFinalizers([]string{})
					})).To(Succeed())
				}
			}, timeout, interval).Should(Succeed())

			By("Check all secrets creation are completed")
			Eventually(func(g Gomega) {
				secrets := &corev1.SecretList{}
				g.Expect(k8sClient.List(ctx, secrets, client.InNamespace(cluster.Namespace), ml)).To(Succeed())
				g.Expect(len(secrets.Items) == secretsCreated+secretsToCreate2).To(BeTrue())
				g.Expect(len(systemAccountReconciler.SecretMapStore.ListKeys()) == 0).To(BeTrue())
			}, timeout, interval).Should(Succeed())

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
})
