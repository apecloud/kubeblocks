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

package component

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ReconcileCompPostStart reconciles the component-level postStart command.
func ReconcileCompPostStart(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent) error {
	needPostStart, err := needDoPostStart(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return err
	}
	if !needPostStart {
		return nil
	}

	if err := createPostStartJobIfNotExist(ctx, cli, cluster, synthesizeComp); err != nil {
		return err
	}

	jobName := genPostStartJobName(cluster.Name, synthesizeComp.Name)
	err = CheckJobSucceed(ctx, cli, cluster, jobName)
	if err != nil {
		return err
	}

	// job executed successfully, add the label to the rsm object to indicate that the postStart has been executed and delete the job
	if err := setPostStartDoneLabel(ctx, cli, cluster, synthesizeComp); err != nil {
		return err
	}

	// clean up the job
	if err := CleanJobByName(ctx, cli, cluster, jobName); err != nil {
		return err
	}

	return nil
}

func needDoPostStart(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent) (bool, error) {
	// if the component does not have a postStartSpec, skip it
	if synthesizeComp.PostStartSpec == nil {
		return false, nil
	}

	// determine whether the component has undergone postStart by examining the annotation of the rsm object
	rsm, err := getUnderlyingRSMObj(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return false, err
	}
	if rsm.Annotations == nil {
		return false, nil
	}
	_, ok := rsm.Annotations[constant.KBPostStartDoneLabelKey]
	if ok {
		return false, nil
	}
	return true, nil
}

// createPostStartJobIfNotExist creates a job to execute component-level post start command, each component only has a corresponding job.
func createPostStartJobIfNotExist(ctx context.Context,
	cli client.Client, cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent) error {
	if synthesizeComp.PostStartSpec == nil {
		return nil
	}

	postStartJob, err := renderPostStartCmdJob(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return err
	}
	// check the postStartJob whether exist
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: postStartJob.Name}
	exist, _ := intctrlutil.CheckResourceExists(ctx, cli, key, &batchv1.Job{})
	if exist {
		return nil
	}

	// set the controller reference
	if err := intctrlutil.SetControllerReference(cluster, postStartJob); err != nil {
		return err
	}

	// create the postStartJob if not exist
	if err := cli.Create(ctx, postStartJob); err != nil {
		return err
	}
	return nil
}

