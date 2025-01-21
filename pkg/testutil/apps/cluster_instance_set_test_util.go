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
	"math"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

const (
	replicas = 3
)

func InitConsensusMysql(testCtx *testutil.TestContext, clusterName, compDefName, compName string) (*appsv1.ComponentDefinition, *appsv1.Cluster) {
	compDef := createCompDef(testCtx, compDefName)
	cluster := CreateDefaultMysqlCluster(testCtx, clusterName, compDef.GetName(), compName)
	return compDef, cluster
}

func CreateDefaultMysqlCluster(testCtx *testutil.TestContext, clusterName, compDefName, compName string, pvcSize ...string) *appsv1.Cluster {
	size := "2Gi"
	if len(pvcSize) > 0 {
		size = pvcSize[0]
	}
	pvcSpec := NewPVCSpec(size)
	return NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponent(compName, compDefName).
		SetReplicas(replicas).
		AddVolumeClaimTemplate("data", pvcSpec).
		Create(testCtx).
		GetObject()
}

func createCompDef(testCtx *testutil.TestContext, compDefName string) *appsv1.ComponentDefinition {
	return NewComponentDefinitionFactory(compDefName).SetDefaultSpec().Create(testCtx).GetObject()
}

// MockInstanceSetComponent mocks the ITS component, just using in envTest
func MockInstanceSetComponent(
	testCtx *testutil.TestContext,
	clusterName,
	itsCompName string) *workloads.InstanceSet {
	itsName := clusterName + "-" + itsCompName
	return NewInstanceSetFactory(testCtx.DefaultNamespace, itsName, clusterName, itsCompName).SetReplicas(replicas).
		AddContainer(corev1.Container{Name: DefaultMySQLContainerName, Image: ApeCloudMySQLImage}).
		SetRoles([]workloads.ReplicaRole{
			{
				Name:                 "leader",
				ParticipatesInQuorum: true,
				UpdatePriority:       5,
			},
			{
				Name:                 "follower",
				ParticipatesInQuorum: true,
				UpdatePriority:       4,
			},
		}).Create(testCtx).GetObject()
}

// MockInstanceSetPods mocks the InstanceSet pods, just using in envTest
func MockInstanceSetPods(
	testCtx *testutil.TestContext,
	its *workloads.InstanceSet,
	cluster *appsv1.Cluster,
	compName string) []*corev1.Pod {
	getReplicas := func() int {
		if its == nil || its.Spec.Replicas == nil {
			return replicas
		}
		return int(*its.Spec.Replicas)
	}
	leaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		highestPriority := 0
		var role *workloads.ReplicaRole
		for i, r := range its.Spec.Roles {
			if its.Spec.Roles[i].UpdatePriority > highestPriority {
				highestPriority = r.UpdatePriority
				role = &r
			}
		}
		return role
	}()
	noneLeaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		lowestPriority := math.MaxInt
		var role *workloads.ReplicaRole
		for i, r := range its.Spec.Roles {
			if its.Spec.Roles[i].UpdatePriority < lowestPriority {
				lowestPriority = r.UpdatePriority
				role = &r
			}
		}
		return role
	}()
	podList := make([]*corev1.Pod, getReplicas())
	podNames := generatePodNames(cluster, compName)
	for i, pName := range podNames {
		var podRole string
		if its != nil && len(its.Spec.Roles) > 0 {
			if i == 0 {
				podRole = leaderRole.Name
			} else {
				podRole = noneLeaderRole.Name
			}
		}
		pod := MockInstanceSetPod(testCtx, its, cluster.Name, compName, pName, podRole)
		annotations := pod.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		pod.Annotations = annotations
		podList[i] = pod
	}
	return podList
}

