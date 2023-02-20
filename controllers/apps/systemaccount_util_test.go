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
	"math/rand"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

func mockSystemAccountsSpec() *appsv1alpha1.SystemAccountSpec {
	var (
		mysqlClientImage = "docker.io/mysql:8.0.30"
		mysqlCmdConfig   = appsv1alpha1.CmdExecutorConfig{
			Image:   mysqlClientImage,
			Command: []string{"mysql"},
			Args:    []string{"-h$(KB_ACCOUNT_ENDPOINT)", "-e $(KB_ACCOUNT_STATEMENT)"},
		}
		pwdConfig = appsv1alpha1.PasswordConfig{
			Length:     10,
			NumDigits:  5,
			NumSymbols: 0,
		}
	)

	spec := &appsv1alpha1.SystemAccountSpec{
		CmdExecutorConfig: &mysqlCmdConfig,
		PasswordConfig:    pwdConfig,
		Accounts:          []appsv1alpha1.SystemAccountConfig{},
	}
	var account appsv1alpha1.SystemAccountConfig
	var scope appsv1alpha1.ProvisionScope
	for _, name := range getAllSysAccounts() {
		randomToss := rand.Intn(10)
		if randomToss%2 == 0 {
			scope = appsv1alpha1.AnyPods
		} else {
			scope = appsv1alpha1.AllPods
		}

		if randomToss%3 == 0 {
			account = mockCreateByRefSystemAccount(name, scope)
		} else {
			account = mockCreateByStmtSystemAccount(name)
		}
		spec.Accounts = append(spec.Accounts, account)
	}
	return spec
}

func mockCreateByStmtSystemAccount(name appsv1alpha1.AccountName) appsv1alpha1.SystemAccountConfig {
	return appsv1alpha1.SystemAccountConfig{
		Name: name,
		ProvisionPolicy: appsv1alpha1.ProvisionPolicy{
			Type: appsv1alpha1.CreateByStmt,
			Statements: &appsv1alpha1.ProvisionStatements{
				CreationStatement: "CREATE USER IF NOT EXISTS $(USERNAME) IDENTIFIED BY \"$(PASSWD)\";",
				DeletionStatement: "DROP USER IF EXISTS $(USERNAME);",
			},
		},
	}
}

func mockCreateByRefSystemAccount(name appsv1alpha1.AccountName, scope appsv1alpha1.ProvisionScope) appsv1alpha1.SystemAccountConfig {
	return appsv1alpha1.SystemAccountConfig{
		Name: name,
		ProvisionPolicy: appsv1alpha1.ProvisionPolicy{
			Type:  appsv1alpha1.ReferToExisting,
			Scope: scope,
			SecretRef: &appsv1alpha1.ProvisionSecretRef{
				Namespace: testCtx.DefaultNamespace,
				Name:      "$(CONN_CREDENTIAL_SECRET_NAME)",
			},
		},
	}
}

