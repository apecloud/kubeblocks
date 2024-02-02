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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	componetutil "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func renderJob(jobName string, action *appsv1alpha1.Action, compKey *compUniqueKey, statement []string, pod *corev1.Pod) *batchv1.Job {
	// place statements and endpoints before user defined envs.
	envs := make([]corev1.EnvVar, 0)
	// inject envs from pod.container[0]
	podsEnvs := pod.Spec.Containers[0].Env
	for _, env := range podsEnvs {
		// ignore envs start with KB_
		if strings.HasPrefix(env.Name, "KB_") {
			continue
		}
		envs = append(envs, env)
	}
	// inject one more system env variables
	statementEnv := corev1.EnvVar{
		Name:  kbAccountStmtEnvName,
		Value: strings.Join(statement, " "),
	}
	endpointEnv := corev1.EnvVar{
		Name:  kbAccountEndPointEnvName,
		Value: pod.Status.PodIP,
	}
	envs = append(envs, statementEnv, endpointEnv)
	if len(action.Env) > 0 {
		envs = append(envs, action.Env...)
	}

	jobContainer := corev1.Container{
		Name:            jobName,
		Image:           action.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         action.Exec.Command,
		Args:            action.Exec.Args,
		Env:             envs,
	}

	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&jobContainer)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: compKey.namespace,
			Name:      jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: compKey.namespace,
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

func getTargetPods(ctx context.Context, r client.Client, action *appsv1alpha1.Action, roles []appsv1alpha1.ReplicaRole, compKey *compUniqueKey) (*corev1.PodList, error) {

	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(compKey.clusterName, compKey.compName)

	switch action.TargetPodSelector {
	case appsv1alpha1.RoleSelector:
		if roles == nil {
			return nil, fmt.Errorf("no roles found in component definition")
		}
		roleName := ""
		for _, role := range roles {
			if role.Serviceable && role.Writable {
				roleName = role.Name
			}
		}
		if roleName == "" {
			return nil, fmt.Errorf("no writable role found in component definition")
		}

		labels[constant.RoleLabelKey] = roleName
		if err := r.List(ctx, podList, client.InNamespace(compKey.namespace), client.MatchingLabels(labels)); err != nil {
			return nil, err
		}
		return podList, nil
	case appsv1alpha1.AnyReplica, appsv1alpha1.AllReplicas:
		if err := r.List(ctx, podList, client.InNamespace(compKey.namespace), client.MatchingLabels(labels)); err != nil {
			return nil, err
		}
		if action.TargetPodSelector == appsv1alpha1.AllReplicas {
			return podList, nil
		}

		for _, p := range podList.Items {
			if p.Status.Phase == corev1.PodRunning {
				return &corev1.PodList{Items: []corev1.Pod{p}}, nil
			}
		}
		return nil, fmt.Errorf("no running pod found for component %s", compKey.compName)
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
	ml := client.MatchingLabels(constant.GetComponentWellKnownLabels(cluster.Name, getCompNameShort(comp)))
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

func passPreCondition(ctx context.Context, r client.Client, cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, cmpd *appsv1alpha1.ComponentDefinition) bool {
	// pass pre-condition check
	preConditionType := appsv1alpha1.ComponentReadyPreConditionType
	handler := cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler
	if handler != nil {
		if handler.PreCondition != nil {
			preConditionType = *cmpd.Spec.LifecycleActions.AccountProvision.CustomHandler.PreCondition
		}
	}
	switch preConditionType {
	case appsv1alpha1.ComponentReadyPreConditionType:
		return comp.Status.Phase == appsv1alpha1.RunningClusterCompPhase
	case appsv1alpha1.ClusterReadyPreConditionType:
		return cluster.Status.Phase == appsv1alpha1.RunningClusterPhase
	case appsv1alpha1.RuntimeReadyPreConditionType:
		podList := &corev1.PodList{}
		labels := constant.GetComponentWellKnownLabels(cluster.Name, getCompNameShort(comp))
		if err := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return false
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false
			}
		}
		return true
	}
	return false
}

func getCompNameShort(comp *appsv1alpha1.Component) string {
	return comp.Labels[constant.KBAppComponentLabelKey]
}