// MockInstanceSetPods2 mocks the InstanceSet pods, just using in envTest
func MockInstanceSetPods2(
	testCtx *testutil.TestContext,
	its *workloads.InstanceSet,
	clusterName, compName string, comp *appsv1.Component) []*corev1.Pod {
	getReplicas := func() int {
		if its == nil || its.Spec.Replicas == nil {
			return replicas
		}
		return int(*its.Spec.Replicas)
	}
	leaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		highestPriority := 0
		var role *workloads.ReplicaRole
		for i, r := range its.Spec.Roles {
			if its.Spec.Roles[i].UpdatePriority > highestPriority {
				highestPriority = r.UpdatePriority
				role = &r
			}
		}
		return role
	}()
	noneLeaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		lowestPriority := math.MaxInt
		var role *workloads.ReplicaRole
		for i, r := range its.Spec.Roles {
			if its.Spec.Roles[i].UpdatePriority < lowestPriority {
				lowestPriority = r.UpdatePriority
				role = &r
			}
		}
		return role
	}()
	podList := make([]*corev1.Pod, getReplicas())
	podNames := generatePodNames2(clusterName, compName, comp)
	for i, pName := range podNames {
		var podRole string
		if its != nil && len(its.Spec.Roles) > 0 {
			if i == 0 {
				podRole = leaderRole.Name
			} else {
				podRole = noneLeaderRole.Name
			}
		}
		pod := MockInstanceSetPod(testCtx, its, clusterName, compName, pName, podRole)
		annotations := pod.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		pod.Annotations = annotations
		podList[i] = pod
	}
	return podList
}

// MockInstanceSetPod mocks to create the pod of the InstanceSet, just using in envTest
func MockInstanceSetPod(
	testCtx *testutil.TestContext,
	its *workloads.InstanceSet,
	clusterName,
	consensusCompName,
	podName,
	podRole string,
	resources ...corev1.ResourceRequirements,
) *corev1.Pod {
	var stsUpdateRevision string
	if its != nil {
		stsUpdateRevision = its.Status.UpdateRevision
	}
	name := ""
	if its != nil {
		name = its.Name
	}
	ml := map[string]string{
		"workloads.kubeblocks.io/managed-by": workloads.InstanceSetKind,
		"workloads.kubeblocks.io/instance":   name,
	}
	podFactory := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences(workloads.GroupVersion.String(), workloads.InstanceSetKind, its).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(consensusCompName).
		AddAppManagedByLabel().
		AddRoleLabel(podRole).
		AddControllerRevisionHashLabel(stsUpdateRevision).
		AddLabelsInMap(ml).
		AddVolume(corev1.Volume{
			Name: DataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-%s", DataVolumeName, podName),
				},
			},
		})
	container := corev1.Container{
		Name:  DefaultMySQLContainerName,
		Image: ApeCloudMySQLImage,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/hello",
					Port: intstr.FromInt(1024),
				},
			},
			TimeoutSeconds:   1,
			PeriodSeconds:    1,
			FailureThreshold: 1,
		},
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(1024),
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: DataVolumeName, MountPath: "/test"},
		},
	}
	if len(resources) > 0 {
		container.Resources = resources[0]
	}
	podFactory.AddContainer(container)
	if its != nil && its.Labels[constant.AppNameLabelKey] != "" {
		podFactory.AddAppNameLabel(its.Labels[constant.AppNameLabelKey])
	}
	pod := podFactory.CheckedCreate(testCtx).GetObject()
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	gomega.Expect(testCtx.Cli.Status().Patch(context.Background(), pod, patch)).Should(gomega.Succeed())
	return pod
}

func generateInstanceNames(parentName, templateName string,
	replicas int32, offlineInstances []string) []string {
	usedNames := sets.New(offlineInstances...)
	var instanceNameList []string
	ordinal := 0
	for count := int32(0); count < replicas; count++ {
		var name string
		for {
			if len(templateName) == 0 {
				name = fmt.Sprintf("%s-%d", parentName, ordinal)
			} else {
				name = fmt.Sprintf("%s-%s-%d", parentName, templateName, ordinal)
			}
			ordinal++
			if !usedNames.Has(name) {
				instanceNameList = append(instanceNameList, name)
				break
			}
		}
	}
	return instanceNameList
}

