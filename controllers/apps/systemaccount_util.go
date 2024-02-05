/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	componetutil "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// customizedEngine helps render jobs.
type customizedEngine struct {
	image      string
	command    []string
	args       []string
	envVarList []corev1.EnvVar
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

// func newCustomizedEngine(execConfig *appsv1alpha1.CmdExecutorConfig, dbcluster *appsv1alpha1.Cluster, compName string) *customizedEngine {
// 	return &customizedEngine{
// 		componentName: compName,
// 		image:         execConfig.Image,
// 		command:       execConfig.Command,
// 		args:          execConfig.Args,
// 		envVarList:    execConfig.Env,
// 	}
// }

// func replaceEnvsValues(clusterName string, sysAccounts *appsv1alpha1.SystemAccountSpec, placeholders map[string]string) {
// 	mergedPlaceholders := componetutil.GetEnvReplacementMapForConnCredential(clusterName)
// 	for k, v := range placeholders {
// 		mergedPlaceholders[k] = v
// 	}

// 	// replace systemAccounts.cmdExecutorConfig.env[].valueFrom.secretKeyRef.name variables
// 	cmdConfig := sysAccounts.CmdExecutorConfig
// 	if cmdConfig != nil {
// 		cmdConfig.Env = componetutil.ReplaceSecretEnvVars(mergedPlaceholders, cmdConfig.Env)
// 	}

// 	// accounts := sysAccounts.Accounts
// 	// for _, acc := range accounts {
// 	// 	if acc.ProvisionPolicy.Type == appsv1alpha1.ReferToExisting {
// 	// 		// replace systemAccounts.accounts[*].provisionPolicy.secretRef.name variables
// 	// 		secretRef := acc.ProvisionPolicy.SecretRef
// 	// 		name := componetutil.ReplaceNamedVars(mergedPlaceholders, secretRef.Name, 1, false)
// 	// 		if name != secretRef.Name {
// 	// 			secretRef.Name = name
// 	// 		}
// 	// 	}
// 	// }
// }

// // getLabelsForSecretsAndJobs constructs matching labels for secrets and jobs.
// // This is consistent with that of secrets created during cluster initialization.
// func getLabelsForSecretsAndJobs(key componentUniqueKey) client.MatchingLabels {
// 	return client.MatchingLabels{
// 		constant.AppInstanceLabelKey:    key.clusterName,
// 		constant.KBAppComponentLabelKey: key.componentName,
// 		constant.AppManagedByLabelKey:   constant.AppName,
// 	}
// }

func renderJob(jobName string, engine *customizedEngine, comp *appsv1alpha1.Component, statement []string, endpoint string) *batchv1.Job {
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
	if len(engine.getEnvs()) > 0 {
		envs = append(envs, engine.getEnvs()...)
	}

	jobContainer := corev1.Container{
		Name:            jobName,
		Image:           engine.getImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         engine.getCommand(),
		Args:            engine.getArgs(),
		Env:             envs,
	}

	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&jobContainer)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: comp.Namespace,
			Name:      jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: comp.Namespace,
					Name:      jobName},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{jobContainer},
				},
			},
		},
	}

	return job
}

func getTargetPods(ctx context.Context, r client.Client, cluster *appsv1alpha1.Cluster,
	cmpd *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component) (*corev1.PodList, error) {

	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(cluster.Name, getCompNameShort(comp))

	if cmpd.Spec.LifecycleActions == nil || cmpd.Spec.LifecycleActions.AccountProvision == nil || cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler == nil {
		return nil, fmt.Errorf("no custom handler found in component definition")
	}
	handler := cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler
	switch handler.TargetPodSelector {
	case appsv1alpha1.RoleSelector:
		roleName := ""
		for _, role := range cmpd.Spec.Roles {
			if role.Serviceable && role.Writable {
				roleName = role.Name
			}
		}
		if roleName == "" {
			return nil, fmt.Errorf("no writable role found in component definition")
		}

		labels[constant.RoleLabelKey] = roleName
		if err := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return nil, err
		}
		return podList, nil

	case appsv1alpha1.AnyReplica, appsv1alpha1.AllReplicas:
		if err := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return nil, err
		}
		if handler.TargetPodSelector == appsv1alpha1.AnyReplica {
			pod := corev1.Pod{}
			for _, p := range podList.Items {
				if p.Status.Phase == corev1.PodRunning {
					pod = p
					break
				}
			}
			return &corev1.PodList{Items: []corev1.Pod{pod}}, nil
		}
		return podList, nil
	}
	return nil, fmt.Errorf("unsupported target pod selector")
}

