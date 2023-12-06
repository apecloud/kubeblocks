/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("clusterDefinition webhook", func() {
	var (
		randomStr              = testCtx.GetRandomStr()
		clusterDefinitionName  = "webhook-cd-" + randomStr
		clusterDefinitionName2 = "webhook-cd2" + randomStr
		clusterDefinitionName3 = "webhook-cd3" + randomStr
	)
	cleanupObjects := func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}
	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	Context("When clusterDefinition create and update", func() {
		It("Should webhook validate passed", func() {

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)).Should(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ = createTestClusterDefinitionObj3(clusterDefinitionName3)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName3}, clusterDef)).Should(Succeed())

			By("By creating a new clusterDefinition with workloadType==Consensus but consensusSpec not present")
			clusterDef, _ = createTestClusterDefinitionObj2(clusterDefinitionName2)
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			By("Set Leader.Replicas > 1")
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec = &ConsensusSetSpec{Leader: DefaultLeader}
			replicas := int32(2)
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec.Leader.Replicas = &replicas
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())
			// restore clusterDef
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec.Leader.Replicas = nil

			By("Set Followers.Replicas to odd")
			followers := make([]ConsensusMember, 1)
			rel := int32(3)
			followers[0] = ConsensusMember{Name: "follower", AccessMode: "Readonly", Replicas: &rel}
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec.Followers = followers
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())
		})

		It("Validate Cluster Definition System Accounts", func() {
			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj3(clusterDefinitionName3)
			cmdExecConfig := &CmdExecutorConfig{
				CommandExecutorEnvItem: CommandExecutorEnvItem{
					Image: "mysql-8.0.30",
				},
				CommandExecutorItem: CommandExecutorItem{
					Command: []string{"mysql", "-e", "$(KB_ACCOUNT_STATEMENT)"},
				},
			}
			By("By creating a new clusterDefinition with duplicated accounts")
			mockAccounts := []SystemAccountConfig{
				{
					Name: AdminAccount,
					ProvisionPolicy: ProvisionPolicy{
						Type: CreateByStmt,
						Statements: &ProvisionStatements{
							CreationStatement: `CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY "$(PASSWD)"; `,
							DeletionStatement: "DROP USER IF EXISTS $(USERNAME);",
						},
					},
				},
				{
					Name: AdminAccount,
					ProvisionPolicy: ProvisionPolicy{
						Type: CreateByStmt,
						Statements: &ProvisionStatements{
							CreationStatement: `CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY "$(PASSWD)"; `,
							DeletionStatement: "DROP USER IF EXISTS $(USERNAME);",
						},
					},
				},
			}
			passwdConfig := PasswordConfig{
				Length: 10,
			}
			clusterDef.Spec.ComponentDefs[0].SystemAccounts = &SystemAccountSpec{
				CmdExecutorConfig: cmdExecConfig,
				PasswordConfig:    passwdConfig,
				Accounts:          mockAccounts,
			}
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			// fix duplication error
			mockAccounts[1].Name = ProbeAccount
			By("By creating a new clusterDefinition with invalid password setting")
			// test password config
			invalidPasswdConfig := PasswordConfig{
				Length:     10,
				NumDigits:  10,
				NumSymbols: 10,
			}
			clusterDef.Spec.ComponentDefs[0].SystemAccounts = &SystemAccountSpec{
				CmdExecutorConfig: cmdExecConfig,
				PasswordConfig:    invalidPasswdConfig,
				Accounts:          mockAccounts,
			}
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition with statements missing")
			mockAccounts[0].ProvisionPolicy.Type = ReferToExisting
			clusterDef.Spec.ComponentDefs[0].SystemAccounts = &SystemAccountSpec{
				CmdExecutorConfig: cmdExecConfig,
				PasswordConfig:    passwdConfig,
				Accounts:          mockAccounts,
			}
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())
			// reset account setting
			mockAccounts[0].ProvisionPolicy.Type = CreateByStmt

			By("Create accounts with empty deletion and update statements, should fail")
			deletionStmt := mockAccounts[1].ProvisionPolicy.Statements.DeletionStatement
			mockAccounts[1].ProvisionPolicy.Statements.DeletionStatement = ""
			clusterDef.Spec.ComponentDefs[0].SystemAccounts = &SystemAccountSpec{
				CmdExecutorConfig: cmdExecConfig,
				PasswordConfig:    passwdConfig,
				Accounts:          mockAccounts,
			}
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())
			// reset account setting
			mockAccounts[1].ProvisionPolicy.Statements.DeletionStatement = deletionStmt

			By("By creating a new clusterDefinition with valid accounts")
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName3}, clusterDef)).Should(Succeed())

		})

		It("Validate Cluster Definition Component Refs", func() {
			By("By creating a new clusterDefinition")
			clusterDef, err := createMultiCompClusterDefObj(clusterDefinitionName3)
			Expect(err).ShouldNot(HaveOccurred())
			componentRefs := []ComponentDefRef{
				{
					ComponentRefEnvs: []ComponentRefEnv{
						{
							Name: "INJECTED_ENV",
							ValueFrom: &ComponentValueFrom{
								Type: FromHeadlessServiceRef,
							},
						},
					},
				},
			}
			By("By creating a new clusterDefinition with empty component name, should fail")
			clusterDef.Spec.ComponentDefs[0].ComponentDefRef = componentRefs
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition with invalid component name, should fail")
			componentRefs[0].ComponentDefName = "invalid-name"
			clusterDef.Spec.ComponentDefs[0].ComponentDefRef = componentRefs
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition with invalid workload type, should fail")
			componentRefs[0].ComponentDefName = "mysql-proxy"
			clusterDef.Spec.ComponentDefs[0].ComponentDefRef = componentRefs
			Expect(testCtx.CreateObj(ctx, clusterDef)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition with valid valueFrom type, should succeed")
			componentRefs[0].ComponentRefEnvs[0].ValueFrom = &ComponentValueFrom{
				Type: FromServiceRef,
			}
			clusterDef.Spec.ComponentDefs[0].ComponentDefRef = componentRefs
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
		})

		It("Should webhook validate configSpec", func() {
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName + "-cfg-test")
			tests := []struct {
				name               string
				tpls               []ComponentConfigSpec
				wantErr            bool
				expectedErrMessage string
			}{{
				name: "cm_duplicate_test",
				tpls: []ComponentConfigSpec{
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl1",
							TemplateRef: "cm1",
							VolumeName:  "volume1",
						},
						ConfigConstraintRef: "constraint1",
					},
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl2",
							TemplateRef: "cm1",
							VolumeName:  "volume2",
						},
						ConfigConstraintRef: "constraint1",
					},
				},
				wantErr:            true,
				expectedErrMessage: "configmap[cm1] already existed.",
			}, {
				name: "name_duplicate_test",
				tpls: []ComponentConfigSpec{
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl1",
							TemplateRef: "cm1",
							VolumeName:  "volume1",
						},
						ConfigConstraintRef: "constraint1",
					},
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl1",
							TemplateRef: "cm2",
							VolumeName:  "volume2",
						},
						ConfigConstraintRef: "constraint2",
					},
				},
				wantErr:            true,
				expectedErrMessage: "Duplicate value: map",
			}, {
				name: "volume_duplicate_test",
				tpls: []ComponentConfigSpec{
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl1",
							TemplateRef: "cm1",
							VolumeName:  "volume1",
						},
						ConfigConstraintRef: "constraint1",
					},
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl2",
							TemplateRef: "cm2",
							VolumeName:  "volume1",
						},
						ConfigConstraintRef: "constraint2",
					},
				},
				wantErr:            true,
				expectedErrMessage: "volume[volume1] already existed.",
			}, {
				name: "normal_test",
				tpls: []ComponentConfigSpec{
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl1",
							TemplateRef: "cm1",
							VolumeName:  "volume1",
						},
						ConfigConstraintRef: "constraint1",
					},
					{
						ComponentTemplateSpec: ComponentTemplateSpec{
							Name:        "tpl2",
							TemplateRef: "cm2",
							VolumeName:  "volume2",
						},
						ConfigConstraintRef: "constraint1",
					},
				},
				wantErr: false,
			}}

			for _, tt := range tests {
				clusterDef.Spec.ComponentDefs[0].ConfigSpecs = tt.tpls
				err := testCtx.CreateObj(ctx, clusterDef)
				if tt.wantErr {
					Expect(err).ShouldNot(Succeed())
					Expect(err.Error()).Should(ContainSubstring(tt.expectedErrMessage))
				} else {
					Expect(err).Should(Succeed())
				}
			}
		})
	})

	It("test mutating webhook", func() {
		clusterDef, _ := createTestClusterDefinitionObj3(clusterDefinitionName + "-mutating")
		By("test set the default value to RoleProbeTimeoutAfterPodsReady when roleProbe is not nil")
		clusterDef.Spec.ComponentDefs[0].Probes = &ClusterDefinitionProbes{
			RoleProbe: &ClusterDefinitionProbe{},
		}
		Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterDef.Name}, clusterDef)).Should(Succeed())
		Expect(clusterDef.Spec.ComponentDefs[0].Probes.RoleProbeTimeoutAfterPodsReady).Should(Equal(DefaultRoleProbeTimeoutAfterPodsReady))

		By("test set zero to RoleProbeTimeoutAfterPodsReady when roleProbe is nil")
		clusterDef.Spec.ComponentDefs[0].Probes = &ClusterDefinitionProbes{
			RoleProbeTimeoutAfterPodsReady: 60,
		}
		Expect(k8sClient.Update(ctx, clusterDef)).Should(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterDef.Name}, clusterDef)).Should(Succeed())
		Expect(clusterDef.Spec.ComponentDefs[0].Probes.RoleProbeTimeoutAfterPodsReady).Should(Equal(int32(0)))

		By("set h-scale policy type to Snapshot")
		clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy = &HorizontalScalePolicy{
			Type: HScaleDataClonePolicyFromSnapshot,
		}
		Expect(k8sClient.Update(ctx, clusterDef)).Should(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterDef.Name}, clusterDef)).Should(Succeed())
		Expect(clusterDef.Spec.ComponentDefs[0].HorizontalScalePolicy.Type).Should(Equal(HScaleDataClonePolicyCloneVolume))
	})
})

