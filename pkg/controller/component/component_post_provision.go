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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// post-provision constants
const (
	kbPostProvisionJobLabelKey      = "kubeblocks.io/post-provision-job"
	kbPostProvisionJobLabelValue    = "kb-post-provision-job"
	kbPostProvisionJobNamePrefix    = "kb-post-provision-job"
	kbPostProvisionJobContainerName = "kb-post-provision-job-container"

	// kbCompPostStartDoneKeyPattern will be deprecated after KubeBlocks v0.8.0 and use kbCompPostProvisionDoneKey instead
	kbCompPostStartDoneKeyPattern = "kubeblocks.io/%s-poststart-done"
	// kbCompPostProvisionDoneKey is used to mark the component postProvision job is done
	kbCompPostProvisionDoneKey = "kubeblocks.io/post-provision-done"

	kbPostProvisionClusterCompList            = "KB_CLUSTER_COMPONENT_LIST"
	kbPostProvisionClusterCompPodNameList     = "KB_CLUSTER_COMPONENT_POD_NAME_LIST"
	kbPostProvisionClusterCompPodIPList       = "KB_CLUSTER_COMPONENT_POD_IP_LIST"
	kbPostProvisionClusterCompPodHostNameList = "KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"
	kbPostProvisionClusterCompPodHostIPList   = "KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"
)

// ReconcileCompPostProvision reconciles the component-level postProvision command.
func ReconcileCompPostProvision(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *SynthesizedComponent,
	dag *graph.DAG) error {
	needPostProvision, err := needDoPostProvision(ctx, cli, cluster, comp, synthesizeComp)
	if err != nil {
		return err
	}
	if !needPostProvision {
		return nil
	}

	job, err := createPostProvisionJobIfNotExist(ctx, cli, cluster, comp, synthesizeComp)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	err = CheckJobSucceed(ctx, cli, cluster, job.Name)
	if err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorWaitCacheRefresh) {
			return nil
		}
		return err
	}

	// job executed successfully, add the annotation to indicate that the postProvision has been executed and delete the job
	compOrig := comp.DeepCopy()
	if err := setPostProvisionDoneAnnotation(cli, comp, dag); err != nil {
		return err
	}

	// clean up the postProvision job
	if err := cleanPostProvisionJob(ctx, cli, cluster, *compOrig, *synthesizeComp, job.Name); err != nil {
		return err
	}

	return nil
}

func needDoPostProvision(ctx context.Context, cli client.Client,
	cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent) (bool, error) {
	// if the component does not have a custom postProvision, skip it
	if !checkPostProvisionAction(synthesizeComp) {
		return false, nil
	}

	// TODO(xingran): The PostProvision handling for the ComponentReady & ClusterReady condition has been implemented. The PostProvision for other conditions is currently pending implementation.
	actionPreCondition := synthesizeComp.LifecycleActions.PostProvision.CustomHandler.PreCondition
	if actionPreCondition != nil {
		switch *actionPreCondition {
		case appsv1alpha1.ComponentReadyPreConditionType:
			if comp.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
				return false, nil
			}
		case appsv1alpha1.ClusterReadyPreConditionType:
			if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
				return false, nil
			}
		default:
			return false, nil
		}
	} else if comp.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
		// if the PreCondition is not set, the default preCondition is ComponentReady
		return false, nil
	}

	if comp.Annotations == nil {
		return true, nil
	}

	// determine whether the component has undergone postProvision by examining the annotation
	jobExist := checkPostProvisionJobExist(ctx, cli, cluster, genPostProvisionJobName(cluster.Name, synthesizeComp.Name))
	finishAnnotationExist := checkPostProvisionDoneAnnotationExist(*cluster, *comp, *synthesizeComp)
	if finishAnnotationExist && !jobExist {
		// if the annotation has been set and the job does not exist, it means that the postProvision has finished, so skip it
		return false, nil
	}
	return true, nil
}

