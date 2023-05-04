/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sethvargo/go-password/password"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	componetutil "github.com/apecloud/kubeblocks/internal/controller/component"
)

const (
	jobPrefix = "job-system-account-"
)

// SecretMapStore is a cache, recording all (key, secret) pair for accounts to be created.
type secretMapStore struct {
	cache.Store
}

// SecretMapEntry records (key, secret) pair for account to be created.
type secretMapEntry struct {
	key   string
	value *corev1.Secret
}

// customizedEngine helps render jobs.
type customizedEngine struct {
	cluster       *appsv1alpha1.Cluster
	componentName string
	image         string
	command       []string
	args          []string
	envVarList    []corev1.EnvVar
}

// SecretKeyFunc to parse out the key from a SecretMapEntry.
var secretKeyFunc = func(obj interface{}) (string, error) {
	if e, ok := obj.(*secretMapEntry); ok {
		return e.key, nil
	}
	return "", fmt.Errorf("could not find key for obj %#v", obj)
}

func newSecretMapStore() *secretMapStore {
	return &secretMapStore{cache.NewStore(secretKeyFunc)}
}

func (r *secretMapStore) addSecret(key string, value *corev1.Secret) error {
	_, exists, err := r.getSecret(key)
	if err != nil {
		return err
	}
	entry := &secretMapEntry{key: key, value: value}
	if exists {
		return r.Update(entry)
	}
	return r.Add(entry)
}

func (r *secretMapStore) getSecret(key string) (*secretMapEntry, bool, error) {
	exp, exists, err := r.GetByKey(key)
	if err != nil {
		return nil, false, err
	}
	if exists {
		return exp.(*secretMapEntry), true, nil
	}
	return nil, false, nil
}

func (r *secretMapStore) deleteSecret(key string) error {
	exp, exist, err := r.GetByKey(key)
	if err != nil {
		return err
	}
	if exist {
		return r.Delete(exp)
	}
	return nil
}

func (e *customizedEngine) getImage() string {
	return e.image
}

func (e *customizedEngine) getEnvs() []corev1.EnvVar {
	return e.envVarList
}

// getPodCommand shows how to execute the sql statement.
// for instance, mysql -h - demo-cluster-replicasets-1 -e  "create user username IDENTIFIED by 'passwd';"
func (e customizedEngine) getCommand() []string {
	return e.command
}

// getPodCommand shows how to execute the sql statement.
// for instance, mysql -h - demo-cluster-replicasets-1 -e  "create user username IDENTIFIED by 'passwd';"
func (e *customizedEngine) getArgs() []string {
	return e.args
}

func newCustomizedEngine(execConfig *appsv1alpha1.CmdExecutorConfig, dbcluster *appsv1alpha1.Cluster, compName string) *customizedEngine {
	return &customizedEngine{
		cluster:       dbcluster,
		componentName: compName,
		image:         execConfig.Image,
		command:       execConfig.Command,
		args:          execConfig.Args,
		envVarList:    execConfig.Env,
	}
}

func replaceEnvsValues(clusterName string, sysAccounts *appsv1alpha1.SystemAccountSpec) {
	namedValuesMap := componetutil.GetEnvReplacementMapForConnCredential(clusterName)
	// replace systemAccounts.cmdExecutorConfig.env[].valueFrom.secretKeyRef.name variables
	cmdConfig := sysAccounts.CmdExecutorConfig
	if cmdConfig != nil {
		cmdConfig.Env = componetutil.ReplaceSecretEnvVars(namedValuesMap, cmdConfig.Env)
	}

	accounts := sysAccounts.Accounts
	for _, acc := range accounts {
		if acc.ProvisionPolicy.Type == appsv1alpha1.ReferToExisting {
			// replace systemAccounts.accounts[*].provisionPolicy.secretRef.name variables
			secretRef := acc.ProvisionPolicy.SecretRef
			name := componetutil.ReplaceNamedVars(namedValuesMap, secretRef.Name, 1, false)
			if name != secretRef.Name {
				secretRef.Name = name
			}
		}
	}
}

