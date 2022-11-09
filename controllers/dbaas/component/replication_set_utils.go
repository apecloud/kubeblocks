/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package component

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func HandleReplicationSet(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	newCreateStsList []*appsv1.StatefulSet,
	existStsList []*appsv1.StatefulSet) error {

	filter := func(stsObj *appsv1.StatefulSet) (*dbaasv1alpha1.ClusterDefinitionComponent, bool, error) {
		typeName := GetComponentTypeName(*cluster, stsObj.Labels[intctrlutil.AppComponentLabelKey])
		component, err := GetComponentFromClusterDefinition(reqCtx.Ctx, cli, cluster, typeName)
		if err != nil {
			return &dbaasv1alpha1.ClusterDefinitionComponent{}, false, err
		}
		if component.ComponentType != dbaasv1alpha1.Replication {
			return component, true, nil
		}
		return component, false, nil
	}

	// handle new create StatefulSets including create a replication relationship and update Pod label, etc
	err := handleReplicationSetNewCreateSts(reqCtx, cli, cluster, newCreateStsList, filter)
	if err != nil {
		return err
	}

	// handle exist StatefulSets including delete sts when pod number larger than cluster.component[i].replicas
	// delete the StatefulSets with the largest sequence number which is not the primary role
	err = handleReplicationSetExistSts(reqCtx, cli, cluster, existStsList, filter)
	if err != nil {
		return err
	}

	return nil
}

func handleReplicationSetExistSts(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	existStsList []*appsv1.StatefulSet,
	filter func(stsObj *appsv1.StatefulSet) (*dbaasv1alpha1.ClusterDefinitionComponent, bool, error)) error {

	clusterCompReplicasMap := make(map[string]int, len(cluster.Spec.Components))
	for _, clusterComp := range cluster.Spec.Components {
		clusterCompReplicasMap[clusterComp.Name] = clusterComp.Replicas
	}

	compOwnsStsMap := make(map[string]int)
	stsToDeleteMap := make(map[string]int)
	for _, stsObj := range existStsList {
		_, skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if _, ok := compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]]; !ok {
			compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] = 0
			stsToDeleteMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] = 0
		}
		compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] += 1
		if compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] > clusterCompReplicasMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] {
			stsToDeleteMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] += 1
		}
	}

	for compKey, stsToDelNum := range stsToDeleteMap {
		if stsToDelNum == 0 {
			break
		}
		// list all statefulSets by componentKey label
		allStsList := &appsv1.StatefulSetList{}
		selector, err := labels.Parse(intctrlutil.AppComponentLabelKey + "=" + compKey)
		if err != nil {
			return err
		}
		if err := cli.List(reqCtx.Ctx, allStsList,
			&client.ListOptions{Namespace: cluster.Namespace},
			client.MatchingLabelsSelector{Selector: selector}); err != nil {
			return err
		}
		if compOwnsStsMap[compKey] != len(allStsList.Items) {
			return fmt.Errorf("statefulset total number has changed")
		}
		dos := make([]*appsv1.StatefulSet, 0)
		partition := len(allStsList.Items) - stsToDelNum
		for _, sts := range allStsList.Items {
			// if current primary statefulSet ordinal is larger than target number replica, return err
			if getOrdinalSts(&sts) > partition && checkStsIsPrimary(&sts) {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals
		sort.Sort(descendingOrdinalSts(dos))

		// delete statefulSets and svc, etc
		for i := 0; i < stsToDelNum; i++ {
			if err := cli.Delete(reqCtx.Ctx, dos[i]); err != nil {
				return err
			}
			svc := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      fmt.Sprintf("%s-%d", dos[i].Name, 0),
			}
			if err := cli.Get(reqCtx.Ctx, svcKey, svc); err != nil {
				return err
			}
			if err := cli.Delete(reqCtx.Ctx, svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func handleReplicationSetNewCreateSts(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	newCreateStsList []*appsv1.StatefulSet,
	filter func(stsObj *appsv1.StatefulSet) (*dbaasv1alpha1.ClusterDefinitionComponent, bool, error)) error {

	podRoleMap := make(map[string]string, len(newCreateStsList))
	stsRoleMap := make(map[string]string, len(newCreateStsList))
	for _, stsObj := range newCreateStsList {
		var isPrimarySts = false
		component, skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		targetPodList, err := GetPodListByStatefulSet(reqCtx.Ctx, cli, stsObj)
		if err != nil {
			return err
		}
		if len(targetPodList) != 1 {
			return fmt.Errorf("pod number in statefulset %s is not equal one", stsObj.Name)
		}
		var dbEnginePod = &targetPodList[0]
		claimPrimaryStsName := fmt.Sprintf("%s-%s-%d", cluster.Name, stsObj.Labels[intctrlutil.AppComponentLabelKey], getClaimPrimaryStsIndex(cluster, *component))
		if stsObj.Name == claimPrimaryStsName {
			isPrimarySts = true
		}
		podRoleMap[dbEnginePod.Name] = string(dbaasv1alpha1.Primary)
		stsRoleMap[stsObj.Name] = string(dbaasv1alpha1.Primary)
		if !isPrimarySts {
			podRoleMap[dbEnginePod.Name] = string(dbaasv1alpha1.Secondary)
			stsRoleMap[stsObj.Name] = string(dbaasv1alpha1.Secondary)
			// if not primary, create a replication relationship by running a Job with kube exec
			err := createReplRelationJobAndEnsure(reqCtx, cli, cluster, *component, stsObj, dbEnginePod)
			if err != nil {
				return err
			}
		}
	}
	// update replicationSet StatefulSet Label
	for k, v := range stsRoleMap {
		stsName := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      k,
		}
		err := updateReplicationSetStsRoleLabel(cli, reqCtx.Ctx, stsName, v)
		if err != nil {
			return err
		}
	}
	// update replicationSet Pod Label
	for k, v := range podRoleMap {
		podName := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      k,
		}
		err := updateReplicationSetPodRoleLabel(cli, reqCtx.Ctx, podName, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func getClaimPrimaryStsIndex(cluster *dbaasv1alpha1.Cluster, clusterDefComp dbaasv1alpha1.ClusterDefinitionComponent) int {
	claimPrimaryStsIndex := clusterDefComp.PrimaryStsIndex
	for _, clusterComp := range cluster.Spec.Components {
		if clusterComp.Type == clusterDefComp.TypeName {
			if clusterComp.PrimaryStsIndex != nil {
				claimPrimaryStsIndex = clusterComp.PrimaryStsIndex
			}
		}
	}
	return *claimPrimaryStsIndex
}

func createReplRelationJobAndEnsure(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component dbaasv1alpha1.ClusterDefinitionComponent,
	stsObj *appsv1.StatefulSet,
	enginePod *corev1.Pod) error {
	key := types.NamespacedName{Namespace: stsObj.Namespace, Name: stsObj.Name + "-repl"}
	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, cli, key, &job)
	if err != nil {
		return err
	}
	if !exists {
		// if not exist job, create a new job
		jobPodSpec, err := buildReplRelationJobPodSpec(component, stsObj, enginePod)
		if err != nil {
			return err
		}
		var ttlSecondsAfterJobFinished int32 = 30
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
				Labels:    nil,
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: key.Namespace,
						Name:      key.Name},
					Spec: jobPodSpec,
				},
				TTLSecondsAfterFinished: &ttlSecondsAfterJobFinished,
			},
		}
		scheme, _ := dbaasv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(cluster, job, scheme); err != nil {
			return err
		}
		reqCtx.Log.Info("create a built-in job from create replication relationship", "job", job)
		if err := cli.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
	}

	// ensure job finished
	jobStatusConditions := job.Status.Conditions
	if len(jobStatusConditions) > 0 {
		if jobStatusConditions[0].Type != batchv1.JobComplete {
			return fmt.Errorf("job status: %s is not Complete, please wait or check", jobStatusConditions[0].Type)
		}
	}
	return nil
}

