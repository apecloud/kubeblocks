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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// switchover constants
const (
	OpsReasonForSkipSwitchover = "SkipSwitchover"

	KBSwitchoverCandidateInstanceForAnyPod = "*"

	KBJobTTLSecondsAfterFinished  = 5
	KBSwitchoverJobLabelKey       = "kubeblocks.io/switchover-job"
	KBSwitchoverJobLabelValue     = "kb-switchover-job"
	KBSwitchoverJobNamePrefix     = "kb-switchover-job"
	KBSwitchoverJobContainerName  = "kb-switchover-job-container"
	KBSwitchoverCheckJobKey       = "CheckJob"
	KBSwitchoverCheckRoleLabelKey = "CheckRoleLabel"

	KBSwitchoverCandidateName = "KB_SWITCHOVER_CANDIDATE_NAME"
	KBSwitchoverCandidateFqdn = "KB_SWITCHOVER_CANDIDATE_FQDN"

	// KBSwitchoverReplicationPrimaryPodIP and the others Replication and Consensus switchover constants will be deprecated in the future, use KBSwitchoverLeaderPodIP instead.
	KBSwitchoverReplicationPrimaryPodIP   = "KB_REPLICATION_PRIMARY_POD_IP"
	KBSwitchoverReplicationPrimaryPodName = "KB_REPLICATION_PRIMARY_POD_NAME"
	KBSwitchoverReplicationPrimaryPodFqdn = "KB_REPLICATION_PRIMARY_POD_FQDN"
	KBSwitchoverConsensusLeaderPodIP      = "KB_CONSENSUS_LEADER_POD_IP"
	KBSwitchoverConsensusLeaderPodName    = "KB_CONSENSUS_LEADER_POD_NAME"
	KBSwitchoverConsensusLeaderPodFqdn    = "KB_CONSENSUS_LEADER_POD_FQDN"

	KBSwitchoverLeaderPodIP   = "KB_LEADER_POD_IP"
	KBSwitchoverLeaderPodName = "KB_LEADER_POD_NAME"
	KBSwitchoverLeaderPodFqdn = "KB_LEADER_POD_FQDN"
)

// needDoSwitchover checks whether we need to perform a switchover.
func needDoSwitchover(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizedComp *component.SynthesizedComponent,
	switchover *appsv1alpha1.Switchover) (bool, error) {
	// get the Pod object whose current role label is primary
	pod, err := getServiceableNWritablePod(ctx, cli, *cluster, *synthesizedComp)
	if err != nil {
		return false, err
	}
	if pod == nil {
		return false, nil
	}
	switch switchover.InstanceName {
	case KBSwitchoverCandidateInstanceForAnyPod:
		return true, nil
	default:
		podList, err := component.GetComponentPodList(ctx, cli, *cluster, synthesizedComp.Name)
		if err != nil {
			return false, err
		}
		podParent, _ := intctrlutil.ParseParentNameAndOrdinal(pod.Name)
		siParent, o := intctrlutil.ParseParentNameAndOrdinal(switchover.InstanceName)
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
	synthesizedComp *component.SynthesizedComponent,
	switchover *appsv1alpha1.Switchover) error {
	switchoverJob, err := renderSwitchoverCmdJob(reqCtx.Ctx, cli, cluster, synthesizedComp, switchover)
	if err != nil {
		return err
	}
	// check the current generation switchoverJob whether exist
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: switchoverJob.Name}
	exists, _ := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, key, &batchv1.Job{})
	if !exists {
		// check the previous generation switchoverJob whether exist
		ml := getSwitchoverCmdJobLabel(cluster.Name, synthesizedComp.Name)
		previousJobs, err := component.GetJobWithLabels(reqCtx.Ctx, cli, cluster, ml)
		if err != nil {
			return err
		}
		if len(previousJobs) > 0 {
			// delete the previous generation switchoverJob
			reqCtx.Log.V(1).Info("delete previous generation switchoverJob", "job", previousJobs[0].Name)
			if err := component.CleanJobWithLabels(reqCtx.Ctx, cli, cluster, ml); err != nil {
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
	synthesizedComp component.SynthesizedComponent,
	switchover *appsv1alpha1.Switchover,
	switchoverCondition *metav1.Condition) (bool, error) {
	if switchover == nil || switchoverCondition == nil {
		return false, nil
	}
	pod, err := getServiceableNWritablePod(ctx, cli, *cluster, synthesizedComp)
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
		if switchoverMessage.ComponentName != synthesizedComp.Name {
			continue
		}
		switch switchoverMessage.Switchover.InstanceName {
		case KBSwitchoverCandidateInstanceForAnyPod:
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
	synthesizedComp *component.SynthesizedComponent,
	switchover *appsv1alpha1.Switchover) (*batchv1.Job, error) {
	if synthesizedComp.LifecycleActions == nil || synthesizedComp.LifecycleActions.Switchover == nil || switchover == nil {
		return nil, errors.New("switchover spec not found")
	}
	pod, err := getServiceableNWritablePod(ctx, cli, *cluster, *synthesizedComp)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("serviceable and writable pod not found")
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
			for _, scriptSpec := range synthesizedComp.ScriptTemplates {
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

	renderJob := func(switchoverSpec *appsv1alpha1.ComponentSwitchover, switchoverEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var (
			cmdExecutorConfig   *appsv1alpha1.Action
			scriptSpecSelectors []appsv1alpha1.ScriptSpecSelector
		)
		switch switchover.InstanceName {
		case KBSwitchoverCandidateInstanceForAnyPod:
			if switchoverSpec.WithoutCandidate != nil && switchoverSpec.WithoutCandidate.Exec != nil {
				cmdExecutorConfig = switchoverSpec.WithoutCandidate
			}
		default:
			if switchoverSpec.WithCandidate != nil && switchoverSpec.WithCandidate.Exec != nil {
				cmdExecutorConfig = switchoverSpec.WithCandidate
			}
		}
		scriptSpecSelectors = append(scriptSpecSelectors, switchoverSpec.ScriptSpecSelectors...)
		if cmdExecutorConfig == nil {
			return nil, errors.New("switchover exec action not found")
		}
		volumes, volumeMounts := renderJobPodVolumes(scriptSpecSelectors)

		// jobName named with generation to distinguish different switchover jobs.
		jobName := genSwitchoverJobName(cluster.Name, synthesizedComp.Name, cluster.Generation)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getSwitchoverCmdJobLabel(cluster.Name, synthesizedComp.Name),
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
								Name:            KBSwitchoverJobContainerName,
								Image:           cmdExecutorConfig.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         cmdExecutorConfig.Exec.Command,
								Args:            cmdExecutorConfig.Exec.Args,
								Env:             switchoverEnvs,
								VolumeMounts:    volumeMounts,
							},
						},
					},
				},
			},
		}
		for i := range job.Spec.Template.Spec.Containers {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&job.Spec.Template.Spec.Containers[i])
		}
		if len(cluster.Spec.Tolerations) > 0 {
			job.Spec.Template.Spec.Tolerations = cluster.Spec.Tolerations
		}
		return job, nil
	}

	switchoverEnvs, err := buildSwitchoverEnvs(ctx, cli, cluster, synthesizedComp, switchover)
	if err != nil {
		return nil, err
	}
	job, err := renderJob(synthesizedComp.LifecycleActions.Switchover, switchoverEnvs)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// genSwitchoverJobName generates the switchover job name.
