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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlcomputil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ReasonSwitchoverSucceed = "SwitchoverSucceed" // ReasonSwitchoverSucceed the component switchover succeed
	ReasonSwitchoverStart   = "SwitchoverStart"   // ReasonSwitchoverSucceed the component is starting switchover
)

// NeedDeaWithSwitchover checks whether we need to handle the switchover process.
func NeedDeaWithSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	// firstly, check whether the candidateInstance is changed by comparing with the pod role label
	changed, _, err := CheckCandidateInstanceChanged(ctx, cli, cluster, component.Name)
	if err != nil {
		return false, err
	}
	// if the candidateInstance is not changed, no need to deal with switchover
	if !changed {
		return false, nil
	}

	// secondly, check the switchover condition information, according to the condition of switchover to judged whether switchover is required.
	oldSwitchoverCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeSwitchoverPrefix+component.Name)
	if oldSwitchoverCondition == nil {
		// TODO(xingran):under the current implementation, the following scenarios need to be optimized:
		// when the candidateInstance is patched for the first time, but there is no switching (for example, the specified index is consistent with the current primary),
		// and then a failover occurs, and the result of the failover is not synchronized to the candidateInstance, and an unexpected switchover will occur at this time

		// if the switchover condition is not exist, it means the first time to do switchover.
		return true, nil
	}

	// if the old switchover condition status is true, it indicates that the last switchover has been successful.
	// We need to judge whether the current candidateInstance information is consistent with the last successful switchover in order to decide whether to perform a new switchover.
	if oldSwitchoverCondition.Status == metav1.ConditionTrue {
		conditionChanged, err := switchoverConditionIsChanged(cluster, component.CandidateInstance, component.Name)
		if err != nil {
			return false, err
		}
		if conditionChanged {
			// if switchover condition candidateInstance information is changed, it means that another new switchover is triggered.
			return true, nil
		}
		if oldSwitchoverCondition.ObservedGeneration != cluster.Generation {
			oldSwitchoverCondition.ObservedGeneration = cluster.Generation
			meta.SetStatusCondition(&cluster.Status.Conditions, *oldSwitchoverCondition)
		}
		// TODO(xingran): under the current implementation, the following scenarios need to be optimized:
		// when a failover occurs, and the result of the failover is not synchronized to the candidateInstance (eg. candidateInstance.failoverSync=false),
		// at this time, the information of candidateInstance is inconsistent with the current primary or leader,
		// and the user cannot switch back to the node in the current candidateInstance at this time because the switchover condition is not changed.
		return false, nil
	}

	// if the old switchover condition status is not true, it means that the current switchover has not been completed,
	// and need to go further to handle switchover.
	return true, nil
}

// DoSwitchover is used to perform switchover operations.
func DoSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) error {
	switchoverJob, err := renderSwitchoverCmdJob(ctx, cli, cluster, component)
	if err != nil {
		return err
	}
	// check the current generation switchoverJob whether exist
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: switchoverJob.Name}
	ml := getSwitchoverCmdJobLabel(cluster.Name, component.Name)
	exists, _ := intctrlutil.CheckResourceExists(ctx, cli, key, &batchv1.Job{})
	if !exists {
		// check the previous generation switchoverJob whether exist
		previousJobs, err := GetJobWithLabels(ctx, cli, cluster, ml)
		if err != nil {
			return err
		}
		var newSwitchoverCondition *metav1.Condition
		if len(previousJobs) > 0 {
			// TODO: delete the previous generation switchoverJob

		}

		// update status.conditions to SwitchoverStart
		newSwitchoverCondition = initSwitchoverCondition(*component.CandidateInstance, component.Name, metav1.ConditionFalse, ReasonSwitchoverStart, cluster.Generation)
		meta.SetStatusCondition(&cluster.Status.Conditions, *newSwitchoverCondition)

		// create the current generation switchoverJob
		if err := cli.Create(ctx, switchoverJob); err != nil {
			return err
		}
	}
	// check the current generation switchoverJob whether succeed
	if err := CheckJobSucceed(ctx, cli, cluster, switchoverJob); err != nil {
		return err
	}

	return PostOpsSwitchover(ctx, cli, cluster, component)
}

// PostOpsSwitchover is used to do some post operations after switchover job execute successfully.
func PostOpsSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) error {
	if component.CandidateInstance == nil {
		return nil
	}
	oldSwitchoverCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeSwitchoverPrefix+component.Name)
	if oldSwitchoverCondition.Status == metav1.ConditionTrue {
		return nil
	}

	ml := getSwitchoverCmdJobLabel(cluster.Name, component.Name)
	// check pod role label consistency
	ok, err := checkPodRoleLabelConsistencyAfterSwitchover(ctx, cli, cluster, component)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("pod role label consistency check failed after switchover.")
	}

	// update status.conditions to SwitchoverSucceed
	newSwitchoverCondition := initSwitchoverCondition(*component.CandidateInstance, component.Name, metav1.ConditionTrue, ReasonSwitchoverSucceed, cluster.Generation)
	meta.SetStatusCondition(&cluster.Status.Conditions, *newSwitchoverCondition)

	// delete the successful job
	return CleanJobWithLabels(ctx, cli, cluster, ml)
}

