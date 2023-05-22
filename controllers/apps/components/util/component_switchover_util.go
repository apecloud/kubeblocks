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

package util

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlcomputil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func doSwitchover() error {
	return nil
}

// getPrimaryOrLeaderPod returns the leader or primary pod of the component.
func getPrimaryOrLeaderPod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpec appsv1alpha1.ClusterComponentSpec) (*corev1.Pod, error) {
	compDef, err := GetComponentDefByCluster(ctx, cli, cluster, compSpec.ComponentDefRef)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(getSupportSwitchoverWorkload(), compDef.WorkloadType) {
		return nil, errors.New("component does not support switchover")
	}
	var podList *corev1.PodList
	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, compSpec.Name, constant.Primary)
	case appsv1alpha1.Consensus:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, compSpec.Name, constant.Leader)
	}
	if err != nil {
		return nil, err
	}
	if len(podList.Items) != 1 {
		return nil, errors.New("component pod list is empty or has more than one pod")
	}
	return &podList.Items[0], nil
}

// getPrimaryOrLeaderPodOrdinal returns the leader or primary pod ordinal of the component
func getPrimaryOrLeaderPodOrdinal(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpec appsv1alpha1.ClusterComponentSpec) (int, error) {
	pod, err := getPrimaryOrLeaderPod(ctx, cli, cluster, compSpec)
	if err != nil {
		return -1, err
	}
	if pod == nil {
		return -1, nil
	}
	_, ordinal := intctrlutil.GetParentNameAndOrdinal(pod)
	return ordinal, nil
}

// CheckCandidateInstanceChanged checks whether candidateInstance has changed.
// @return bool - true is candidateInstance inconsistent
// @return string - current primary/leader Instance name; "" if error
// @return error
func CheckCandidateInstanceChanged(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (bool, string, error) {
	if clusterCompSpec == nil || clusterCompSpec.CandidateInstance == nil {
		return false, "", nil
	}
	// get the Pod object whose current role label is primary or leader
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, *clusterCompSpec)
	if err != nil {
		return false, "", err
	}
	if pod == nil {
		return false, "", nil
	}
	candidateInstanceName := fmt.Sprintf("%s-%s-%d", cluster.Name, clusterCompSpec.Name, clusterCompSpec.CandidateInstance.Index)
	if clusterCompSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		return pod.Name != candidateInstanceName, pod.Name, nil
	}
	if clusterCompSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual {
		return pod.Name == candidateInstanceName, pod.Name, nil
	}
	return false, pod.Name, nil
}

// getSupportSwitchoverWorkload returns the kinds that support switchover.
func getSupportSwitchoverWorkload() []appsv1alpha1.WorkloadType {
	return []appsv1alpha1.WorkloadType{
		appsv1alpha1.Replication,
		appsv1alpha1.Consensus,
	}
}

// getSwitchoverCmdJobLabel gets the labels for job that execute the switchover commands.
func getSwitchoverCmdJobLabel(clusterName, componentName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:     clusterName,
		constant.KBAppComponentLabelKey:  componentName,
		constant.AppManagedByLabelKey:    constant.AppName,
		constant.KBSwitchoverJobLabelKey: constant.KBSwitchoverJobLabelValue,
	}
}

// replaceSwitchoverConnCredentialEnv replaces the connection credential environment variables for the switchover job.
func replaceSwitchoverConnCredentialEnv(clusterName string, switchoverSpec *appsv1alpha1.SwitchoverSpec) {
	namedValuesMap := intctrlcomputil.GetEnvReplacementMapForConnCredential(clusterName)
	if switchoverSpec != nil {
		switchoverSpec.Env = intctrlcomputil.ReplaceSecretEnvVars(namedValuesMap, switchoverSpec.Env)
	}
}

// buildSwitchoverWorkloadEnvs builds the replication or consensus workload environment variables for the switchover job.
func buildSwitchoverWorkloadEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, *clusterCompSpec)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("primary/leader pod not found")
	}
	svcName := strings.Join([]string{cluster.Name, clusterCompSpec.Name, "headless"}, "-")
	switch componentDef.WorkloadType {
	case appsv1alpha1.Replication:
		workloadEnvs = append(workloadEnvs, corev1.EnvVar{
			Name:  constant.KBSwitchoverReplicationPrimaryPodName,
			Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
		})
	case appsv1alpha1.Consensus:
		workloadEnvs = append(workloadEnvs, corev1.EnvVar{
			Name:  constant.KBSwitchoverConsensusLeaderPodName,
			Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
		})
	}
	return workloadEnvs, nil
}