func genSwitchoverJobName(clusterName, componentName string, generation int64) string {
	return fmt.Sprintf("%s-%s-%s-%d", KBSwitchoverJobNamePrefix, clusterName, componentName, generation)
}

// getSwitchoverCmdJobLabel gets the labels for job that execute the switchover commands.
func getSwitchoverCmdJobLabel(clusterName, componentName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
		KBSwitchoverJobLabelKey:         KBSwitchoverJobLabelValue,
	}
}

// buildSwitchoverCandidateEnv builds the candidate instance name environment variable for the switchover job.
func buildSwitchoverCandidateEnv(
	cluster *appsv1alpha1.Cluster,
	componentName string,
	switchover *appsv1alpha1.Switchover) []corev1.EnvVar {
	svcName := strings.Join([]string{cluster.Name, componentName, "headless"}, "-")
	if switchover == nil {
		return nil
	}
	if switchover.InstanceName == KBSwitchoverCandidateInstanceForAnyPod {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name:  KBSwitchoverCandidateName,
			Value: switchover.InstanceName,
		},
		{
			Name:  KBSwitchoverCandidateFqdn,
			Value: fmt.Sprintf("%s.%s", switchover.InstanceName, svcName),
		},
	}
}

// buildSwitchoverEnvs builds the environment variables for the switchover job.
func buildSwitchoverEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *component.SynthesizedComponent,
	switchover *appsv1alpha1.Switchover) ([]corev1.EnvVar, error) {
	if synthesizeComp == nil || synthesizeComp.LifecycleActions == nil ||
		synthesizeComp.LifecycleActions.Switchover == nil || switchover == nil {
		return nil, errors.New("switchover spec not found")
	}

	if synthesizeComp.LifecycleActions.Switchover.WithCandidate == nil && synthesizeComp.LifecycleActions.Switchover.WithoutCandidate == nil {
		return nil, errors.New("switchover spec withCandidate and withoutCandidate can't be nil at the same time")
	}

	// replace secret env and merge envs defined in SwitchoverSpec
	replaceSwitchoverConnCredentialEnv(synthesizeComp.LifecycleActions.Switchover, cluster.Name, synthesizeComp.Name)
	var switchoverEnvs []corev1.EnvVar
	switch switchover.InstanceName {
	case KBSwitchoverCandidateInstanceForAnyPod:
		if synthesizeComp.LifecycleActions.Switchover.WithoutCandidate != nil {
			switchoverEnvs = append(switchoverEnvs, synthesizeComp.LifecycleActions.Switchover.WithoutCandidate.Env...)
		}
	default:
		if synthesizeComp.LifecycleActions.Switchover.WithCandidate != nil {
			switchoverEnvs = append(switchoverEnvs, synthesizeComp.LifecycleActions.Switchover.WithCandidate.Env...)
		}
	}

	// inject the old primary info into the environment variable
	workloadEnvs, err := buildSwitchoverWorkloadEnvs(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return nil, err
	}
	switchoverEnvs = append(switchoverEnvs, workloadEnvs...)

	// inject the candidate instance name into the environment variable if specify the candidate instance
	switchoverCandidateEnvs := buildSwitchoverCandidateEnv(cluster, synthesizeComp.Name, switchover)
	switchoverEnvs = append(switchoverEnvs, switchoverCandidateEnvs...)
	return switchoverEnvs, nil
}

