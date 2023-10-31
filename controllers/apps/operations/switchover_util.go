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

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomputil "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	SwitchoverCheckJobKey       = "CheckJob"
	SwitchoverCheckRoleLabelKey = "CheckRoleLabel"

	OpsReasonForSkipSwitchover = "SkipSwitchover"
)

// needDoSwitchover checks whether we need to perform a switchover.
func needDoSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	switchover *appsv1alpha1.Switchover) (bool, error) {
	// get the Pod object whose current role label is primary
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, componentSpec.Name, componentSpec.ComponentDefRef)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	switch switchover.InstanceName {
	case constant.KBSwitchoverCandidateInstanceForAnyPod:
		return true, nil
	default:
		podList, err := components.GetComponentPodList(ctx, cli, *cluster, componentSpec.Name)
		if err != nil {
			return false, err
		}
		podParent, _ := common.ParseParentNameAndOrdinal(pod.Name)
		siParent, o := common.ParseParentNameAndOrdinal(switchover.InstanceName)
		if podParent != siParent || o < 0 || o >= int32(len(podList.Items)) {
			return false, errors.New("switchover.InstanceName is invalid")
		}
		// If the current instance is already the primary, then no switchover will be performed.
		if pod.Name == switchover.InstanceName {
			return false, nil
		}
	}
	return true, nil
}

// createSwitchoverJob creates a switchover job to do switchover.
func createSwitchoverJob(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	switchover *appsv1alpha1.Switchover) error {
	switchoverJob, err := renderSwitchoverCmdJob(reqCtx.Ctx, cli, cluster, componentSpec, componentDef, switchover)
	if err != nil {
		return err
	}
	// check the current generation switchoverJob whether exist
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: switchoverJob.Name}
	exists, _ := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, key, &batchv1.Job{})
	if !exists {
		// check the previous generation switchoverJob whether exist
		ml := getSwitchoverCmdJobLabel(cluster.Name, componentSpec.Name)
		previousJobs, err := getJobWithLabels(reqCtx.Ctx, cli, cluster, ml)
		if err != nil {
			return err
		}
		if len(previousJobs) > 0 {
			// delete the previous generation switchoverJob
			reqCtx.Log.V(1).Info("delete previous generation switchoverJob", "job", previousJobs[0].Name)
			if err := cleanJobWithLabels(reqCtx.Ctx, cli, cluster, ml); err != nil {
				return err
			}
		}
		// create the current generation switchoverJob
		if err := cli.Create(reqCtx.Ctx, switchoverJob); err != nil {
			return err
		}
		return nil
	}
	return nil
}

// checkPodRoleLabelConsistency checks whether the pod role label is consistent with the specified role label after switchover.
func checkPodRoleLabelConsistency(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	switchover *appsv1alpha1.Switchover,
	switchoverCondition *metav1.Condition) (bool, error) {
	if switchover == nil || switchoverCondition == nil {
		return false, nil
	}
	// get the Pod object whose current role label is primary
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, componentSpec.Name, componentDef.Name)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	var switchoverMessageMap map[string]SwitchoverMessage
	if err := json.Unmarshal([]byte(switchoverCondition.Message), &switchoverMessageMap); err != nil {
		return false, err
	}

	for _, switchoverMessage := range switchoverMessageMap {
		if switchoverMessage.ComponentName != componentSpec.Name {
			continue
		}
		switch switchoverMessage.Switchover.InstanceName {
		case constant.KBSwitchoverCandidateInstanceForAnyPod:
			if pod.Name != switchoverMessage.OldPrimary {
				return true, nil
			}
		default:
			if pod.Name == switchoverMessage.Switchover.InstanceName {
				return true, nil
			}
		}
	}
	return false, nil
}

