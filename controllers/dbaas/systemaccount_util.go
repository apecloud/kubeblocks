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
	"fmt"
	"strings"

	"github.com/sethvargo/go-password/password"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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

// SystemAccountExpectation records accounts to be created.
type systemAccountExpectation struct {
	toCreate dbaasv1alpha1.KBAccountType
	key      string
}

// customizedEngine helps render jobs.
type customizedEngine struct {
	cluster       *dbaasv1alpha1.Cluster
	componentName string
	image         string
	command       []string
	args          []string
	envVarList    []corev1.EnvVar
}

// SystemAccountExpectationsManager is a cache, recording a key-expectation pair for each cluster
// In each expectation we records accounts to create, and to delete.
// Requirements are collected from objects:
// - ClusterDefintion: kbprob, kbreplicator
// - Cluster: kbmonitoring
// - BackupPolicy: kbdataprotecion
// - Others: kbadmin
type systemAccountExpectationsManager struct {
	cache.Store
}

// SecretKeyFunc to parse out the key from a SecretMapEntry.
var secretKeyFunc = func(obj interface{}) (string, error) {
	if e, ok := obj.(*secretMapEntry); ok {
		return e.key, nil
	}
	return "", fmt.Errorf("could not find key for obj %#v", obj)
}

// ExpKeyFunc to parse out the key from an expecationl.
var expKeyFunc = func(obj interface{}) (string, error) {
	if e, ok := obj.(*systemAccountExpectation); ok {
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

func (e *systemAccountExpectation) set(add dbaasv1alpha1.KBAccountType) {
	e.toCreate |= add
}

func (e *systemAccountExpectation) getExpectation() dbaasv1alpha1.KBAccountType {
	return e.toCreate
}

func newExpectationsManager() *systemAccountExpectationsManager {
	return &systemAccountExpectationsManager{cache.NewStore(expKeyFunc)}
}

func (r *systemAccountExpectationsManager) createExpectation(key string) (*systemAccountExpectation, error) {
	exp, exists, err := r.getExpectation(key)
	if err != nil {
		return nil, err
	}

	if exists {
		return exp, nil
	}

	exp = &systemAccountExpectation{toCreate: dbaasv1alpha1.KBAccountInvalid, key: key}
	err = r.Add(exp)
	return exp, err
}

func (r *systemAccountExpectationsManager) getExpectation(cluster string) (*systemAccountExpectation, bool, error) {
	exp, exists, err := r.GetByKey(cluster)
	if err != nil {
		return nil, false, err
	}
	if exists {
		return exp.(*systemAccountExpectation), true, nil
	}
	return nil, false, nil
}

func (r *systemAccountExpectationsManager) deleteExpectation(cluster string) error {
	exp, exists, err := r.GetByKey(cluster)
	if err != nil {
		return err
	}

	if exists {
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

func newCustomizedEngine(execConfig *dbaasv1alpha1.CmdExecutorConfig, dbcluster *dbaasv1alpha1.Cluster, compName string) *customizedEngine {
	return &customizedEngine{
		cluster:       dbcluster,
		componentName: compName,
		image:         execConfig.Image,
		command:       execConfig.Command,
		args:          execConfig.Args,
		envVarList:    execConfig.Env,
	}
}

func replaceNamedVars(namedValues map[string]string, needle string, limits int, matchAll bool) string {
	for k, v := range namedValues {
		r := strings.Replace(needle, k, v, limits)
		// early termination on matching, when matchAll = false
		if r != needle && !matchAll {
			return r
		}
		needle = r
	}
	return needle
}

func replaceEnvsValues(clusterName string, sysAccounts *dbaasv1alpha1.SystemAccountSpec) {
	namedValues := getEnvReplacementMapForConnCrential(clusterName)
	// replace systemAccounts.cmdExecutorConfig.env[].valueFrom.secretKeyRef.name variables
	cmdConfig := sysAccounts.CmdExecutorConfig
	if cmdConfig != nil {
		for _, e := range cmdConfig.Env {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
				continue
			}
			secretRef := e.ValueFrom.SecretKeyRef
			name := replaceNamedVars(namedValues, secretRef.Name, 1, false)
			if name != secretRef.Name {
				secretRef.Name = name
			}
		}
	}

	accounts := sysAccounts.Accounts
	for _, acc := range accounts {
		if acc.ProvisionPolicy.Type == dbaasv1alpha1.ReferToExisting {
			// replace systemAccounts.accounts[*].provisionPolciy.secretRef.name variables
			secretRef := acc.ProvisionPolicy.SecretRef
			name := replaceNamedVars(namedValues, secretRef.Name, 1, false)
			if name != secretRef.Name {
				secretRef.Name = name
			}
		}
	}
}

// getLabelsForSecretsAndJobs construct matching labels for secrets and jobs.
// This is consistent with that of secrets created during cluster initialization.
func getLabelsForSecretsAndJobs(clusterName, clusterDefType, clusterDefName, compName string) client.MatchingLabels {
	// get account facts, i.e., secrets created
	return client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:  clusterName,
		intctrlutil.AppNameLabelKey:      fmt.Sprintf("%s-%s", clusterDefType, clusterDefName),
		intctrlutil.AppComponentLabelKey: compName,
	}
}

func renderJob(engine *customizedEngine, namespace, clusterName, clusterDefType, clusterDefName, compName, username string,
	statement []string, endpoint string) *batchv1.Job {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	jobName := clusterName + "-" + randomStr

	// label
	ml := getLabelsForSecretsAndJobs(clusterName, clusterDefType, clusterDefName, compName)
	ml[clusterAccountLabelKey] = username

	// set job ttl to 1 seconds
	var defaultTTLSeconds int32 = 1

	// inject one more system env variables
	statementEnv := corev1.EnvVar{
		Name:  kbAccountStmtEnvName,
		Value: strings.Join(statement, ";"),
	}
	endpointEnv := corev1.EnvVar{
		Name:  kbAccountEndPointEnvName,
		Value: endpoint,
	}

	envs := append(engine.getEnvs(), statementEnv, endpointEnv)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      jobName,
			Labels:    ml,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &defaultTTLSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
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

func renderSecretWithPwd(namespace, clusterName, clusterDefType, clusterDefName, compName, username, passwd string) *corev1.Secret {
	secretData := map[string][]byte{}
	secretData[accountNameForSecret] = []byte(username)
	secretData[accountPasswdForSecret] = []byte(passwd)

	ml := getLabelsForSecretsAndJobs(clusterName, clusterDefType, clusterDefName, compName)
	ml[clusterAccountLabelKey] = username
	return renderSecret(namespace, clusterName, compName, username, ml, secretData)
}

func renderSecretByCopy(namespace, clusterName, clusterDefType, clusterDefName, compName, username string, fromSecret *corev1.Secret) *corev1.Secret {
	ml := getLabelsForSecretsAndJobs(clusterName, clusterDefType, clusterDefName, compName)
	ml[clusterAccountLabelKey] = username
	return renderSecret(namespace, clusterName, compName, username, ml, fromSecret.Data)
}

func renderSecret(namespace, clusterName, compName, username string, labels client.MatchingLabels, data map[string][]byte) *corev1.Secret {
	// secret labels and secret fianlizers should be consistent with that of Cluster secret created by Cluster Controller.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       strings.Join([]string{clusterName, compName, username}, "-"),
			Labels:     labels,
			Finalizers: []string{dbClusterFinalizerName},
		},
		Data: data,
	}
	return secret
}

func retrieveEndpoints(scope dbaasv1alpha1.ProvisionScope,
	svcEP *corev1.Endpoints, headlessEP *corev1.Endpoints) []string {
	// parse endpoints
	endpoints := make([]string, 0)
	if scope == dbaasv1alpha1.AnyPods {
		endpoints = append(endpoints, svcEP.Subsets[0].Addresses[0].IP)
	} else {
		for _, ss := range headlessEP.Subsets {
			for _, add := range ss.Addresses {
				endpoints = append(endpoints, add.IP)
			}
		}
	}
	return endpoints
}

func getAccountFacts(secrets *corev1.SecretList, jobs *batchv1.JobList) (detectedFacts dbaasv1alpha1.KBAccountType) {
	// parse account name from secret's label
	for _, secret := range secrets.Items {
		if accountName, exists := secret.ObjectMeta.Labels[clusterAccountLabelKey]; exists {
			updateFacts(dbaasv1alpha1.AccountName(accountName), &detectedFacts)
		}
	}
	// parse account name from job's label
	for _, job := range jobs.Items {
		if accountName, exists := job.ObjectMeta.Labels[clusterAccountLabelKey]; exists {
			updateFacts(dbaasv1alpha1.AccountName(accountName), &detectedFacts)
		}
	}
	return
}

func updateFacts(accountName dbaasv1alpha1.AccountName, detectedFacts *dbaasv1alpha1.KBAccountType) {
	switch accountName {
	case dbaasv1alpha1.AdminAccount:
		*detectedFacts |= dbaasv1alpha1.KBAccountAdmin
	case dbaasv1alpha1.DataprotectionAccount:
		*detectedFacts |= dbaasv1alpha1.KBAccountDataprotection
	case dbaasv1alpha1.ProbeAccount:
		*detectedFacts |= dbaasv1alpha1.KBAccountProbe
	case dbaasv1alpha1.MonitorAccount:
		*detectedFacts |= dbaasv1alpha1.KBAccountMonitor
	case dbaasv1alpha1.ReplicatorAccount:
		*detectedFacts |= dbaasv1alpha1.KBAccountReplicator
	}
}

func concatSecretName(ns, clusterName, component string, username string) string {
	return fmt.Sprintf("%s-%s-%s-%s", ns, clusterName, component, username)
}

func expectationKey(ns, clusterName, component string) string {
	return fmt.Sprintf("%s-%s-%s", ns, clusterName, component)
}

// getClusterAndEngineType infers cluster type and engine type we supported at the moment.
func getEngineType(clusterDefType string, compDef dbaasv1alpha1.ClusterDefinitionComponent) string {
	if len(compDef.CharacterType) > 0 {
		return compDef.CharacterType
	}

	switch clusterDefType {
	// clusterDefType define well known cluster types. could be one of
	// [state.redis, mq.mqtt, mq.kafka, state.mysql-8, state.mysql-5.7, state.mysql-5.6, state-mongodb]
	case "state.mysql-8", "state.mysql-5.7", "state.mysql-5.6":
		return kMysql
	default:
		return ""
	}
}

func getCreationStmtForAccount(namespace, clusterName, clusterDefType, clusterDefName, compName string, passConfig dbaasv1alpha1.PasswordConfig,
	accountConfig dbaasv1alpha1.SystemAccountConfig) ([]string, *corev1.Secret) {
	// generated password with mixedcases = true
	passwd, _ := password.Generate((int)(passConfig.Length), (int)(passConfig.NumDigits), (int)(passConfig.NumSymbols), false, false)
	// refine pasword to upper or lower cases w.r.t configuration
	switch passConfig.LetterCase {
	case dbaasv1alpha1.UpperCases:
		passwd = strings.ToUpper(passwd)
	case dbaasv1alpha1.LowerCases:
		passwd = strings.ToLower(passwd)
	}

	userName := (string)(accountConfig.Name)

	namedVars := getEnvReplacementMapForAccount(userName, passwd)

	creationStmt := make([]string, 0)
	// drop if exists + create if not exists
	statements := accountConfig.ProvisionPolicy.Statements

	stmt := replaceNamedVars(namedVars, statements.DeletionStatement, -1, true)
	creationStmt = append(creationStmt, stmt)

	stmt = replaceNamedVars(namedVars, statements.CreationStatement, -1, true)
	creationStmt = append(creationStmt, stmt)

	secret := renderSecretWithPwd(namespace, clusterName, clusterDefType, clusterDefName, compName, userName, passwd)
	return creationStmt, secret
}