func TestUpdateFacts(t *testing.T) {
	type testCase struct {
		// accounts
		accounts []appsv1alpha1.AccountName
		// expectation
		expect appsv1alpha1.KBAccountType
	}
	testCases := []testCase{
		{
			accounts: []appsv1alpha1.AccountName{appsv1alpha1.AdminAccount},
			expect:   appsv1alpha1.KBAccountAdmin,
		},
		{
			accounts: []appsv1alpha1.AccountName{appsv1alpha1.AdminAccount, appsv1alpha1.DataprotectionAccount},
			expect:   appsv1alpha1.KBAccountAdmin | appsv1alpha1.KBAccountDataprotection,
		},
		{
			accounts: []appsv1alpha1.AccountName{appsv1alpha1.AdminAccount, appsv1alpha1.DataprotectionAccount, appsv1alpha1.ProbeAccount},
			expect:   appsv1alpha1.KBAccountAdmin | appsv1alpha1.KBAccountDataprotection | appsv1alpha1.KBAccountProbe,
		},
		{
			accounts: []appsv1alpha1.AccountName{appsv1alpha1.AdminAccount, appsv1alpha1.DataprotectionAccount, appsv1alpha1.ProbeAccount, appsv1alpha1.MonitorAccount},
			expect:   appsv1alpha1.KBAccountAdmin | appsv1alpha1.KBAccountDataprotection | appsv1alpha1.KBAccountProbe | appsv1alpha1.KBAccountMonitor,
		},
		{
			accounts: []appsv1alpha1.AccountName{appsv1alpha1.AdminAccount, appsv1alpha1.DataprotectionAccount, appsv1alpha1.ProbeAccount, appsv1alpha1.MonitorAccount, appsv1alpha1.ReplicatorAccount},
			expect:   appsv1alpha1.KBAccountAdmin | appsv1alpha1.KBAccountDataprotection | appsv1alpha1.KBAccountProbe | appsv1alpha1.KBAccountMonitor | appsv1alpha1.KBAccountReplicator,
		},
	}

	var facts appsv1alpha1.KBAccountType
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
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterNamePrefix  = "test-cluster"
		mysqlCompType      = "replicasets"
		mysqlCompName      = "mysql"
	)

	systemAccount := mockSystemAccountsSpec()
	clusterDef := testapps.NewClusterDefFactory(clusterDefName).
		AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
		AddSystemAccountSpec(systemAccount).
		GetObject()
	assert.NotNil(t, clusterDef)
	assert.NotNil(t, clusterDef.Spec.ComponentDefs[0].SystemAccounts)

	cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDef.Name, clusterVersionName).
		AddComponent(mysqlCompType, mysqlCompName).GetObject()
	assert.NotNil(t, cluster)

	accountsSetting := clusterDef.Spec.ComponentDefs[0].SystemAccounts
	replaceEnvsValues(cluster.Name, accountsSetting)
	cmdExecutorConfig := accountsSetting.CmdExecutorConfig

	engine := newCustomizedEngine(cmdExecutorConfig, cluster, mysqlCompName)
	assert.NotNil(t, engine)

	compKey := componentUniqueKey{
		namespace:     cluster.Namespace,
		clusterName:   cluster.Name,
		componentName: mysqlCompName,
	}

	for _, acc := range accountsSetting.Accounts {
		switch acc.ProvisionPolicy.Type {
		case appsv1alpha1.CreateByStmt:
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
		case appsv1alpha1.ReferToExisting:
			assert.False(t, strings.Contains(acc.ProvisionPolicy.SecretRef.Name, constant.ConnCredentialPlaceHolder))
		}
	}
}

func TestAccountNum(t *testing.T) {
	totalAccounts := getAllSysAccounts()
	accountNum := len(totalAccounts)
	assert.Greater(t, accountNum, 0)
	expectedMaxKBAccountType := 1 << (accountNum - 1)
	assert.Equal(t, expectedMaxKBAccountType, appsv1alpha1.KBAccountMAX)
}

func TestAccountDebugMode(t *testing.T) {
	type testCase struct {
		viperEnvOn       bool
		annotatedStrings []string
		expectedR        bool
	}

	trueStrings := []string{"1", "t", "T", "TRUE", "true", "True"}            // should be parsed to true
	falseStrings := []string{"0", "f", "F", "FALSE", "false", "False"}        // should be parsed to false
	randomString := []string{"", "badCase", "invalidSettings", "TTT", "test"} // should be parsed to false

	testCases := []testCase{
		{
			viperEnvOn:       false,
			annotatedStrings: falseStrings,
			expectedR:        false,
		},
		{
			viperEnvOn:       false,
			annotatedStrings: trueStrings,
			expectedR:        true,
		},
		{
			viperEnvOn:       true,
			annotatedStrings: falseStrings,
			expectedR:        true,
		},
		{
			viperEnvOn:       true,
			annotatedStrings: trueStrings,
			expectedR:        true,
		},
		{
			viperEnvOn:       false,
			annotatedStrings: randomString,
			expectedR:        false,
		},
	}

	for _, test := range testCases {
		if test.viperEnvOn {
			viper.Set(systemAccountsDebugMode, true)
		} else {
			viper.Set(systemAccountsDebugMode, false)
		}

		for _, annotation := range test.annotatedStrings {
			debugOn := getDebugMode(annotation)
			assert.Equal(t, test.expectedR, debugOn)
		}
	}
}