// CheckCandidateInstanceChanged checks whether candidateInstance has changed.
// @return bool - true is candidateInstance inconsistent
// @return string - current primary/leader Instance name; "" if error
// @return error
func CheckCandidateInstanceChanged(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentName string) (bool, string, error) {
	compSpec := GetClusterComponentSpecByName(*cluster, componentName)
	if compSpec.CandidateInstance == nil {
		return false, "", nil
	}
	// get the Pod object whose current role label is primary or leader
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, compSpec.Name)
	if err != nil {
		return false, "", err
	}
	if pod == nil {
		return false, "", nil
	}
	candidateInstanceName := fmt.Sprintf("%s-%s-%d", cluster.Name, componentName, compSpec.CandidateInstance.Index)
	if compSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		return pod.Name != candidateInstanceName, pod.Name, nil
	}
	if compSpec.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual {
		return pod.Name == candidateInstanceName, pod.Name, nil
	}
	return false, pod.Name, nil
}

// getPrimaryOrLeaderPod returns the leader or primary pod of the component.
func getPrimaryOrLeaderPod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, componentName string) (*corev1.Pod, error) {
	var (
		err     error
		podList *corev1.PodList
	)
	compDef, err := GetComponentDefByCluster(ctx, cli, cluster, componentName)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(getSupportSwitchoverWorkload(), compDef.WorkloadType) {
		return nil, errors.New("component does not support switchover")
	}
	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, componentName, constant.Primary)
	case appsv1alpha1.Consensus:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, componentName, constant.Leader)
	}
	if err != nil {
		return nil, err
	}
	if len(podList.Items) != 1 {
		return nil, errors.New("component pod list is empty or has more than one pod")
	}
	return &podList.Items[0], nil
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
	component *component.SynthesizedComponent) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, component.Name)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("primary or leader pod not found")
	}
	svcName := strings.Join([]string{cluster.Name, component.Name, "headless"}, "-")
	switch component.WorkloadType {
	case appsv1alpha1.Replication:
		rsEnvs := []corev1.EnvVar{
			{
				Name:  constant.KBSwitchoverReplicationPrimaryPodIp,
				Value: fmt.Sprintf("%s", pod.Status.PodIP),
			},
			{
				Name:  constant.KBSwitchoverReplicationPrimaryPodName,
				Value: fmt.Sprintf("%s", pod.Name),
			},
			{
				Name:  constant.KBSwitchoverReplicationPrimaryPodFqdn,
				Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
			},
		}
		workloadEnvs = append(workloadEnvs, rsEnvs...)
	case appsv1alpha1.Consensus:
		csEnvs := []corev1.EnvVar{
			{
				Name:  constant.KBSwitchoverConsensusLeaderPodIp,
				Value: fmt.Sprintf("%s", pod.Status.PodIP),
			},
			{
				Name:  constant.KBSwitchoverConsensusLeaderPodName,
				Value: fmt.Sprintf("%s", pod.Name),
			},
			{
				Name:  constant.KBSwitchoverConsensusLeaderPodFqdn,
				Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
			},
		}
		workloadEnvs = append(workloadEnvs, csEnvs...)
	}
	return workloadEnvs, nil
}

// buildSwitchoverCandidateInstanceEnv builds the candidate instance name environment variable for the switchover job.
func buildSwitchoverCandidateInstanceEnv(
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) []corev1.EnvVar {
	svcName := strings.Join([]string{cluster.Name, component.Name, "headless"}, "-")
	if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		cEnvs := []corev1.EnvVar{
			{
				Name:  constant.KBSwitchoverCandidateInstanceName,
				Value: fmt.Sprintf("%s-%s-%d", cluster.Name, component.Name, component.CandidateInstance.Index),
			},
			{
				Name:  constant.KBSwitchoverCandidateInstanceFqdn,
				Value: fmt.Sprintf("%s-%s-%d.%s", cluster.Name, component.Name, component.CandidateInstance.Index, svcName),
			},
		}
		return cEnvs
	}
	return nil
}

// buildSwitchoverEnvs builds the environment variables for the switchover job.
func buildSwitchoverEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) ([]corev1.EnvVar, error) {
	if component.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}
	var switchoverEnvs []corev1.EnvVar
	// replace secret env and merge envs defined in SwitchoverSpec
	replaceSwitchoverConnCredentialEnv(cluster.Name, component.SwitchoverSpec)
	switchoverEnvs = append(switchoverEnvs, component.SwitchoverSpec.Env...)

	// inject the old primary or leader info into the environment variable
	workloadEnvs, err := buildSwitchoverWorkloadEnvs(ctx, cli, cluster, component)
	if err != nil {
		return nil, err
	}
	switchoverEnvs = append(switchoverEnvs, workloadEnvs...)

	// inject the candidate instance name into the environment variable if specify the candidate instance
	candidateInstanceEnvs := buildSwitchoverCandidateInstanceEnv(cluster, component)
	switchoverEnvs = append(switchoverEnvs, candidateInstanceEnvs...)
	return switchoverEnvs, nil
}

