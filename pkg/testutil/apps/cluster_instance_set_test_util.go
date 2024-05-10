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
	"fmt"
	"strconv"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

const (
	errorLogName      = "error"
	ConsensusReplicas = 3
)

// InitConsensusMysql initializes a cluster environment which only contains a component of ConsensusSet type for testing,
// includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitConsensusMysql(testCtx *testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	consensusCompType,
	consensusCompName string) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, *appsv1alpha1.Cluster) {
	clusterDef := CreateConsensusMysqlClusterDef(testCtx, clusterDefName, consensusCompType)
	clusterVersion := CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName, consensusCompType)
	cluster := CreateConsensusMysqlCluster(testCtx, clusterDefName, clusterVersionName, clusterName, consensusCompType, consensusCompName)
	return clusterDef, clusterVersion, cluster
}

// CreateConsensusMysqlCluster creates a mysql cluster with a component of ConsensusSet type.
func CreateConsensusMysqlCluster(
	testCtx *testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	workloadType,
	consensusCompName string, pvcSize ...string) *appsv1alpha1.Cluster {
	size := "2Gi"
	if len(pvcSize) > 0 {
		size = pvcSize[0]
	}
	pvcSpec := NewPVCSpec(size)
	return NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
		AddComponent(consensusCompName, workloadType).SetReplicas(ConsensusReplicas).SetEnabledLogs(errorLogName).
		AddVolumeClaimTemplate("data", pvcSpec).Create(testCtx).GetObject()
}

// CreateConsensusMysqlClusterDef creates a mysql clusterDefinition with a component of ConsensusSet type.
func CreateConsensusMysqlClusterDef(testCtx *testutil.TestContext, clusterDefName, componentDefName string) *appsv1alpha1.ClusterDefinition {
	filePathPattern := "/data/mysql/log/mysqld.err"
	return NewClusterDefFactory(clusterDefName).AddComponentDef(ConsensusMySQLComponent, componentDefName).
		AddLogConfig(errorLogName, filePathPattern).Create(testCtx).GetObject()
}

// CreateConsensusMysqlClusterVersion creates a mysql clusterVersion with a component of ConsensusSet type.
func CreateConsensusMysqlClusterVersion(testCtx *testutil.TestContext, clusterDefName, clusterVersionName, workloadType string) *appsv1alpha1.ClusterVersion {
	return NewClusterVersionFactory(clusterVersionName, clusterDefName).AddComponentVersion(workloadType).AddContainerShort("mysql", ApeCloudMySQLImage).
		Create(testCtx).GetObject()
}

// MockInstanceSetComponent mocks the ITS component, just using in envTest
func MockInstanceSetComponent(
	testCtx *testutil.TestContext,
	clusterName,
	itsCompName string) *workloads.InstanceSet {
	itsName := clusterName + "-" + itsCompName
	return NewInstanceSetFactory(testCtx.DefaultNamespace, itsName, clusterName, itsCompName).SetReplicas(ConsensusReplicas).
		AddContainer(corev1.Container{Name: DefaultMySQLContainerName, Image: ApeCloudMySQLImage}).Create(testCtx).GetObject()
}

// MockInstanceSetPods mocks the InstanceSet pods, just using in envTest
func MockInstanceSetPods(
	testCtx *testutil.TestContext,
	its *workloads.InstanceSet,
	cluster *appsv1alpha1.Cluster,
	consensusCompName string) []*corev1.Pod {
	getReplicas := func() int {
		if its == nil || its.Spec.Replicas == nil {
			return ConsensusReplicas
		}
		return int(*its.Spec.Replicas)
	}
	leaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		for i := range its.Spec.Roles {
			if its.Spec.Roles[i].IsLeader {
				return &its.Spec.Roles[i]
			}
		}
		return nil
	}()
	noneLeaderRole := func() *workloads.ReplicaRole {
		if its == nil {
			return nil
		}
		for i := range its.Spec.Roles {
			if !its.Spec.Roles[i].IsLeader {
				return &its.Spec.Roles[i]
			}
		}
		return nil
	}()
	replicas := getReplicas()
	replicasStr := strconv.Itoa(replicas)
	podList := make([]*corev1.Pod, replicas)
	podNames := make([]string, 0)
	insTPLReplicasCnt := int32(0)
	workloadName := constant.GenerateWorkloadNamePattern(cluster.Name, consensusCompName)
	compSpec := cluster.Spec.GetComponentByName(consensusCompName)
	for _, insTpl := range compSpec.Instances {
		insReplicas := *insTpl.Replicas
		insTPLReplicasCnt += insReplicas
		podNames = append(podNames, generateInstanceNames(workloadName, insTpl.Name, insReplicas, compSpec.OfflineInstances)...)
	}
	if insTPLReplicasCnt < compSpec.Replicas {
		podNames = append(podNames, generateInstanceNames(workloadName, "",
			compSpec.Replicas-insTPLReplicasCnt, compSpec.OfflineInstances)...)
	}
	for i, pName := range podNames {
		var podRole, accessMode string
		if its != nil && len(its.Spec.Roles) > 0 {
			if i == 0 {
				podRole = leaderRole.Name
				accessMode = string(leaderRole.AccessMode)
			} else {
				podRole = noneLeaderRole.Name
				accessMode = string(noneLeaderRole.AccessMode)
			}
		}
		pod := MockInstanceSetPod(testCtx, its, cluster.Name, consensusCompName, pName, podRole, accessMode)
		annotations := pod.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[constant.ComponentReplicasAnnotationKey] = replicasStr
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
	podRole, accessMode string,
	resources ...corev1.ResourceRequirements) *corev1.Pod {
	var stsUpdateRevision string
	if its != nil {
		stsUpdateRevision = its.Status.UpdateRevision
	}
	name := ""
	if its != nil {
		name = its.Name
	}
	ml := map[string]string{
		"workloads.kubeblocks.io/managed-by": workloads.Kind,
		"workloads.kubeblocks.io/instance":   name,
	}
	podFactory := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences(workloads.GroupVersion.String(), workloads.Kind, its).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(consensusCompName).
		AddAppManagedByLabel().
		AddRoleLabel(podRole).
		AddConsensusSetAccessModeLabel(accessMode).
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