// createPostProvisionJobIfNotExist creates a job to execute component-level postProvision command, each component only has a corresponding job.
func createPostProvisionJobIfNotExist(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *SynthesizedComponent) (*batchv1.Job, error) {
	if !checkPostProvisionAction(synthesizeComp) {
		return nil, nil
	}

	postProvisionJob, err := renderPostProvisionCmdJob(ctx, cli, cluster, synthesizeComp)
	if err != nil {
		return nil, err
	}

	key := types.NamespacedName{Namespace: cluster.Namespace, Name: postProvisionJob.Name}
	existJob := &batchv1.Job{}
	exist, _ := intctrlutil.CheckResourceExists(ctx, cli, key, existJob)
	if exist {
		return existJob, nil
	}

	// set the controller reference
	if err := intctrlutil.SetControllerReference(comp, postProvisionJob); err != nil {
		return postProvisionJob, err
	}

	// create the postProvisionJob if not exist
	if err := cli.Create(ctx, postProvisionJob); err != nil {
		return postProvisionJob, err
	}
	return postProvisionJob, nil
}

func checkPostProvisionJobExist(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, jobName string) bool {
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: jobName}
	existJob := &batchv1.Job{}
	exist, _ := intctrlutil.CheckResourceExists(ctx, cli, key, existJob)
	return exist
}

// renderPostProvisionCmdJob renders and creates the postProvision command job.
func renderPostProvisionCmdJob(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent) (*batchv1.Job, error) {
	if !checkPostProvisionAction(synthesizeComp) {
		return nil, errors.New("postProvision CustomHandler spec not found")
	}

	if synthesizeComp.LifecycleActions.PostProvision.CustomHandler.Exec == nil {
		return nil, errors.New("postProvision customHandler only support exec command by now, please check your customHandler spec.")
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

	renderJob := func(postProvisionSpec *appsv1alpha1.LifecycleActionHandler, postProvisionEnvs []corev1.EnvVar) (*batchv1.Job, error) {
		var (
			postProvisionCustomHandler = postProvisionSpec.CustomHandler
		)
		volumes, volumeMounts := renderJobPodVolumes()
		jobName := genPostProvisionJobName(cluster.Name, synthesizeComp.Name)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      jobName,
				Labels:    getPostProvisionCmdJobLabel(cluster.Name, synthesizeComp.Name),
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
								Name:            kbPostProvisionJobContainerName,
								Image:           postProvisionCustomHandler.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         postProvisionCustomHandler.Exec.Command,
								Args:            postProvisionCustomHandler.Exec.Args,
								Env:             postProvisionEnvs,
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
		return job, nil
	}

	postProvisionEnvs, err := buildPostProvisionEnvs(cluster, synthesizeComp, pods, &tplPod)
	if err != nil {
		return nil, err
	}

	job, err := renderJob(synthesizeComp.LifecycleActions.PostProvision, postProvisionEnvs)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// buildPostProvisionEnvs builds the postProvision command job envs.
func buildPostProvisionEnvs(cluster *appsv1alpha1.Cluster,
	synthesizeComp *SynthesizedComponent,
	pods []corev1.Pod,
	tplPod *corev1.Pod) ([]corev1.EnvVar, error) {
	var workloadEnvs []corev1.EnvVar

	if synthesizeComp != nil && synthesizeComp.LifecycleActions != nil &&
		synthesizeComp.LifecycleActions.PostProvision != nil && synthesizeComp.LifecycleActions.PostProvision.CustomHandler != nil {
		workloadEnvs = append(workloadEnvs, synthesizeComp.LifecycleActions.PostProvision.CustomHandler.Env...)
	}

	if tplPod != nil && len(tplPod.Spec.Containers) > 0 {
		// add tht first container's environment variables of the template pod
		workloadEnvs = append(workloadEnvs, tplPod.Spec.Containers[0].Env...)
	}

	compEnvs := genClusterComponentEnv(cluster, pods)
	if len(compEnvs) > 0 {
		workloadEnvs = append(workloadEnvs, compEnvs...)
	}

	return workloadEnvs, nil
}

// genClusterComponentEnv generates the cluster component relative envs.
func genClusterComponentEnv(cluster *appsv1alpha1.Cluster, pods []corev1.Pod) []corev1.EnvVar {
	if cluster == nil || cluster.Spec.ComponentSpecs == nil {
		return nil
	}
	compEnvs := make([]corev1.EnvVar, 0)
	compList := make([]string, 0, len(cluster.Spec.ComponentSpecs))
	compPodNameList := make([]string, 0, len(pods))
	compPodIPList := make([]string, 0, len(pods))
	compHostNameList := make([]string, 0, len(pods))
	compHostIPList := make([]string, 0, len(pods))

	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compList = append(compList, compSpec.Name)
	}
	compEnvs = append(compEnvs, corev1.EnvVar{
		Name:  kbPostProvisionClusterCompList,
		Value: strings.Join(compList, ","),
	})

	for _, pod := range pods {
		compPodNameList = append(compPodNameList, pod.Name)
		compPodIPList = append(compPodIPList, pod.Status.PodIP)
		compHostNameList = append(compHostNameList, pod.Spec.NodeName)
		compHostIPList = append(compHostIPList, pod.Status.HostIP)
	}
	compEnvs = append(compEnvs, []corev1.EnvVar{
		{
			Name:  kbPostProvisionClusterCompPodNameList,
			Value: strings.Join(compPodNameList, ","),
		},
		{
			Name:  kbPostProvisionClusterCompPodIPList,
			Value: strings.Join(compPodIPList, ","),
		},
		{
			Name:  kbPostProvisionClusterCompPodHostNameList,
			Value: strings.Join(compHostNameList, ","),
		},
		{
			Name:  kbPostProvisionClusterCompPodHostIPList,
			Value: strings.Join(compHostIPList, ","),
		}}...)

	return compEnvs
}

// genPostProvisionJobName generates the postProvision job name.
func genPostProvisionJobName(clusterName, componentName string) string {
	return fmt.Sprintf("%s-%s-%s", kbPostProvisionJobNamePrefix, clusterName, componentName)
}

// getPostProvisionCmdJobLabel gets the labels for job that execute the postProvision commands.
func getPostProvisionCmdJobLabel(clusterName, componentName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
		constant.AppManagedByLabelKey:   constant.AppName,
		kbPostProvisionJobLabelKey:      kbPostProvisionJobLabelValue,
	}
}