// getLabelsForSecretsAndJobs construct matching labels for secrets and jobs.
// This is consistent with that of secrets created during cluster initialization.
func getLabelsForSecretsAndJobs(key componentUniqueKey) client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    key.clusterName,
		constant.KBAppComponentLabelKey: key.componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
}

func renderJob(engine *customizedEngine, key componentUniqueKey, statement []string, endpoint string) *batchv1.Job {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	jobName := jobPrefix + key.clusterName + "-" + randomStr

	// inject one more system env variables
	statementEnv := corev1.EnvVar{
		Name:  kbAccountStmtEnvName,
		Value: strings.Join(statement, " "),
	}
	endpointEnv := corev1.EnvVar{
		Name:  kbAccountEndPointEnvName,
		Value: endpoint,
	}
	// place statements and endpoints before user defined envs.
	envs := make([]corev1.EnvVar, 0, 2+len(engine.getEnvs()))
	envs = append(envs, statementEnv, endpointEnv)
	envs = append(envs, engine.getEnvs()...)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.namespace,
			Name:      jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.namespace,
					Name:      jobName},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            randomStr,
							Image:           engine.getImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         engine.getCommand(),
							Args:            engine.getArgs(),
							Env:             envs,
						},
					},
				},
			},
		},
	}

	return job
}

func renderSecretWithPwd(key componentUniqueKey, username, passwd string) *corev1.Secret {
	secretData := map[string][]byte{}
	secretData[constant.AccountNameForSecret] = []byte(username)
	secretData[constant.AccountPasswdForSecret] = []byte(passwd)

	ml := getLabelsForSecretsAndJobs(key)
	ml[constant.ClusterAccountLabelKey] = username
	return renderSecret(key, username, ml, secretData)
}

func renderSecretByCopy(key componentUniqueKey, username string, fromSecret *corev1.Secret) *corev1.Secret {
	ml := getLabelsForSecretsAndJobs(key)
	ml[constant.ClusterAccountLabelKey] = username
	return renderSecret(key, username, ml, fromSecret.Data)
}

func renderSecret(key componentUniqueKey, username string, labels client.MatchingLabels, data map[string][]byte) *corev1.Secret {
	// secret labels and secret fianlizers should be consistent with that of Cluster secret created by Cluster Controller.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  key.namespace,
			Name:       strings.Join([]string{key.clusterName, key.componentName, username}, "-"),
			Labels:     labels,
			Finalizers: []string{dbClusterFinalizerName},
		},
		Data: data,
	}
	return secret
}

func retrieveEndpoints(scope appsv1alpha1.ProvisionScope, svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints) []string {
	// parse endpoints
	endpoints := make([]string, 0)
	if scope == appsv1alpha1.AnyPods {
		for _, ss := range svcEP.Subsets {
			for _, add := range ss.Addresses {
				endpoints = append(endpoints, add.IP)
				break
			}
		}
	} else {
		for _, ss := range headlessEP.Subsets {
			for _, add := range ss.Addresses {
				endpoints = append(endpoints, add.IP)
			}
		}
	}
	return endpoints
}

func getAccountFacts(secrets *corev1.SecretList, jobs *batchv1.JobList) (detectedFacts appsv1alpha1.KBAccountType) {
	detectedFacts = appsv1alpha1.KBAccountInvalid
	// parse account name from secret's label
	for _, secret := range secrets.Items {
		if accountName, exists := secret.ObjectMeta.Labels[constant.ClusterAccountLabelKey]; exists {
			updateFacts(appsv1alpha1.AccountName(accountName), &detectedFacts)
		}
	}
	// parse account name from job's label
	for _, job := range jobs.Items {
		if accountName, exists := job.ObjectMeta.Labels[constant.ClusterAccountLabelKey]; exists {
			updateFacts(appsv1alpha1.AccountName(accountName), &detectedFacts)
		}
	}
	return
}

func updateFacts(accountName appsv1alpha1.AccountName, detectedFacts *appsv1alpha1.KBAccountType) {
	switch accountName {
	case appsv1alpha1.AdminAccount:
		*detectedFacts |= appsv1alpha1.KBAccountAdmin
	case appsv1alpha1.DataprotectionAccount:
		*detectedFacts |= appsv1alpha1.KBAccountDataprotection
	case appsv1alpha1.ProbeAccount:
		*detectedFacts |= appsv1alpha1.KBAccountProbe
	case appsv1alpha1.MonitorAccount:
		*detectedFacts |= appsv1alpha1.KBAccountMonitor
	case appsv1alpha1.ReplicatorAccount:
		*detectedFacts |= appsv1alpha1.KBAccountReplicator
	}
}