// renderPostStartCmdJob renders and creates the postStart command job.
func renderPostStartCmdJob(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent) (*batchv1.Job, error) {
	if synthesizeComp == nil || synthesizeComp.PostStartSpec == nil {
		return nil, errors.New("PostStart spec not found")
	}
	podList, err := getComponentPodList(ctx, cli, *cluster, synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if podList == nil || len(podList.Items) == 0 {
		return nil, errors.New("component pods not found")
	}
	tplPod := podList.Items[0]

	renderJobPodVolumes := func(scriptSpecSelectors []appsv1alpha1.ScriptSpecSelector) ([]corev1.Volume, []corev1.VolumeMount) {
		volumes := make([]corev1.Volume, 0)
		volumeMounts := make([]corev1.VolumeMount, 0)

		// find current pod's volume which mapped to configMapRefs
		findVolumes := func(tplSpec appsv1alpha1.ComponentTemplateSpec, scriptSpecSelector appsv1alpha1.ScriptSpecSelector) {
			if tplSpec.Name != scriptSpecSelector.Name {
				return
			}
			for _, podVolume := range tplPod.Spec.Volumes {
				if podVolume.Name == tplSpec.VolumeName {
					volumes = append(volumes, podVolume)
					break
				}
			}
		}

		// filter out the corresponding script configMap volumes from the volumes of the current leader pod based on the scriptSpecSelectors defined by the user.
		for _, scriptSpecSelector := range scriptSpecSelectors {
			for _, scriptSpec := range synthesizeComp.ScriptTemplates {
				findVolumes(scriptSpec, scriptSpecSelector)
			}
		}

		// find current pod's volumeMounts which mapped to volumes
		for _, volume := range volumes {
			for _, container := range tplPod.Spec.Containers {
				for _, volumeMount := range container.VolumeMounts {
					if volumeMount.Name == volume.Name {
						volumeMounts = append(volumeMounts, volumeMount)
						break
					}
				}
			}
		}

		return volumes, volumeMounts
	}

	renderJob := func(postStartSpec *appsv1alpha1.PostStartAction, postStartEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var (
			cmdExecutorConfig   = postStartSpec.CmdExecutorConfig
			scriptSpecSelectors = postStartSpec.ScriptSpecSelectors
		)
		if cmdExecutorConfig == nil {
			return nil, errors.New("postStart action not found")
		}
		volumes, volumeMounts := renderJobPodVolumes(scriptSpecSelectors)
		jobName := genPostStartJobName(cluster.Name, synthesizeComp.Name)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getPostStartCmdJobLabel(cluster.Name, synthesizeComp.Name),
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
								Name:            constant.KBPostStartJobContainerName,
								Image:           cmdExecutorConfig.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         cmdExecutorConfig.Command,
								Args:            cmdExecutorConfig.Args,
								Env:             postStartEnvs,
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

	postStartEnvs, err := buildPostStartEnvs(cluster, synthesizeComp, &tplPod)
	if err != nil {
		return nil, err
	}

	job, err := renderJob(synthesizeComp.PostStartSpec, postStartEnvs)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// buildPostStartEnvs builds the postStart command job envs.
func buildPostStartEnvs(cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent,
	tplPod *corev1.Pod) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar

	if synthesizeComp != nil && synthesizeComp.PostStartSpec != nil && synthesizeComp.PostStartSpec.CmdExecutorConfig != nil {
		workloadEnvs = append(workloadEnvs, synthesizeComp.PostStartSpec.CmdExecutorConfig.Env...)
	}

	if tplPod != nil && len(tplPod.Spec.Containers) > 0 {
		// add tht first container's environment variables of the template pod
		workloadEnvs = append(workloadEnvs, tplPod.Spec.Containers[0].Env...)
	}

	compEnvs := genClusterComponentEnv(cluster)
	if len(compEnvs) > 0 {
		workloadEnvs = append(workloadEnvs, compEnvs...)
	}

	return workloadEnvs, nil
}

// genClusterComponentEnv generates the cluster component relative envs.
func genClusterComponentEnv(cluster *appsv1alpha1.Cluster) []corev1.EnvVar {
	if cluster == nil || cluster.Spec.ComponentSpecs == nil {
		return nil
	}
	compList := make([]string, 0, len(cluster.Spec.ComponentSpecs))
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compList = append(compList, compSpec.Name)
	}
	return []corev1.EnvVar{
		{
			Name:  constant.KBPostStartClusterCompList,
			Value: strings.Join(compList, ","),
		},
	}
}

// genPostStartJobName generates the switchover job name.
func genPostStartJobName(clusterName, componentName string) string {
	return fmt.Sprintf("%s-%s-%s", constant.KBPostStartJobNamePrefix, clusterName, componentName)
}

// getPostStartCmdJobLabel gets the labels for job that execute the postStart commands.
func getPostStartCmdJobLabel(clusterName, componentName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.KBPostStartJobLabelKey: constant.KBPostStartJobLabelValue,
	}
}

// getComponentMatchLabels gets the labels for matching the cluster component
func getComponentMatchLabels(clusterName, componentName string) map[string]string {
	return client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
}

// getComponentPodList gets the pod list by cluster and componentName
func getComponentPodList(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		client.MatchingLabels(getComponentMatchLabels(cluster.Name, componentName)))
	return podList, err
}

// getUnderlyingRSMObj gets the underlying ReplicatedStateMachine object of component.
func getUnderlyingRSMObj(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	rsmKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      fmt.Sprintf("%s-%s", cluster.Name, synthesizeComp.Name),
	}
	rsm := &workloads.ReplicatedStateMachine{}
	if err := cli.Get(ctx, rsmKey, rsm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return rsm, nil
}

// setPostStartDoneLabel sets the postStart done annotation to the underlying ReplicatedStateMachine object of component.
func setPostStartDoneLabel(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent) error {
	rsm, err := getUnderlyingRSMObj(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return err
	}
	rmsObj := rsm.DeepCopy()
	if rsm.Annotations == nil {
		rsm.Annotations = make(map[string]string)
	}
	timeStr := time.Now().Format(time.RFC3339Nano)
	rsm.Annotations[constant.KBPostStartDoneLabelKey] = timeStr
	if err := cli.Patch(ctx, rsm, client.MergeFrom(rmsObj)); err != nil {
		return err
	}
	return nil
}
