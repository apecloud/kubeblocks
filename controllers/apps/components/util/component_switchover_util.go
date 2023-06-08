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
	intctrlcomputil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ReasonSwitchoverSucceed = "SwitchoverSucceed" // ReasonSwitchoverSucceed the component switchover succeed
	ReasonSwitchoverStart   = "SwitchoverStart"   // ReasonSwitchoverSucceed the component is starting switchover
)

// HandleSwitchover is the entrypoint of workload switchover
func HandleSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomputil.SynthesizedComponent,
	obj client.Object) error {
	if component.CandidateInstance == nil {
		return nil
	}
	// check if all Pods have role label
	podList, err := GetRunningPods(ctx, cli, obj)
	if err != nil {
		return err
	}
	if component.CandidateInstance.Index > int32(len(podList)-1) {
		return errors.New("the candidate instance index is out of range")
	}
	for _, pod := range podList {
		// if the pod does not have the role label, we do nothing.
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			return nil
		}
	}

	// check if the switchover is needed
	needSwitchover, err := NeedDealWithSwitchover(ctx, cli, cluster, component)
	if err != nil {
		return err
	}
	if needSwitchover {
		// create a job to do switchover and check the result
		if err := DoSwitchover(ctx, cli, cluster, component); err != nil {
			return err
		}
	} else {
		// if the switchover is not needed, it means that the switchover has been completed, and the switchover job can be deleted.
		if err := PostOpsSwitchover(ctx, cli, cluster, component); err != nil {
			return err
		}
	}
	return nil
}

// HandleFailoverSync synchronizes the results of failover to candidateInstance if necessary.
func HandleFailoverSync(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomputil.SynthesizedComponent) error {
	if component.CandidateInstance == nil {
		return nil
	}
	// if the failover sync is not enabled, we do not sync the failover result.
	if !component.CandidateInstance.FailoverSync {
		return nil
	}
	switchoverCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeSwitchoverPrefix+component.Name)
	// if the switchover condition is exist and status is not true, it means that the switchover is not completed, we do not sync the failover result.
	if switchoverCondition != nil && switchoverCondition.Status != metav1.ConditionTrue {
		return nil
	}
	ok, pod, err := checkPodRoleLabelConsistency(ctx, cli, cluster, component)
	if err != nil {
		return err
	}
	if ok || pod == nil {
		return nil
	}
	// synchronize the current primary or leader pod ordinal to candidateInstance.Index and set candidateInstance.Operator to equal.
	_, o := ParseParentNameAndOrdinal(pod.Name)
	for index, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == component.Name {
			cluster.Spec.ComponentSpecs[index].CandidateInstance = &appsv1alpha1.CandidateInstance{
				Index:        o,
				Operator:     appsv1alpha1.CandidateOpEqual,
				FailoverSync: true,
			}
		}
	}
	return nil
}

// NeedDealWithSwitchover checks whether we need to handle the switchover process.
func NeedDealWithSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomputil.SynthesizedComponent) (bool, error) {
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
	component *intctrlcomputil.SynthesizedComponent) error {
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
		if len(previousJobs) > 0 {
			// delete the previous generation switchoverJob
			if err := CleanJobWithLabels(ctx, cli, cluster, ml); err != nil {
				return err
			}
		}

		// update status.conditions to SwitchoverStart
		newSwitchoverCondition := initSwitchoverCondition(*component.CandidateInstance, component.Name, metav1.ConditionFalse, ReasonSwitchoverStart, cluster.Generation)
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
	component *intctrlcomputil.SynthesizedComponent) error {
	if component.CandidateInstance == nil {
		return nil
	}
	oldSwitchoverCondition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeSwitchoverPrefix+component.Name)
	if oldSwitchoverCondition.Status == metav1.ConditionTrue {
		return nil
	}

	ml := getSwitchoverCmdJobLabel(cluster.Name, component.Name)
	// check pod role label consistency
	ok, _, err := checkPodRoleLabelConsistency(ctx, cli, cluster, component)
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
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, compSpec.Name, compSpec.ComponentDefRef)
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
func getPrimaryOrLeaderPod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpecName, compDefName string) (*corev1.Pod, error) {
	var (
		err     error
		podList *corev1.PodList
	)
	compDef, err := GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(getSupportSwitchoverWorkload(), compDef.WorkloadType) {
		return nil, errors.New("component does not support switchover")
	}
	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, compSpecName, constant.Primary)
	case appsv1alpha1.Consensus:
		podList, err = GetComponentPodListWithRole(ctx, cli, cluster, compSpecName, constant.Leader)
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
	component *intctrlcomputil.SynthesizedComponent) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, component.Name, component.CompDefName)
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
				Name:  constant.KBSwitchoverReplicationPrimaryPodIP,
				Value: pod.Status.PodIP,
			},
			{
				Name:  constant.KBSwitchoverReplicationPrimaryPodName,
				Value: pod.Name,
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
				Name:  constant.KBSwitchoverConsensusLeaderPodIP,
				Value: pod.Status.PodIP,
			},
			{
				Name:  constant.KBSwitchoverConsensusLeaderPodName,
				Value: pod.Name,
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
	component *intctrlcomputil.SynthesizedComponent) []corev1.EnvVar {
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
	component *intctrlcomputil.SynthesizedComponent) ([]corev1.EnvVar, error) {
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
	component *intctrlcomputil.SynthesizedComponent) (*batchv1.Job, error) {
	if component.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}

	renderJob := func(switchoverSpec *appsv1alpha1.SwitchoverSpec, switchoverEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var switchoverAction *appsv1alpha1.SwitchoverAction
		switch component.CandidateInstance.Operator {
		case appsv1alpha1.CandidateOpEqual:
			switchoverAction = switchoverSpec.WithCandidateInstance
		case appsv1alpha1.CandidateOpNotEqual:
			switchoverAction = switchoverSpec.WithoutCandidateInstance
		}
		if switchoverAction == nil {
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
func checkPodRoleLabelConsistency(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *intctrlcomputil.SynthesizedComponent) (bool, *corev1.Pod, error) {
	if component.CandidateInstance == nil {
		return true, nil, nil
	}
	// get the Pod object whose current role label is primary or leader
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, component.Name, component.CompDefName)
	if err != nil {
		return false, nil, err
	}
	if pod == nil {
		return false, nil, nil
	}
	candidateInstanceName := fmt.Sprintf("%s-%s-%d", cluster.Name, component.Name, component.CandidateInstance.Index)
	if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpEqual {
		return pod.Name == candidateInstanceName, pod, nil
	}
	if component.CandidateInstance.Operator == appsv1alpha1.CandidateOpNotEqual {
		return pod.Name != candidateInstanceName, pod, nil
	}
	return false, pod, nil
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

// GetRunningPods gets the running pods of the specified statefulSet.
func GetRunningPods(ctx context.Context, cli client.Client, obj client.Object) ([]corev1.Pod, error) {
	sts := ConvertToStatefulSet(obj)
	if sts == nil || sts.Generation != sts.Status.ObservedGeneration {
		return nil, nil
	}
	return GetPodListByStatefulSet(ctx, cli, sts)
}
