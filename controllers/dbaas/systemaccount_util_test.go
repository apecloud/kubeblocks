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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

const (
	leader   = "leader"
	follower = "follower"
)

func mockContainer() corev1.Container {
	container := corev1.Container{
		Name:            "test",
		Image:           "busybox",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sleep 5d"},
	}
	return container
}

func mockClusterDefinition(clusterDefName string, clusterEngineType string, typeName string) *dbaasv1alpha1.ClusterDefinition {
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
						Accounts: []dbaasv1alpha1.SystemAccountConfig{
							{
								Name: dbaasv1alpha1.AdminAccount,
								ProvisionPolicy: dbaasv1alpha1.ProvisionPolicy{
									Type: dbaasv1alpha1.CreateByStmt,
									Statements: &dbaasv1alpha1.ProvisionStatements{
										CreationStatement: `CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY "$(PASSWD)"; GRANT ALL PRIVILEGES ON *.* TO $(USERNAME);`,
										DeletionStatement: `DROP USER IF EXISTS $(USERNAME);`},
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
func mockCluster(clusterDefName, appVerName, typeName, clusterName string, replicas int32) *dbaasv1alpha1.Cluster {
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

func privateSetup() (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.Cluster, error) {
	var (
		clusterDefName     = "myclusterdef"
		ClusterVersionName = "myClusterVersion"
		clusterType        = " state.mysql"
		typeName           = "mycomponent"
		clusterName        = "mycluster"
	)
	clusterDef := mockClusterDefinition(clusterDefName, clusterType, typeName)
	cluster := mockCluster(clusterDefName, ClusterVersionName, typeName, clusterName, 3)
	return clusterDef, cluster, nil
}

func TestUpdateFacts(t *testing.T) {
	type testCase struct {
		// accounts
		accounts []dbaasv1alpha1.AccountName
		// expectation
		expect dbaasv1alpha1.KBAccountType
	}
	testCases := []testCase{
		{
			accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount},
			expect:   dbaasv1alpha1.KBAccountAdmin,
		},
		{
			accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.DataprotectionAccount},
			expect:   dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection,
		},
		{
			accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.DataprotectionAccount, dbaasv1alpha1.ProbeAccount},
			expect:   dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection | dbaasv1alpha1.KBAccountProbe,
		},
		{
			accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.DataprotectionAccount, dbaasv1alpha1.ProbeAccount, dbaasv1alpha1.MonitorAccount},
			expect:   dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection | dbaasv1alpha1.KBAccountProbe | dbaasv1alpha1.KBAccountMonitor,
		},
		{
			accounts: []dbaasv1alpha1.AccountName{dbaasv1alpha1.AdminAccount, dbaasv1alpha1.DataprotectionAccount, dbaasv1alpha1.ProbeAccount, dbaasv1alpha1.MonitorAccount, dbaasv1alpha1.ReplicatorAccount},
			expect:   dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection | dbaasv1alpha1.KBAccountProbe | dbaasv1alpha1.KBAccountMonitor | dbaasv1alpha1.KBAccountReplicator,
		},
	}

	var facts dbaasv1alpha1.KBAccountType
	for _, test := range testCases {
		facts = 0
		for _, acc := range test.accounts {
			updateFacts(acc, &facts)
		}
		assert.Equal(t, test.expect, facts)
	}
}

func TestExepectation(t *testing.T) {
	type accountExpect struct {
		toCreate dbaasv1alpha1.KBAccountType
	}
	// prepare settings
	settings := map[string]accountExpect{
		"minimal": {
			toCreate: dbaasv1alpha1.KBAccountAdmin,
		},
		"enableProbe": {
			toCreate: dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountProbe,
		},

		"createBackupPolicy": {
			toCreate: dbaasv1alpha1.KBAccountAdmin | dbaasv1alpha1.KBAccountDataprotection,
		},
	}
	mgr := newExpectationsManager()
	assert.NotNil(t, mgr)
	for key, value := range settings {
		expect, exist, _ := mgr.getExpectation(key)
		assert.False(t, exist)
		assert.Nil(t, expect)

		expect, _ = mgr.createExpectation(key)
		assert.NotNil(t, expect)
		expect.set(value.toCreate)
		assert.Equal(t, expect.getExpectation(), value.toCreate)
	}
	for key := range settings {
		_, exist, _ := mgr.getExpectation(key)
		assert.True(t, exist)
		err := mgr.deleteExpectation(key)
		assert.Nil(t, err)
	}
	assert.Equal(t, len(mgr.ListKeys()), 0)
}

func TestRenderJob(t *testing.T) {
	// for simplicity, create cluster and cluster definition from template and do some mutations.
	clusterDef, cluster, _ := privateSetup()
	cmdExecutorConfig := &dbaasv1alpha1.CmdExecutorConfig{
		Image:   "mysql-8.0.30",
		Command: []string{"mysql", "-e", "$(KB_ACCOUNT_STATEMENT)"},
	}

	engine := newCustomizedEngine(cmdExecutorConfig, cluster, cluster.Spec.Components[0].Name)
	accountsSetting := clusterDef.Spec.Components[0].SystemAccounts
	for _, acc := range accountsSetting.Accounts {
		creationStmt, secrets := getCreationStmtForAccount(cluster.Namespace, cluster.Name, clusterDef.Spec.Type, clusterDef.Name,
			cluster.Spec.Components[0].Name, accountsSetting.PasswordConfig, acc)
		assert.NotNil(t, secrets)

		for _, stmt := range creationStmt {
			assert.False(t, strings.Contains(stmt, "$(USERNAME)"))
			assert.False(t, strings.Contains(stmt, "$(PASSWD)"))
		}

		job := renderJob(engine, cluster.Namespace, cluster.Name, clusterDef.Spec.Type,
			clusterDef.Name, cluster.Spec.Components[0].Name, string(acc.Name), creationStmt, "10.0.0.1")
		assert.NotNil(t, job)
		envList := job.Spec.Template.Spec.Containers[0].Env
		assert.GreaterOrEqual(t, len(envList), 1)
		assert.Equal(t, job.Spec.Template.Spec.Containers[0].Image, cmdExecutorConfig.Image)
	}
}