// replaceSwitchoverConnCredentialEnv replaces the connection credential environment variables for the switchover job.
func replaceSwitchoverConnCredentialEnv(switchoverSpec *appsv1alpha1.ComponentSwitchover, clusterName, componentName string) {
	if switchoverSpec == nil {
		return
	}
	connCredentialMap := component.GetEnvReplacementMapForConnCredential(clusterName)
	replaceEnvVars := func(action *appsv1alpha1.Action) {
		if action != nil {
			action.Env = component.ReplaceSecretEnvVars(connCredentialMap, action.Env)
		}
	}
	replaceEnvVars(switchoverSpec.WithCandidate)
	replaceEnvVars(switchoverSpec.WithoutCandidate)
}

// buildSwitchoverWorkloadEnvs builds the replication or consensus workload environment variables for the switchover job.
func buildSwitchoverWorkloadEnvs(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *component.SynthesizedComponent) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar
	pod, err := getServiceableNWritablePod(ctx, cli, *cluster, *synthesizeComp)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, errors.New("serviceable and writable pod not found")
	}
	svcName := strings.Join([]string{cluster.Name, synthesizeComp.Name, "headless"}, "-")

	workloadEnvs = append(workloadEnvs, []corev1.EnvVar{
		{
			Name:  KBSwitchoverLeaderPodIP,
			Value: pod.Status.PodIP,
		},
		{
			Name:  KBSwitchoverLeaderPodName,
			Value: pod.Name,
		},
		{
			Name:  KBSwitchoverLeaderPodFqdn,
			Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
		},
	}...)

	// TODO(xingran): backward compatibility for the old env based on workloadType, it will be removed in the future
	workloadEnvs = append(workloadEnvs, []corev1.EnvVar{
		{
			Name:  KBSwitchoverReplicationPrimaryPodIP,
			Value: pod.Status.PodIP,
		},
		{
			Name:  KBSwitchoverReplicationPrimaryPodName,
			Value: pod.Name,
		},
		{
			Name:  KBSwitchoverReplicationPrimaryPodFqdn,
			Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
		},
		{
			Name:  KBSwitchoverConsensusLeaderPodIP,
			Value: pod.Status.PodIP,
		},
		{
			Name:  KBSwitchoverConsensusLeaderPodName,
			Value: pod.Name,
		},
		{
			Name:  KBSwitchoverConsensusLeaderPodFqdn,
			Value: fmt.Sprintf("%s.%s", pod.Name, svcName),
		},
	}...)

	// add the first container's environment variables of the primary pod
	workloadEnvs = append(workloadEnvs, pod.Spec.Containers[0].Env...)
	return workloadEnvs, nil
}

// getServiceableNWritablePod returns the serviceable and writable pod of the component.
func getServiceableNWritablePod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, synthesizeComp component.SynthesizedComponent) (*corev1.Pod, error) {
	if synthesizeComp.Roles == nil {
		return nil, errors.New("component does not support switchover")
	}

	targetRole := ""
	for _, role := range synthesizeComp.Roles {
		if role.Serviceable && role.Writable {
			if targetRole != "" {
				return nil, errors.New("component has more than role is serviceable and writable, does not support switchover")
			}
			targetRole = role.Name
		}
	}
	if targetRole == "" {
		return nil, errors.New("component has no role is serviceable and writable, does not support switchover")
	}

	podList, err := component.GetComponentPodListWithRole(ctx, cli, cluster, synthesizeComp.Name, targetRole)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) != 1 {
		return nil, errors.New("component pod list is empty or has more than one serviceable and writable pod")
	}
	return &podList.Items[0], nil
}
