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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
// includes ClusterDefinition/Cluster resources.
func InitConsensusMysql(testCtx *testutil.TestContext,
	clusterDefName,
	clusterName,
	consensusCompType,
	consensusCompName string) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.Cluster) {
	clusterDef := CreateConsensusMysqlClusterDef(testCtx, clusterDefName, consensusCompType)
	cluster := CreateConsensusMysqlCluster(testCtx, clusterDefName, clusterName, consensusCompType, consensusCompName)
	return clusterDef, cluster
}

// CreateConsensusMysqlCluster creates a mysql cluster with a component of ConsensusSet type.
func CreateConsensusMysqlCluster(
	testCtx *testutil.TestContext,
	clusterDefName,
	clusterName,
	workloadType,
	consensusCompName string, pvcSize ...string) *appsv1alpha1.Cluster {
	size := "2Gi"
	if len(pvcSize) > 0 {
		size = pvcSize[0]
	}
	pvcSpec := NewPVCSpec(size)
	return NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
		AddComponent(consensusCompName, workloadType).SetReplicas(ConsensusReplicas).SetEnabledLogs(errorLogName).
		AddVolumeClaimTemplate("data", pvcSpec).Create(testCtx).GetObject()
}

// CreateConsensusMysqlClusterDef creates a mysql clusterDefinition with a component of ConsensusSet type.
func CreateConsensusMysqlClusterDef(testCtx *testutil.TestContext, clusterDefName, componentDefName string) *appsv1alpha1.ClusterDefinition {
	filePathPattern := "/data/mysql/log/mysqld.err"
	return NewClusterDefFactory(clusterDefName).AddComponentDef(ConsensusMySQLComponent, componentDefName).
		AddLogConfig(errorLogName, filePathPattern).Create(testCtx).GetObject()
}

// MockConsensusComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(
	testCtx *testutil.TestContext,
	clusterName,
	consensusCompName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + consensusCompName
	return NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, consensusCompName).SetReplicas(ConsensusReplicas).
		AddContainer(corev1.Container{Name: DefaultMySQLContainerName, Image: ApeCloudMySQLImage}).Create(testCtx).GetObject()
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

// MockConsensusComponentStsPod mocks to create the pod of the consensus StatefulSet, just using in envTest
func MockConsensusComponentStsPod(
	testCtx *testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	consensusCompName,
	podName,
	podRole, accessMode string) *corev1.Pod {
	var stsUpdateRevision string
	if sts != nil {
		stsUpdateRevision = sts.Status.UpdateRevision
	}
	podFactory := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences("apps/v1", constant.StatefulSetKind, sts).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(consensusCompName).
		AddAppManagedByLabel().
		AddRoleLabel(podRole).
		AddConsensusSetAccessModeLabel(accessMode).
		AddControllerRevisionHashLabel(stsUpdateRevision).
		AddVolume(corev1.Volume{
			Name: DataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("%s-%s", DataVolumeName, podName),
				},
			},
		}).
		AddContainer(corev1.Container{
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
		})
	if sts != nil && sts.Labels[constant.AppNameLabelKey] != "" {
		podFactory.AddAppNameLabel(sts.Labels[constant.AppNameLabelKey])
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

// MockConsensusComponentPods mocks the component pods, just using in envTest
func MockConsensusComponentPods(
	testCtx *testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	consensusCompName string) []*corev1.Pod {
	getReplicas := func() int {
		if sts == nil || sts.Spec.Replicas == nil {
			return ConsensusReplicas
		}
		return int(*sts.Spec.Replicas)
	}
	replicas := getReplicas()
	replicasStr := strconv.Itoa(replicas)
	podList := make([]*corev1.Pod, replicas)
	for i := 0; i < replicas; i++ {
		podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusCompName, i)
		podRole := "follower"
		accessMode := "Readonly"
		if i == 0 {
			podRole = "leader"
			accessMode = "ReadWrite"
		}
		// mock StatefulSet to create all pods
		pod := MockConsensusComponentStsPod(testCtx, sts, clusterName, consensusCompName, podName, podRole, accessMode)
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