// renderSwitchoverCmdJob renders and creates the switchover command jobs.
func renderSwitchoverCmdJob(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	switchover *appsv1alpha1.Switchover) (*batchv1.Job, error) {
	if componentDef.SwitchoverSpec == nil || switchover == nil {
		return nil, errors.New("switchover spec not found")
	}
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, componentSpec.Name, componentDef.Name)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("primary pod not found")
	}

	renderJobPodVolumes := func(scriptSpecSelectors []appsv1alpha1.ScriptSpecSelector) ([]corev1.Volume, []corev1.VolumeMount) {
		volumes := make([]corev1.Volume, 0)
		volumeMounts := make([]corev1.VolumeMount, 0)

		// find current pod's volume which mapped to configMapRefs
		findVolumes := func(tplSpec appsv1alpha1.ComponentTemplateSpec, scriptSpecSelector appsv1alpha1.ScriptSpecSelector) {
			if tplSpec.Name != scriptSpecSelector.Name {
				return
			}
			for _, podVolume := range pod.Spec.Volumes {
				if podVolume.Name == tplSpec.VolumeName {
					volumes = append(volumes, podVolume)
					break
				}
			}
		}

		// filter out the corresponding script configMap volumes from the volumes of the current leader pod based on the scriptSpecSelectors defined by the user.
		for _, scriptSpecSelector := range scriptSpecSelectors {
			for _, scriptSpec := range componentDef.ScriptSpecs {
				findVolumes(scriptSpec, scriptSpecSelector)
			}
		}

		// find current pod's volumeMounts which mapped to volumes
		for _, volume := range volumes {
			for _, volumeMount := range pod.Spec.Containers[0].VolumeMounts {
				if volumeMount.Name == volume.Name {
					volumeMounts = append(volumeMounts, volumeMount)
					break
				}
			}
		}

		return volumes, volumeMounts
	}

	renderJob := func(switchoverSpec *appsv1alpha1.SwitchoverSpec, switchoverEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var (
			cmdExecutorConfig   *appsv1alpha1.CmdExecutorConfig
			scriptSpecSelectors []appsv1alpha1.ScriptSpecSelector
		)
		switch switchover.InstanceName {
		case constant.KBSwitchoverCandidateInstanceForAnyPod:
			if switchoverSpec.WithoutCandidate != nil {
				cmdExecutorConfig = switchoverSpec.WithoutCandidate.CmdExecutorConfig
				scriptSpecSelectors = switchoverSpec.WithoutCandidate.ScriptSpecSelectors
			}
		default:
			if switchoverSpec.WithCandidate != nil {
				cmdExecutorConfig = switchoverSpec.WithCandidate.CmdExecutorConfig
				scriptSpecSelectors = switchoverSpec.WithCandidate.ScriptSpecSelectors
			}
		}
		if cmdExecutorConfig == nil {
			return nil, errors.New("switchover action not found")
		}
		volumes, volumeMounts := renderJobPodVolumes(scriptSpecSelectors)

		// jobName named with generation to distinguish different switchover jobs.
		jobName := genSwitchoverJobName(cluster.Name, componentSpec.Name, cluster.Generation)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getSwitchoverCmdJobLabel(cluster.Name, componentSpec.Name),
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: cluster.Namespace,
						Name:      jobName,
					},
					Spec: corev1.PodSpec{
						Volumes:       volumes,
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:            constant.KBSwitchoverJobContainerName,
								Image:           cmdExecutorConfig.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         cmdExecutorConfig.Command,
								Args:            cmdExecutorConfig.Args,
								Env:             switchoverEnvs,
								VolumeMounts:    volumeMounts,
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

	switchoverEnvs, err := buildSwitchoverEnvs(ctx, cli, cluster, componentSpec, componentDef, switchover)
	if err != nil {
		return nil, err
	}
	job, err := renderJob(componentDef.SwitchoverSpec, switchoverEnvs)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// genSwitchoverJobName generates the switchover job name.
func genSwitchoverJobName(clusterName, componentName string, generation int64) string {
	return fmt.Sprintf("%s-%s-%s-%d", constant.KBSwitchoverJobNamePrefix, clusterName, componentName, generation)
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

// buildSwitchoverCandidateEnv builds the candidate instance name environment variable for the switchover job.
func buildSwitchoverCandidateEnv(
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	switchover *appsv1alpha1.Switchover) []corev1.EnvVar {
	svcName := strings.Join([]string{cluster.Name, componentSpec.Name, "headless"}, "-")
	if switchover == nil {
		return nil
	}
	if switchover.InstanceName == constant.KBSwitchoverCandidateInstanceForAnyPod {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name:  constant.KBSwitchoverCandidateName,
			Value: switchover.InstanceName,
		},
		{
			Name:  constant.KBSwitchoverCandidateFqdn,
			Value: fmt.Sprintf("%s.%s", switchover.InstanceName, svcName),
		},
	}
}

// buildSwitchoverEnvs builds the environment variables for the switchover job.
func buildSwitchoverEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	switchover *appsv1alpha1.Switchover) ([]corev1.EnvVar, error) {
	if componentSpec == nil || switchover == nil || componentDef.SwitchoverSpec == nil {
		return nil, errors.New("switchover spec not found")
	}
	// replace secret env and merge envs defined in SwitchoverSpec
	replaceSwitchoverConnCredentialEnv(cluster.Name, componentDef.SwitchoverSpec)
	var switchoverEnvs []corev1.EnvVar
	switch switchover.InstanceName {
	case constant.KBSwitchoverCandidateInstanceForAnyPod:
		if componentDef.SwitchoverSpec.WithoutCandidate != nil {
			switchoverEnvs = append(switchoverEnvs, componentDef.SwitchoverSpec.WithoutCandidate.CmdExecutorConfig.Env...)
		}
	default:
		if componentDef.SwitchoverSpec.WithCandidate != nil {
			switchoverEnvs = append(switchoverEnvs, componentDef.SwitchoverSpec.WithCandidate.CmdExecutorConfig.Env...)
		}
	}

	// inject the old primary info into the environment variable
	workloadEnvs, err := buildSwitchoverWorkloadEnvs(ctx, cli, cluster, componentSpec, componentDef)
	if err != nil {
		return nil, err
	}
	switchoverEnvs = append(switchoverEnvs, workloadEnvs...)

	// inject the candidate instance name into the environment variable if specify the candidate instance
	switchoverCandidateEnvs := buildSwitchoverCandidateEnv(cluster, componentSpec, switchover)
	switchoverEnvs = append(switchoverEnvs, switchoverCandidateEnvs...)
	return switchoverEnvs, nil
}