func generatePodNames(cluster *appsv1.Cluster, compName string) []string {
	podNames := make([]string, 0)
	insTPLReplicasCnt := int32(0)
	workloadName := constant.GenerateWorkloadNamePattern(cluster.Name, compName)
	compSpec := cluster.Spec.GetComponentByName(compName)
	for _, insTpl := range compSpec.Instances {
		insReplicas := *insTpl.Replicas
		insTPLReplicasCnt += insReplicas
		podNames = append(podNames, generateInstanceNames(workloadName, insTpl.Name, insReplicas, compSpec.OfflineInstances)...)
	}
	if insTPLReplicasCnt < compSpec.Replicas {
		podNames = append(podNames, generateInstanceNames(workloadName, "",
			compSpec.Replicas-insTPLReplicasCnt, compSpec.OfflineInstances)...)
	}
	return podNames
}

func generatePodNames2(clusterName, compName string, comp *appsv1.Component) []string {
	podNames := make([]string, 0)
	insTPLReplicasCnt := int32(0)
	workloadName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	for _, insTpl := range comp.Spec.Instances {
		insReplicas := *insTpl.Replicas
		insTPLReplicasCnt += insReplicas
		podNames = append(podNames, generateInstanceNames(workloadName, insTpl.Name, insReplicas, comp.Spec.OfflineInstances)...)
	}
	if insTPLReplicasCnt < comp.Spec.Replicas {
		podNames = append(podNames, generateInstanceNames(workloadName, "",
			comp.Spec.Replicas-insTPLReplicasCnt, comp.Spec.OfflineInstances)...)
	}
	return podNames
}

func podIsReady(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func MockInstanceSetStatus(testCtx testutil.TestContext, cluster *appsv1.Cluster, compName string) {
	itsName := constant.GenerateClusterComponentName(cluster.Name, compName)
	its := &workloads.InstanceSet{}
	gomega.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKey{Name: itsName, Namespace: cluster.Namespace}, its)).Should(gomega.Succeed())
	currentPodNames := generatePodNames(cluster, compName)
	updateRevisions := map[string]string{}
	for _, podName := range currentPodNames {
		updateRevisions[podName] = "revision"
	}
	podList := &corev1.PodList{}
	gomega.Expect(testCtx.Cli.List(testCtx.Ctx, podList, client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compName,
	})).Should(gomega.Succeed())
	currRevisions := map[string]string{}
	newMembersStatus := make([]workloads.MemberStatus, 0)
	notReadyPodNames := make([]string, 0)
	for _, pod := range podList.Items {
		currRevisions[pod.Name] = "revision"
		if !podIsReady(&pod) {
			notReadyPodNames = append(notReadyPodNames, pod.Name)
			continue
		}
		if _, ok := pod.Labels[constant.RoleLabelKey]; !ok {
			continue
		}
		var role *workloads.ReplicaRole
		for _, r := range its.Spec.Roles {
			if r.Name == pod.Labels[constant.RoleLabelKey] {
				role = r.DeepCopy()
				break
			}
		}
		// role can be nil
		memberStatus := workloads.MemberStatus{
			PodName:     pod.Name,
			ReplicaRole: role,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}
	compSpec := cluster.Spec.GetComponentByName(compName)
	gomega.Eventually(GetAndChangeObjStatus(&testCtx, client.ObjectKey{Name: itsName, Namespace: cluster.Namespace}, func(its *workloads.InstanceSet) {
		its.Status.CurrentRevisions = currRevisions
		its.Status.UpdateRevisions = updateRevisions
		its.Status.Replicas = compSpec.Replicas
		its.Status.CurrentReplicas = int32(len(podList.Items))
		its.Status.MembersStatus = newMembersStatus
		if len(notReadyPodNames) > 0 {
			msg, _ := json.Marshal(notReadyPodNames)
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
				Type:               string(workloads.InstanceReady),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: its.Generation,
				Reason:             workloads.ReasonNotReady,
				Message:            string(msg),
			})
		} else {
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
				Type:               string(workloads.InstanceReady),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: its.Generation,
				Reason:             workloads.ReasonReady,
			})
		}
	})).Should(gomega.Succeed())
}
