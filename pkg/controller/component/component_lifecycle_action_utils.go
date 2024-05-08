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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// LifeCycleActionType represents the lifecycle action type.
type LifeCycleActionType string

const (
	// PostProvisionAction represents the post-provision action.
	PostProvisionAction LifeCycleActionType = "postProvision"

	// PreTerminateAction represents the pre-terminate action.
	PreTerminateAction LifeCycleActionType = "preTerminate"
)

// component lifecycle action constants
const (
	kbLifecycleActionJobContainerName = "kb-lifecycle-action-job"

	kbLifecycleActionClusterPodNameList     = "KB_CLUSTER_POD_NAME_LIST"
	kbLifecycleActionClusterPodIPList       = "KB_CLUSTER_POD_IP_LIST"
	kbLifecycleActionClusterPodHostNameList = "KB_CLUSTER_POD_HOST_NAME_LIST"
	kbLifecycleActionClusterPodHostIPList   = "KB_CLUSTER_POD_HOST_IP_LIST"

	kbLifecycleActionClusterCompPodNameList     = "KB_CLUSTER_COMPONENT_POD_NAME_LIST"
	kbLifecycleActionClusterCompPodIPList       = "KB_CLUSTER_COMPONENT_POD_IP_LIST"
	kbLifecycleActionClusterCompPodHostNameList = "KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"
	kbLifecycleActionClusterCompPodHostIPList   = "KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"

	// kbLifecycleActionClusterCompIsScalingIn indicates whether current component is scaling in
	kbLifecycleActionClusterCompIsScalingIn = "KB_CLUSTER_COMPONENT_IS_SCALING_IN"
	// kbLifecycleActionClusterCompList indicates all the components of the cluster
	kbLifecycleActionClusterCompList = "KB_CLUSTER_COMPONENT_LIST"
	// kbLifecycleActionClusterCompDeletingList indicates the components list which are deleting
	kbLifecycleActionClusterCompDeletingList = "KB_CLUSTER_COMPONENT_DELETING_LIST"
	// kbLifecycleActionClusterCompUndeletedList indicates the components list which are not deleted
	kbLifecycleActionClusterCompUndeletedList = "KB_CLUSTER_COMPONENT_UNDELETED_LIST"
)

// createActionJobIfNotExist creates a job to execute component-level custom lifecycle action command, each component only has a corresponding job.
func createActionJobIfNotExist(ctx context.Context,
	cli client.Reader,
	graphCli model.GraphClient,
	dag *graph.DAG,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *SynthesizedComponent,
	actionType LifeCycleActionType) (*batchv1.Job, error) {
	// check if the lifecycle action definition exists
	actionExist, _ := checkLifeCycleAction(synthesizeComp, actionType)
	if !actionExist {
		return nil, nil
	}

	renderJob, err := renderActionCmdJob(ctx, cli, cluster, synthesizeComp, actionType)
	if err != nil {
		return nil, err
	}

	key := types.NamespacedName{Namespace: cluster.Namespace, Name: renderJob.Name}
	existJob := &batchv1.Job{}
	exist, _ := intctrlutil.CheckResourceExists(ctx, cli, key, existJob)
	if exist {
		return existJob, nil
	}

	// set the controller reference
	if err := intctrlutil.SetControllerReference(comp, renderJob); err != nil {
		return renderJob, err
	}

	// create the job if not exist
	graphCli.Create(dag, renderJob)
	return renderJob, nil
}