func buildReplRelationJobPodSpec(
	component dbaasv1alpha1.ClusterDefinitionComponent,
	stsObj *appsv1.StatefulSet,
	dbEnginePod *corev1.Pod) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	container := corev1.Container{}
	container.Name = stsObj.Name

	var targetEngineContainer corev1.Container
	for _, c := range dbEnginePod.Spec.Containers {
		if c.Name == component.ReplicationSpec.CreateReplication.DbEngineContainer {
			targetEngineContainer = c
		}
	}
	container.Command = []string{"kubectl", "exec", "-i", dbEnginePod.Name, "-c", targetEngineContainer.Name, "--", "sh", "-c"}
	container.Args = component.ReplicationSpec.CreateReplication.Commands
	container.Image = component.ReplicationSpec.CreateReplication.Image
	container.VolumeMounts = targetEngineContainer.VolumeMounts
	container.Env = targetEngineContainer.Env
	podSpec.Containers = []corev1.Container{container}
	podSpec.Volumes = dbEnginePod.Spec.Volumes
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	return podSpec, nil
}

func updateReplicationSetPodRoleLabel(cli client.Client, ctx context.Context, podName types.NamespacedName, role string) error {
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}

	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[intctrlutil.ReplicationSetRoleLabelKey] = role
	err := cli.Patch(ctx, pod, patch)
	if err != nil {
		return err
	}
	return nil
}

func updateReplicationSetStsRoleLabel(cli client.Client, ctx context.Context, stsName types.NamespacedName, role string) error {
	sts := &appsv1.StatefulSet{}
	if err := cli.Get(ctx, stsName, sts); err != nil {
		return err
	}

	patch := client.MergeFrom(sts.DeepCopy())
	sts.Labels[intctrlutil.ReplicationSetRoleLabelKey] = role
	err := cli.Patch(ctx, sts, patch)
	if err != nil {
		return err
	}
	return nil
}