func getCreationStmtForAccount(userName string, passwd string, stmts string, strategy updateStrategy) []string {

	namedVars := getEnvReplacementMapForAccount(userName, passwd)

	execStmts := make([]string, 0)

	// if strategy == inPlaceUpdate && len(statements.UpdateStatement) == 0 {
	// 	// if update statement is empty, use reCreate strategy, which will drop and create the account.
	// 	strategy = reCreate
	// 	klog.Warningf("account %s in cluster %s exists, but its update statement is not set, will use %s strategy to update account.", userName, key.clusterName, strategy)
	// }

	// if strategy == inPlaceUpdate {
	// 	// use update statement
	// 	stmt := componetutil.ReplaceNamedVars(namedVars, statements.UpdateStatement, -1, true)
	// 	execStmts = append(execStmts, stmt)
	// } else {
	// 	// drop if exists + create if not exists
	// 	if len(statements.DeletionStatement) > 0 {
	// 		stmt := componetutil.ReplaceNamedVars(namedVars, statements.DeletionStatement, -1, true)
	// 		execStmts = append(execStmts, stmt)
	// 	}
	// 	stmt := componetutil.ReplaceNamedVars(namedVars, statements.CreationStatement, -1, true)
	// 	execStmts = append(execStmts, stmt)
	// }
	// secret := renderSecretWithPwd(key, userName, passwd)
	stmt := componetutil.ReplaceNamedVars(namedVars, stmts, -1, true)
	execStmts = append(execStmts, stmt)
	return execStmts
}

func getDebugMode(annotatedDebug string) bool {
	debugOn, _ := strconv.ParseBool(annotatedDebug)
	return viper.GetBool(systemAccountsDebugMode) || debugOn
}

func calibrateJobMetaAndSpec(job *batchv1.Job, cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, accountName string) error {
	debugModeOn := getDebugMode(cluster.Annotations[debugClusterAnnotationKey])
	// add label
	ml := client.MatchingLabels(constant.GetComponentWellKnownLabels(cluster.Name, comp.Name))
	ml[constant.ClusterAccountLabelKey] = accountName
	job.ObjectMeta.Labels = ml

	// if debug mode is on, jobs will retain after execution.
	if debugModeOn {
		job.Spec.TTLSecondsAfterFinished = nil
	} else {
		defaultTTLZero := (int32)(1)
		job.Spec.TTLSecondsAfterFinished = &defaultTTLZero
	}

	// build tolerations
	tolerations := cluster.Spec.Tolerations
	if len(comp.Spec.Tolerations) != 0 {
		tolerations = comp.Spec.Tolerations
	}
	// build data plane tolerations from config
	var dpTolerations []corev1.Toleration
	if val := viper.GetString(constant.CfgKeyDataPlaneTolerations); val != "" {
		if err := json.Unmarshal([]byte(val), &dpTolerations); err != nil {
			return err
		}
		tolerations = append(tolerations, dpTolerations...)
	}
	job.Spec.Template.Spec.Tolerations = tolerations
	return nil
}

// // completeExecConfig overrides the image of execConfig if version is not nil.
// func completeExecConfig(execConfig *appsv1alpha1.CmdExecutorConfig, version *appsv1alpha1.ClusterComponentVersion) {
// 	if version == nil || version.SystemAccountSpec == nil || version.SystemAccountSpec.CmdExecutorConfig == nil {
// 		return
// 	}
// 	sysAccountSpec := version.SystemAccountSpec
// 	if len(sysAccountSpec.CmdExecutorConfig.Image) > 0 {
// 		execConfig.Image = sysAccountSpec.CmdExecutorConfig.Image
// 	}

// 	// envs from sysAccountSpec will override the envs from execConfig
// 	if sysAccountSpec.CmdExecutorConfig.Env == nil {
// 		return
// 	}
// 	if len(sysAccountSpec.CmdExecutorConfig.Env) == 0 {
// 		// clean up envs
// 		execConfig.Env = nil
// 	} else {
// 		execConfig.Env = sysAccountSpec.CmdExecutorConfig.Env
// 	}
// }