// createTestClusterDefinitionObj  other webhook_test called this function, carefully for modifying the function
func createTestClusterDefinitionObj(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  componentDefs:
  - name: replicasets
    workloadType: Stateful
    podSpec:
      containers:
      - name: nginx
        image: nginx:latest
  - name: proxy
    workloadType: Stateless
    podSpec:
      containers:
      - name: nginx
        image: nginx:latest
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}

// createTestClusterDefinitionObj2 create an invalid obj
func createTestClusterDefinitionObj2(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  componentDefs:
  - name: mysql-rafted
    workloadType: Consensus
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/apecloud-mysql-server:latest
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}

func createTestClusterDefinitionObj3(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  componentDefs:
  - name: replicasets
    logConfig:
      - name: error
        filePathPattern: /data/mysql/log/mysqld.err
      - name: slow
        filePathPattern: /data/mysql/mysqld-slow.log
    configSpecs:
      - name: mysql-tree-node-template-8.0
        templateRef: mysql-tree-node-template-8.0
        volumeName: mysql-config
    workloadType: Consensus
    consensusSpec:
      leader:
        name: leader
        accessMode: ReadWrite
      followers:
        - name: follower
          accessMode: Readonly
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/apecloud-mysql-server:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /data
            name: data
          - mountPath: /log
            name: log
          - mountPath: /data/config/mysql
            name: mysql-config
        env:
        command: ["/usr/bin/bash", "-c"]
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}

func createMultiCompClusterDefObj(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  componentDefs:
  - name: mysql-rafted
    workloadType: Stateful
    podSpec:
      containers:
      - name: mysql-raft
        command: ["/usr/bin/bash", "-c"]
  - name: mysql-proxy
    workloadType: Stateless
    podSpec:
      containers:
      - name: mysql-proxy
        command: ["/usr/bin/bash", "-c"]
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}