func concatSecretName(key componentUniqueKey, username string) string {
	return fmt.Sprintf("%s-%s-%s-%s", key.namespace, key.clusterName, key.componentName, username)
}

func getCreationStmtForAccount(key componentUniqueKey, passConfig appsv1alpha1.PasswordConfig,
	accountConfig appsv1alpha1.SystemAccountConfig, strategy updateStrategy) ([]string, *corev1.Secret) {
	// generated password with mixedcases = true
	passwd, _ := password.Generate((int)(passConfig.Length), (int)(passConfig.NumDigits), (int)(passConfig.NumSymbols), false, false)
	// refine password to upper or lower cases w.r.t configuration
	switch passConfig.LetterCase {
	case appsv1alpha1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case appsv1alpha1.LowerCases:
		passwd = strings.ToLower(passwd)
	}

	userName := (string)(accountConfig.Name)

	namedVars := getEnvReplacementMapForAccount(userName, passwd)

	execStmts := make([]string, 0)

	statements := accountConfig.ProvisionPolicy.Statements
	if strategy == inPlaceUpdate {
		// use update statement
		stmt := componetutil.ReplaceNamedVars(namedVars, statements.UpdateStatement, -1, true)
		execStmts = append(execStmts, stmt)
	} else {
		// drop if exists + create if not exists
		if len(statements.DeletionStatement) > 0 {
			stmt := componetutil.ReplaceNamedVars(namedVars, statements.DeletionStatement, -1, true)
			execStmts = append(execStmts, stmt)
		}
		stmt := componetutil.ReplaceNamedVars(namedVars, statements.CreationStatement, -1, true)
		execStmts = append(execStmts, stmt)
	}
	secret := renderSecretWithPwd(key, userName, passwd)
	return execStmts, secret
}

func getAllSysAccounts() []appsv1alpha1.AccountName {
	return []appsv1alpha1.AccountName{
		appsv1alpha1.AdminAccount,
		appsv1alpha1.DataprotectionAccount,
		appsv1alpha1.ProbeAccount,
		appsv1alpha1.MonitorAccount,
		appsv1alpha1.ReplicatorAccount,
	}
}

func getDefaultAccounts() appsv1alpha1.KBAccountType {
	accountID := appsv1alpha1.KBAccountInvalid
	for _, name := range getAllSysAccounts() {
		accountID |= name.GetAccountID()
	}
	return accountID
}

func getDebugMode(annotatedDebug string) bool {
	debugOn, _ := strconv.ParseBool(annotatedDebug)
	return viper.GetBool(systemAccountsDebugMode) || debugOn
}

func calibrateJobMetaAndSpec(job *batchv1.Job, cluster *appsv1alpha1.Cluster, compKey componentUniqueKey, account appsv1alpha1.AccountName) {
	debugModeOn := getDebugMode(cluster.Annotations[debugClusterAnnotationKey])
	// add label
	ml := getLabelsForSecretsAndJobs(compKey)
	ml[constant.ClusterAccountLabelKey] = (string)(account)
	job.ObjectMeta.Labels = ml

	// if debug mode is on, jobs will retain after execution.
	if debugModeOn {
		job.Spec.TTLSecondsAfterFinished = nil
	} else {
		defaultTTLZero := (int32)(0)
		job.Spec.TTLSecondsAfterFinished = &defaultTTLZero
	}

	// add toleration
	tolerations := cluster.Spec.Tolerations
	clusterComp := cluster.Spec.GetComponentByName(compKey.componentName)
	if clusterComp != nil {
		if len(clusterComp.Tolerations) != 0 {
			tolerations = clusterComp.Tolerations
		}
	}
	// add built-in toleration
	tolerations = componetutil.PatchBuiltInToleration(tolerations)
	job.Spec.Template.Spec.Tolerations = tolerations
}