// setPostProvisionDoneAnnotation sets the postProvision done annotation to the component object.
func setPostProvisionDoneAnnotation(cli client.Client,
	comp *appsv1alpha1.Component,
	dag *graph.DAG) error {
	graphCli := model.NewGraphClient(cli)
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	_, ok := comp.Annotations[kbCompPostProvisionDoneKey]
	if ok {
		return nil
	}
	compObj := comp.DeepCopy()
	timeStr := time.Now().Format(time.RFC3339Nano)
	comp.Annotations[kbCompPostProvisionDoneKey] = timeStr
	graphCli.Update(dag, compObj, comp, &model.ReplaceIfExistingOption{})
	return nil
}

func cleanPostProvisionJob(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp appsv1alpha1.Component,
	synthesizeComp SynthesizedComponent,
	jobName string) error {
	if cluster.Annotations == nil || comp.Annotations == nil {
		return errors.New("cluster or component annotations not found")
	}

	// check post-provision done annotation has been set
	if !checkPostProvisionDoneAnnotationExist(*cluster, comp, synthesizeComp) {
		return errors.New("cluster post-provision done annotation has not been set")
	}
	return CleanJobByName(ctx, cli, cluster, jobName)
}

func checkPostProvisionDoneAnnotationExist(cluster appsv1alpha1.Cluster,
	comp appsv1alpha1.Component,
	synthesizeComp SynthesizedComponent) bool {
	// TODO(xingran): for backward compatibility before KubeBlocks v0.8.0, check the annotation of the cluster object first, it will be deprecated in the future
	compPostStartDoneKey := fmt.Sprintf(kbCompPostStartDoneKeyPattern, fmt.Sprintf("%s-%s", cluster.Name, synthesizeComp.Name))
	_, ok := cluster.Annotations[compPostStartDoneKey]
	if ok {
		return true
	}
	_, ok = comp.Annotations[kbCompPostProvisionDoneKey]
	return ok
}

func checkPostProvisionAction(synthesizeComp *SynthesizedComponent) bool {
	if synthesizeComp == nil || synthesizeComp.LifecycleActions == nil ||
		synthesizeComp.LifecycleActions.PostProvision == nil || synthesizeComp.LifecycleActions.PostProvision.CustomHandler == nil {
		return false
	}
	return true
}