// renderSwitchoverCmdJob renders and creates the switchover command jobs.
func renderSwitchoverCmdJob(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (*batchv1.Job, error) {
	if component.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}

	renderJob := func(switchoverSpec *appsv1alpha1.SwitchoverSpec, switchoverEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var switchoverAction *appsv1alpha1.SwitchoverAction
		if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual && switchoverSpec.WithCandidateInstance != nil {
			switchoverAction = switchoverSpec.WithCandidateInstance
		} else if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual && switchoverSpec.WithoutCandidateInstance != nil {
			switchoverAction = switchoverSpec.WithoutCandidateInstance
		} else {
			return nil, errors.New("switchover action not found")
		}
		// jobName named with generation to distinguish different switchover jobs.
		jobName := fmt.Sprintf("%s-%s-%s-%d", constant.KBSwitchoverJobNamePrefix, cluster.Name, component.Name, cluster.Generation)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getSwitchoverCmdJobLabel(cluster.Name, component.Name),
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

	switchoverEnvs, err := buildSwitchoverEnvs(ctx, cli, cluster, component)
	if err != nil {
		return nil, err
	}
	job, err := renderJob(component.SwitchoverSpec, switchoverEnvs)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// GetJobWithLabels gets the job list with the specified labels.
func GetJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) ([]batchv1.Job, error) {
	jobList := &batchv1.JobList{}
	if err := cli.List(ctx, jobList, client.InNamespace(cluster.Namespace), matchLabels); err != nil {
		return nil, err
	}
	return jobList.Items, nil
}

// CleanJobWithLabels cleans up the job task that execute the switchover commands.
func CleanJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) error {
	jobList, err := GetJobWithLabels(ctx, cli, cluster, matchLabels)
	if err != nil {
		return err
	}
	for _, job := range jobList {
		var ttl = int32(constant.KBJobTTLSecondsAfterFinished)
		patch := client.MergeFrom(job.DeepCopy())
		job.Spec.TTLSecondsAfterFinished = &ttl
		if err := cli.Patch(ctx, &job, patch); err != nil {
			return err
		}
	}
	return nil
}

// CheckJobSucceed checks the result of job execution.
func CheckJobSucceed(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	job *batchv1.Job) error {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: job.Name}
	currentJob := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(ctx, cli, key, &currentJob)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("job not exist, pls check.")
	}
	jobStatusConditions := currentJob.Status.Conditions
	if len(jobStatusConditions) > 0 {
		switch jobStatusConditions[0].Type {
		case batchv1.JobComplete:
			return nil
		case batchv1.JobFailed:
			return errors.New("job failed, pls check.")
		default:
			return errors.New("job unfinished.")
		}
	} else {
		return errors.New("job check conditions status failed")
	}
}

// checkPodRoleLabelConsistency checks whether the pod role label is consistent with the specified role label.
func checkPodRoleLabelConsistencyAfterSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent) (bool, error) {
	if component.CandidateInstance == nil {
		return true, nil
	}
	// get the Pod object whose current role label is primary or leader
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, component.Name)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	candidateInstanceName := fmt.Sprintf("%s-%s-%d", cluster.Name, component.Name, component.CandidateInstance.Index)
	if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		return pod.Name == candidateInstanceName, nil
	}
	if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual {
		return pod.Name != candidateInstanceName, nil
	}
	return false, nil
}

// switchoverConditionIsChanged checks whether the switchover condition candidateInstance information is changed.
func switchoverConditionIsChanged(cluster *appsv1alpha1.Cluster, currentCandidateInstance *appsv1alpha1.CandidateInstance, componentName string) (bool, error) {
	switchoverCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeSwitchoverPrefix+componentName)
	if switchoverCondition == nil {
		return true, nil
	}
	var oldCandidateInstance *appsv1alpha1.CandidateInstance
	if err := json.Unmarshal([]byte(switchoverCondition.Message), &oldCandidateInstance); err != nil {
		return false, err
	}
	return !reflect.DeepEqual(oldCandidateInstance, currentCandidateInstance), nil
}

// initSwitchoverCondition initializes the switchover condition.
func initSwitchoverCondition(candidateInstance appsv1alpha1.CandidateInstance, componentName string, status metav1.ConditionStatus, reason string, clusterGeneration int64) *metav1.Condition {
	msg, _ := json.Marshal(candidateInstance)
	return &metav1.Condition{
		Type:               appsv1alpha1.ConditionTypeSwitchoverPrefix + componentName,
		Status:             status,
		Message:            string(msg),
		Reason:             reason,
		ObservedGeneration: clusterGeneration,
	}
}
