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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/yaml"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdata "github.com/apecloud/kubeblocks/test/testdata"
)

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

func TestRenderJob(t *testing.T) {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-" + randomStr
		clusterVersionName    = "clusterversion-" + randomStr
		clusterName           = "cluster-" + randomStr
		consensusCompName     = "consensus" + randomStr
	)

	mockClusterDef := func(filePath string) *dbaasv1alpha1.ClusterDefinition {
		clusterDefBytes, err := testdata.GetTestDataFileContent(filePath)
		assert.Nil(t, err)
		clusterDefYaml := fmt.Sprintf(string(clusterDefBytes), clusterDefinitionName)
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		err = yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)
		assert.Nil(t, err)
		return clusterDef
	}

	mockCluster := func(filePath string) *dbaasv1alpha1.Cluster {
		clusterBytes, err := testdata.GetTestDataFileContent(filePath)
		assert.Nil(t, err)
		clusterYaml := fmt.Sprintf(string(clusterBytes), clusterVersionName, clusterDefinitionName, clusterName,
			clusterVersionName, clusterDefinitionName, consensusCompName)
		cluster := &dbaasv1alpha1.Cluster{}
		err = yaml.Unmarshal([]byte(clusterYaml), cluster)
		assert.Nil(t, err)
		return cluster
	}

	clusterDef := mockClusterDef("consensusset/wesql_cd_sysacct.yaml")
	assert.NotNil(t, clusterDef)
	assert.NotNil(t, clusterDef.Spec.Components[0].SystemAccounts)
	cluster := mockCluster("consensusset/wesql.yaml")
	assert.NotNil(t, cluster)

	accountsSetting := clusterDef.Spec.Components[0].SystemAccounts
	replaceEnvsValues(cluster.Name, accountsSetting)
	cmdExecutorConfig := accountsSetting.CmdExecutorConfig

	engine := newCustomizedEngine(cmdExecutorConfig, cluster, consensusCompName)
	assert.NotNil(t, engine)

	compKey := componentUniqueKey{
		namespace:     cluster.Namespace,
		clusterName:   cluster.Name,
		componentName: consensusCompName,
	}

	for _, acc := range accountsSetting.Accounts {
		switch acc.ProvisionPolicy.Type {
		case dbaasv1alpha1.CreateByStmt:
			creationStmt, secrets := getCreationStmtForAccount(compKey, accountsSetting.PasswordConfig, acc)
			// make sure all variables have been replaced
			for _, stmt := range creationStmt {
				assert.False(t, strings.Contains(stmt, "$(USERNAME)"))
				assert.False(t, strings.Contains(stmt, "$(PASSWD)"))
			}
			// render job with debug mode off
			job := renderJob(engine, compKey, string(acc.Name), creationStmt, "10.0.0.1", false)
			assert.NotNil(t, job)
			assert.NotNil(t, job.Spec.TTLSecondsAfterFinished)
			assert.Equal(t, (int32)(0), *job.Spec.TTLSecondsAfterFinished)
			envList := job.Spec.Template.Spec.Containers[0].Env
			assert.GreaterOrEqual(t, len(envList), 1)
			assert.Equal(t, job.Spec.Template.Spec.Containers[0].Image, cmdExecutorConfig.Image)
			// render job with debug mode on
			job = renderJob(engine, compKey, string(acc.Name), creationStmt, "10.0.0.1", true)
			assert.NotNil(t, job)
			assert.Nil(t, job.Spec.TTLSecondsAfterFinished)
			assert.NotNil(t, secrets)
		case dbaasv1alpha1.ReferToExisting:
			assert.False(t, strings.Contains(acc.ProvisionPolicy.SecretRef.Name, "$(CONN_CREDENTIAL_SECRET_NAME)"))
		}
	}
}

func TestAccountNum(t *testing.T) {
	totalAccounts := getAllSysAccounts()
	accountNum := len(totalAccounts)
	assert.Greater(t, accountNum, 0)
	expectedMaxKBAccountType := 1 << (accountNum - 1)
	assert.Equal(t, expectedMaxKBAccountType, dbaasv1alpha1.KBAccountMAX)
}