// replaceSwitchoverConnCredentialEnv replaces the connection credential environment variables for the switchover job.
func replaceSwitchoverConnCredentialEnv(clusterName string, switchoverSpec *appsv1alpha1.SwitchoverSpec) {
	if switchoverSpec == nil {
		return
	}
	namedValuesMap := intctrlcomputil.GetEnvReplacementMapForConnCredential(clusterName)
	replaceEnvVars := func(cmdExecutorConfig *appsv1alpha1.CmdExecutorConfig) {
		if cmdExecutorConfig != nil {
			cmdExecutorConfig.Env = intctrlcomputil.ReplaceSecretEnvVars(namedValuesMap, cmdExecutorConfig.Env)
		}
	}
	replaceEnvVars(switchoverSpec.WithCandidate.CmdExecutorConfig)
	replaceEnvVars(switchoverSpec.WithoutCandidate.CmdExecutorConfig)
}

// buildSwitchoverWorkloadEnvs builds the replication or consensus workload environment variables for the switchover job.
func buildSwitchoverWorkloadEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar
	pod, err := getPrimaryOrLeaderPod(ctx, cli, *cluster, componentSpec.Name, componentDef.Name)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("primary pod not found")
	}
	svcName := strings.Join([]string{cluster.Name, componentSpec.Name, "headless"}, "-")
	switch componentDef.WorkloadType {
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
	// add tht first container's environment variables of the primary pod
	workloadEnvs = append(workloadEnvs, pod.Spec.Containers[0].Env...)
	return workloadEnvs, nil
}

// getJobWithLabels gets the job list with the specified labels.
func getJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) ([]batchv1.Job, error) {
	jobList := &batchv1.JobList{}
	if err := cli.List(ctx, jobList, client.InNamespace(cluster.Namespace), matchLabels); err != nil {
		return nil, err
	}
	return jobList.Items, nil
}

// cleanJobWithLabels cleans up the job tasks with label that execute the switchover commands.
func cleanJobWithLabels(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	matchLabels client.MatchingLabels) error {
	jobList, err := getJobWithLabels(ctx, cli, cluster, matchLabels)
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

// cleanJobByName cleans up the job task by name that execute the switchover commands.
func cleanJobByName(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	jobName string) error {
	job := &batchv1.Job{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	if err := cli.Get(ctx, key, job); err != nil {
		return err
	}
	var ttl = int32(constant.KBJobTTLSecondsAfterFinished)
	patch := client.MergeFrom(job.DeepCopy())
	job.Spec.TTLSecondsAfterFinished = &ttl
	if err := cli.Patch(ctx, job, patch); err != nil {
		return err
	}
	return nil
}

// checkJobSucceed checks the result of job execution.
// Returns:
// - bool: whether job exist, true exist
// - error: any error that occurred during the handling
func checkJobSucceed(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	jobName string) error {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
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
			return intctrlutil.NewErrorf(intctrlutil.ErrorWaitCacheRefresh, "requeue to waiting for job %s finished.", key.Name)
		}
	} else {
		return errors.New("job check conditions status failed")
	}
}

// getPrimaryOrLeaderPod returns the leader or primary pod of the component.
func getPrimaryOrLeaderPod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpecName, compDefName string) (*corev1.Pod, error) {
	var (
		err     error
		podList *corev1.PodList
	)
	compDef, err := appsv1alpha1.GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(getSupportSwitchoverWorkload(), compDef.WorkloadType) {
		return nil, errors.New("component does not support switchover")
	}
	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		podList, err = components.GetComponentPodListWithRole(ctx, cli, cluster, compSpecName, constant.Primary)
	case appsv1alpha1.Consensus:
		podList, err = components.GetComponentPodListWithRole(ctx, cli, cluster, compSpecName, compDef.ConsensusSpec.Leader.Name)
	}
	if err != nil {
		return nil, err
	}
	if len(podList.Items) != 1 {
		return nil, errors.New("component pod list is empty or has more than one pod")
	}
	return &podList.Items[0], nil
}