func getAccountSecretByName(ctx context.Context, client client.Client, compKey *compUniqueKey, accountName string) (*corev1.Secret, error) {
	secretKey := types.NamespacedName{
		Namespace: compKey.namespace,
		Name:      constant.GenerateAccountSecretName(compKey.clusterName, compKey.compName, accountName),
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, secretKey, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func getAccountsProvisioned(reqCtx intctrlutil.RequestCtx, r client.Client, compKey *compUniqueKey) (accountNameSet, error) {
	matchingLabels := client.MatchingLabels(constant.GetComponentWellKnownLabels(compKey.clusterName, compKey.compName))
	accounts := accountNameSet{}

	secrets := &corev1.SecretList{}
	if err := r.List(reqCtx.Ctx, secrets, client.InNamespace(compKey.namespace), matchingLabels); err != nil {
		return nil, err
	}
	// parse account name from secret's label
	for _, secret := range secrets.Items {
		if accountName, exists := secret.ObjectMeta.Labels[constant.ClusterAccountLabelKey]; exists {
			annotations := secret.GetAnnotations()
			if annotations == nil {
				continue
			}
			if val, ok := annotations[constant.ComponentAccountProvisionKey]; ok && val == constant.AccountProvisioned {
				accounts.Add(accountName)
			}
		}
	}
	// get all running jobs
	jobs := &batchv1.JobList{}
	if err := r.List(reqCtx.Ctx, jobs, client.InNamespace(compKey.namespace), matchingLabels); err != nil {
		return nil, err
	}
	// parse account name from job's label
	for _, job := range jobs.Items {
		if accountName, exists := job.ObjectMeta.Labels[constant.ClusterAccountLabelKey]; exists {
			accounts.Add(accountName)
		}
	}
	return accounts, nil
}

func getAccountsInEngine(reqCtx intctrlutil.RequestCtx, r client.Client, compKey *compUniqueKey) accountNameSet {
	accounts := accountNameSet{}

	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(compKey.clusterName, compKey.compName)
	if err := r.List(reqCtx.Ctx, podList, client.InNamespace(compKey.namespace), client.MatchingLabels(labels)); err != nil {
		reqCtx.Log.Error(err, "failed to get pods for component", "component", compKey.compName)
		return accounts
	}
	// find the first running pod
	var target *corev1.Pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			target = &pod
			break
		}
	}

	if target == nil {
		reqCtx.Log.V(1).Info("no pod is running for component", "component", compKey.compName)
		return accounts
	}

	lorryClient, err := lorry.NewClient(*target)
	if err != nil {
		reqCtx.Log.Error(err, "failed to create lorry client", "pod", target.Name)
		return accounts
	}
	if intctrlutil.IsNil(lorryClient) {
		reqCtx.Log.Info("failed to create lorry client", "pod", target.Name)
		return accounts
	}
	accountsName, err := lorryClient.ListSystemAccounts(reqCtx.Ctx)
	if err != nil {
		reqCtx.Log.Error(err, "exec lorry client with err", "pod", target.Name)
		return accounts
	}

	for _, acc := range accountsName {
		if name, ok := acc["userName"]; ok {
			accounts.Add(name.(string))
		}
	}
	return accounts
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}

func getClusterName(comp *appsv1alpha1.Component) (string, error) {
	// get cluster info
	compKey := types.NamespacedName{Namespace: comp.Namespace, Name: comp.Name}
	clusterName := ""
	compOwneres := comp.GetObjectMeta().GetOwnerReferences()
	if len(compOwneres) < 1 {
		return clusterName, fmt.Errorf("component %s has no owner", compKey)
	}

	for _, owner := range compOwneres {
		if owner.Kind == appsv1alpha1.ClusterKind {
			clusterName = owner.Name
			return clusterName, nil
		}
	}
	return clusterName, fmt.Errorf("failed to parse Cluster from component %s", compKey)
}

// Add adds an element to the set.
func (s accountNameSet) Add(element string) {
	s[element] = struct{}{}
}

// Remove removes an element from the set.
func (s accountNameSet) Remove(element string) {
	delete(s, element)
}

// Contains checks if the set contains a given element.
func (s accountNameSet) Contains(element string) bool {
	_, exists := s[element]
	return exists
}

// Size returns the number of elements in the set.
func (s accountNameSet) Size() int {
	return len(s)
}

// Print prints the elements of the set.
func (s accountNameSet) Print() {
	for element := range s {
		fmt.Println(element)
	}
}