// buildSwitchoverCandidateInstanceEnv builds the candidate instance name environment variable for the switchover job.
func buildSwitchoverCandidateInstanceEnv(
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) *corev1.EnvVar {
	if clusterCompSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		return &corev1.EnvVar{
			Name:  constant.KBSwitchoverCandidateInstanceName,
			Value: fmt.Sprintf("%s-%s-%d", cluster.Name, clusterCompSpec.Name, clusterCompSpec.CandidateInstance.Index),
		}
	}
	return nil
}

// buildSwitchoverEnvs builds the environment variables for the switchover job.
func buildSwitchoverEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) ([]corev1.EnvVar, error) {
	if clusterCompSpec == nil || componentDef.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}
	var switchoverEnvs []corev1.EnvVar
	// replace secret env and merge envs defined in SwitchoverSpec
	replaceSwitchoverConnCredentialEnv(cluster.Name, componentDef.SwitchoverSpec)
	switchoverEnvs = append(switchoverEnvs, componentDef.SwitchoverSpec.Env...)

	// inject the old primary or leader info into the environment variable
	workloadEnvs, err := buildSwitchoverWorkloadEnvs(ctx, cli, cluster, clusterCompSpec, componentDef)
	if err != nil {
		return nil, err
	}
	switchoverEnvs = append(switchoverEnvs, workloadEnvs...)

	// inject the candidate instance name into the environment variable if specify the candidate instance
	candidateInstanceEnv := buildSwitchoverCandidateInstanceEnv(cluster, clusterCompSpec)
	if candidateInstanceEnv != nil {
		switchoverEnvs = append(switchoverEnvs, *candidateInstanceEnv)
	}
	return switchoverEnvs, nil
}

// renderAndCreateSwitchoverCmdJobs renders and creates the switchover command jobs.
func renderAndCreateSwitchoverCmdJobs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) (*batchv1.Job, error) {
	if clusterCompSpec == nil || componentDef.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}

	renderJob := func(jobName string, switchoverSpec *appsv1alpha1.SwitchoverSpec, switchoverEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var switchoverAction *appsv1alpha1.SwitchoverAction
		if clusterCompSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual && switchoverSpec.WithCandidateInstance != nil {
			switchoverAction = switchoverSpec.WithCandidateInstance
		} else if clusterCompSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual && switchoverSpec.WithoutCandidateInstance != nil {
			switchoverAction = switchoverSpec.WithoutCandidateInstance
		} else {
			return nil, errors.New("switchover action not found")
		}
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getSwitchoverCmdJobLabel(cluster.Name, clusterCompSpec.Name),
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cluster.Namespace,
						Name:      jobName,
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:            constant.KBSwitchoverJobContainerName,
								Image:           switchoverSpec.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         switchoverAction.Command,
								Args:            switchoverAction.Args,
								Env:             switchoverEnvs,
							},
						},
					},
				},
			},
		}
		if len(cluster.Spec.Tolerations) > 0 {
			job.Spec.Template.Spec.Tolerations = cluster.Spec.Tolerations
		}
		return job, nil
	}

	switchoverEnvs, err := buildSwitchoverEnvs(ctx, cli, cluster, clusterCompSpec, componentDef)
	if err != nil {
		return nil, err
	}
	jobName := fmt.Sprintf("%s-%s-%d", constant.KBSwitchoverJobNamePrefix, clusterCompSpec.Name, cluster.Generation)
	job, err := renderJob(jobName, componentDef.SwitchoverSpec, switchoverEnvs)
	if err != nil {
		return job, err
	}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	exists, _ := intctrlutil.CheckResourceExists(ctx, cli, key, &batchv1.Job{})
	if exists {
		return job, nil
	}
	// if job not exist, create a job
	if err := cli.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// checkPodRoleLabelConsistency checks whether the pod role label is consistent with the specified role label.
func checkPodRoleLabelConsistency(pod *corev1.Pod, roleLabel string) bool {
	if pod.Labels == nil {
		return false
	}
	if pod.Labels[constant.RoleLabelKey] != roleLabel {
		return false
	}
	return true
}