// renderActionCmdJob renders and creates the action command job.
func renderActionCmdJob(ctx context.Context,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent,
	actionType LifeCycleActionType) (*batchv1.Job, error) {

	exist, action := checkLifeCycleAction(synthesizeComp, actionType)
	if !exist {
		return nil, fmt.Errorf("lifecycle action %s custom handler not found", actionType)
	}
	if action.Exec == nil {
		return nil, fmt.Errorf("lifecycle action %s custom handler only support exec command by now, please check your customHandler spec", actionType)
	}

	podList, err := GetComponentPodList(ctx, cli, *cluster, synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if podList == nil || len(podList.Items) == 0 {
		return nil, errors.New("component pods not found")
	}
	pods := podList.Items
	tplPod := podList.Items[0]

	renderJobPodVolumes := func() ([]corev1.Volume, []corev1.VolumeMount) {
		volumes := make([]corev1.Volume, 0)
		volumeMounts := make([]corev1.VolumeMount, 0)

		// find current pod's volume which mapped to scriptsTemplates
		findVolumes := func(tplSpec appsv1alpha1.ComponentTemplateSpec) {
			for _, podVolume := range tplPod.Spec.Volumes {
				if podVolume.Name == tplSpec.VolumeName {
					volumes = append(volumes, podVolume)
					break
				}
			}
		}

		for _, scriptSpec := range synthesizeComp.ScriptTemplates {
			findVolumes(scriptSpec)
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

	renderJob := func(customAction *appsv1alpha1.Action, envs []corev1.EnvVar, envFroms []corev1.EnvFromSource) (*batchv1.Job, error) {
		volumes, volumeMounts := renderJobPodVolumes()
		jobName, err := genActionJobName(cluster.Name, synthesizeComp.Name, actionType)
		if err != nil {
			return nil, err
		}
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getActionCmdJobLabels(cluster.Name, synthesizeComp.Name, actionType),
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: pointer.Int32(2),
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
								Name:            kbLifecycleActionJobContainerName,
								Image:           customAction.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         customAction.Exec.Command,
								Args:            customAction.Exec.Args,
								Env:             envs,
								EnvFrom:         envFroms,
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
		for i := range job.Spec.Template.Spec.Containers {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&job.Spec.Template.Spec.Containers[i])
		}
		if customAction.RetryPolicy != nil && customAction.RetryPolicy.MaxRetries > 0 {
			job.Spec.BackoffLimit = pointer.Int32(int32(customAction.RetryPolicy.MaxRetries))
		}
		return job, nil
	}

	envs, envFroms, err := buildLifecycleActionEnvs(ctx, cli, cluster, synthesizeComp, action, pods, &tplPod)
	if err != nil {
		return nil, err
	}

	job, err := renderJob(action, envs, envFroms)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// buildLifecycleActionEnvs builds the environment variables for lifecycle actions.
func buildLifecycleActionEnvs(ctx context.Context,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent,
	action *appsv1alpha1.Action,
	pods []corev1.Pod,
	tplPod *corev1.Pod) ([]corev1.EnvVar, []corev1.EnvFromSource, error) {
	var workloadEnvs []corev1.EnvVar
	var workloadEnvFroms []corev1.EnvFromSource

	// add custom environment variables of the lifecycle action
	if action != nil {
		workloadEnvs = append(workloadEnvs, action.Env...)
	}

	if tplPod != nil && len(tplPod.Spec.Containers) > 0 {
		// add tht first container's environment variables of the template pod
		workloadEnvs = append(workloadEnvs, tplPod.Spec.Containers[0].Env...)
		workloadEnvFroms = append(workloadEnvFroms, tplPod.Spec.Containers[0].EnvFrom...)
	}

	genEnvs, err := genClusterNComponentEnvs(ctx, cli, cluster, synthesizeComp, pods)
	if err != nil {
		return nil, nil, err
	}
	if len(genEnvs) > 0 {
		workloadEnvs = append(workloadEnvs, genEnvs...)
	}

	return workloadEnvs, workloadEnvFroms, nil
}

// genClusterNComponentEnvs generates the cluster and component relative envs.
func genClusterNComponentEnvs(ctx context.Context,
	cli client.Reader,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent,
	pods []corev1.Pod) ([]corev1.EnvVar, error) {
	if cluster == nil || (cluster.Spec.ComponentSpecs == nil && cluster.Spec.ShardingSpecs == nil) {
		return nil, nil
	}

	envs := make([]corev1.EnvVar, 0)
	podEnvs, err := genComponentPodEnvs(pods)
	if err != nil {
		return nil, err
	}
	envs = append(envs, podEnvs...)

	compList, err := ListClusterComponents(ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	compEnvs, err := genComponentEnvs(synthesizeComp, compList)
	if err != nil {
		return nil, err
	}
	envs = append(envs, compEnvs...)

	clusterEnvs, err := genClusterEnvs(ctx, cli, cluster, compList)
	if err != nil {
		return nil, err
	}
	envs = append(envs, clusterEnvs...)

	return envs, nil
}

// genComponentEnvs generates the component relative envs.
func genComponentEnvs(synthesizeComp *SynthesizedComponent, components []appsv1alpha1.Component) ([]corev1.EnvVar, error) {
	compEnvs := make([]corev1.EnvVar, 0)
	for _, comp := range components {
		if comp.Name == synthesizeComp.FullCompName {
			scaleInVal, ok := comp.Annotations[constant.ComponentScaleInAnnotationKey]
			if ok {
				compEnvs = append(compEnvs, corev1.EnvVar{
					Name:  kbLifecycleActionClusterCompIsScalingIn,
					Value: scaleInVal,
				})
			}
		}
	}
	return compEnvs, nil
}

// genComponentPodEnvs generates the component pod relative envs.
func genComponentPodEnvs(compPods []corev1.Pod) ([]corev1.EnvVar, error) {
	compEnvs := make([]corev1.EnvVar, 0)
	compPodNameList := make([]string, 0, len(compPods))
	compPodIPList := make([]string, 0, len(compPods))
	compPodHostNameList := make([]string, 0, len(compPods))
	compPodHostIPList := make([]string, 0, len(compPods))

	for _, pod := range compPods {
		compPodNameList = append(compPodNameList, pod.Name)
		compPodIPList = append(compPodIPList, pod.Status.PodIP)
		compPodHostNameList = append(compPodHostNameList, pod.Spec.NodeName)
		compPodHostIPList = append(compPodHostIPList, pod.Status.HostIP)
	}
	compEnvs = append(compEnvs, []corev1.EnvVar{
		{
			Name:  kbLifecycleActionClusterCompPodNameList,
			Value: strings.Join(compPodNameList, ","),
		},
		{
			Name:  kbLifecycleActionClusterCompPodIPList,
			Value: strings.Join(compPodIPList, ","),
		},
		{
			Name:  kbLifecycleActionClusterCompPodHostNameList,
			Value: strings.Join(compPodHostNameList, ","),
		},
		{
			Name:  kbLifecycleActionClusterCompPodHostIPList,
			Value: strings.Join(compPodHostIPList, ","),
		}}...)

	return compEnvs, nil
}

// genClusterEnvs generates the cluster scope relative envs.
func genClusterEnvs(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster, components []appsv1alpha1.Component) ([]corev1.EnvVar, error) {
	clusterPods := make([]corev1.Pod, 0)
	compNames := make([]string, len(components))
	deletingCompNames := make([]string, len(components))
	undeletedCompNames := make([]string, len(components))
	for _, comp := range components {
		compShortName, err := ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		compPodList, err := GetComponentPodList(ctx, cli, *cluster, compShortName)
		if err != nil {
			return nil, err
		}
		if compPodList == nil || len(compPodList.Items) == 0 {
			continue
		}
		clusterPods = append(clusterPods, compPodList.Items...)
		compNames = append(compNames, compShortName)
		if model.IsObjectDeleting(&comp) {
			deletingCompNames = append(deletingCompNames, compShortName)
		} else {
			undeletedCompNames = append(undeletedCompNames, compShortName)
		}
	}
	clusterEnvs := make([]corev1.EnvVar, 0)
	clusterPodNameList := make([]string, 0, len(clusterPods))
	clusterPodIPList := make([]string, 0, len(clusterPods))
	clusterPodHostNameList := make([]string, 0, len(clusterPods))
	clusterPodHostIPList := make([]string, 0, len(clusterPods))

	for _, pod := range clusterPods {
		clusterPodNameList = append(clusterPodNameList, pod.Name)
		clusterPodIPList = append(clusterPodIPList, pod.Status.PodIP)
		clusterPodHostNameList = append(clusterPodHostNameList, pod.Spec.NodeName)
		clusterPodHostIPList = append(clusterPodHostIPList, pod.Status.HostIP)
	}
	clusterEnvs = append(clusterEnvs, []corev1.EnvVar{
		{
			Name:  kbLifecycleActionClusterCompList,
			Value: strings.Join(compNames, ","),
		},
		{
			Name:  kbLifecycleActionClusterCompDeletingList,
			Value: strings.Join(deletingCompNames, ","),
		},
		{
			Name:  kbLifecycleActionClusterCompUndeletedList,
			Value: strings.Join(undeletedCompNames, ","),
		},
	}...)
	clusterEnvs = append(clusterEnvs, []corev1.EnvVar{
		{
			Name:  kbLifecycleActionClusterPodNameList,
			Value: strings.Join(clusterPodNameList, ","),
		},
		{
			Name:  kbLifecycleActionClusterPodIPList,
			Value: strings.Join(clusterPodIPList, ","),
		},
		{
			Name:  kbLifecycleActionClusterPodHostNameList,
			Value: strings.Join(clusterPodHostNameList, ","),
		},
		{
			Name:  kbLifecycleActionClusterPodHostIPList,
			Value: strings.Join(clusterPodHostIPList, ","),
		}}...)

	return clusterEnvs, nil
}

// needDoActionByCheckingJobNAnnotation checks if the action needs to be executed by checking the job and annotation.
func needDoActionByCheckingJobNAnnotation(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent, actionType LifeCycleActionType) (bool, error) {
	if comp.Annotations == nil {
		return true, nil
	}
	// determine whether the component has undergone action by checking the annotation and job
	jobName, _ := genActionJobName(cluster.Name, synthesizeComp.Name, actionType)
	jobExist := checkActionJobExist(ctx, cli, cluster, jobName)
	finishAnnotationExist := checkActionDoneAnnotationExist(*cluster, *comp, *synthesizeComp, actionType)
	if finishAnnotationExist && !jobExist {
		// if the annotation has been set and the job does not exist, it means that the action has finished, so skip it
		return false, nil
	}
	return true, nil
}

func checkActionJobExist(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster, jobName string) bool {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	existJob := &batchv1.Job{}
	exist, _ := intctrlutil.CheckResourceExists(ctx, cli, key, existJob)
	return exist
}

// setActionDoneAnnotation sets the action done annotation for the component.
func setActionDoneAnnotation(graphCli model.GraphClient, comp *appsv1alpha1.Component, dag *graph.DAG, actionType LifeCycleActionType) error {
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	var actionDoneKey string
	switch actionType {
	case PostProvisionAction:
		actionDoneKey = kbCompPostProvisionDoneKey
	case PreTerminateAction:
		actionDoneKey = kbCompPreTerminateDoneKey
	default:
		return errors.New("unsupported lifecycle action type")
	}
	_, ok := comp.Annotations[actionDoneKey]
	if ok {
		return nil
	}
	compObj := comp.DeepCopy()
	timeStr := time.Now().Format(time.RFC3339Nano)
	comp.Annotations[actionDoneKey] = timeStr
	graphCli.Update(dag, compObj, comp, &model.ReplaceIfExistingOption{})
	return nil
}

// cleanActionJob cleans the action job by name.
func cleanActionJob(ctx context.Context,
	cli client.Reader,
	dag *graph.DAG,
	cluster *appsv1alpha1.Cluster,
	comp appsv1alpha1.Component,
	synthesizeComp SynthesizedComponent,
	actionType LifeCycleActionType,
	jobName string) error {
	if cluster.Annotations == nil || comp.Annotations == nil {
		return errors.New("cluster or component annotations not found")
	}
	// check action done annotation has been set
	if !checkActionDoneAnnotationExist(*cluster, comp, synthesizeComp, actionType) {
		return fmt.Errorf("cluster %s %s done annotation has not been set", cluster.Name, actionType)
	}
	return CleanJobByNameWithDAG(ctx, cli, dag, cluster, jobName)
}

// checkActionDoneAnnotationExist checks if the action done annotation exists.
func checkActionDoneAnnotationExist(cluster appsv1alpha1.Cluster,
	comp appsv1alpha1.Component,
	synthesizeComp SynthesizedComponent,
	actionType LifeCycleActionType) bool {
	if cluster.Annotations == nil || comp.Annotations == nil {
		return false
	}
	var actionDoneKey string
	switch actionType {
	case PostProvisionAction:
		// TODO(xingran): for backward compatibility before KubeBlocks v0.8.0, check the annotation of the cluster object first, it will be deprecated in the future
		compPostStartDoneKey := fmt.Sprintf(kbCompPostStartDoneKeyPattern, fmt.Sprintf("%s-%s", cluster.Name, synthesizeComp.Name))
		_, ok := cluster.Annotations[compPostStartDoneKey]
		if ok {
			return true
		}
		actionDoneKey = kbCompPostProvisionDoneKey
	case PreTerminateAction:
		actionDoneKey = kbCompPreTerminateDoneKey
	default:
		return false
	}
	_, ok := comp.Annotations[actionDoneKey]
	return ok
}

// checkLifeCycleAction checks if the lifecycle action definition exists and returns the action.
func checkLifeCycleAction(synthesizeComp *SynthesizedComponent, actionType LifeCycleActionType) (bool, *appsv1alpha1.Action) {
	if synthesizeComp == nil || synthesizeComp.LifecycleActions == nil {
		return false, nil
	}

	var action *appsv1alpha1.Action
	switch actionType {
	case PostProvisionAction:
		if actions := synthesizeComp.LifecycleActions.PostProvision; actions != nil {
			action = actions.CustomHandler
		}
	case PreTerminateAction:
		if actions := synthesizeComp.LifecycleActions.PreTerminate; actions != nil {
			action = actions.CustomHandler
		}
	default:
		return false, nil
	}

	return action != nil, action
}

// genActionJobName generates the action job name.
func genActionJobName(clusterName, componentName string, actionType LifeCycleActionType) (string, error) {
	switch actionType {
	case PostProvisionAction:
		return fmt.Sprintf("%s-%s-%s", kbPostProvisionJobNamePrefix, clusterName, componentName), nil
	case PreTerminateAction:
		return fmt.Sprintf("%s-%s-%s", kbPreTerminateJobNamePrefix, clusterName, componentName), nil
	}
	return "", errors.New("unsupported lifecycle action type")
}

// getActionCmdJobLabels gets the labels for job that execute the action commands.
func getActionCmdJobLabels(clusterName, componentName string, actionType LifeCycleActionType) map[string]string {
	labels := map[string]string{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
	switch actionType {
	case PostProvisionAction:
		labels[kbPostProvisionJobLabelKey] = kbPostProvisionJobLabelValue
	case PreTerminateAction:
		labels[kbPreTerminateJobLabelKey] = kbPreTerminateJobLabelValue
	}
	return labels
}
