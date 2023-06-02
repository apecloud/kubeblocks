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

package apps

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

const (
	errorLogName      = "error"
	leader            = "leader"
	follower          = "follower"
	learner           = "learner"
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

// MockConsensusComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(
	testCtx *testutil.TestContext,
	clusterName,
	consensusCompName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + consensusCompName
	return NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, consensusCompName).SetReplicas(ConsensusReplicas).
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
	pod := NewPodFactory(testCtx.DefaultNamespace, podName).
		SetOwnerReferences("apps/v1", constant.StatefulSetKind, sts).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(consensusCompName).
		AddAppManangedByLabel().
		AddRoleLabel(podRole).
		AddConsensusSetAccessModeLabel(accessMode).
		AddControllerRevisionHashLabel(stsUpdateRevision).
		AddContainer(corev1.Container{Name: DefaultMySQLContainerName, Image: ApeCloudMySQLImage}).
		CheckedCreate(testCtx).GetObject()
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
	podList := make([]*corev1.Pod, ConsensusReplicas)
	for i := 0; i < ConsensusReplicas; i++ {
		podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusCompName, i)
		podRole := "follower"
		accessMode := "Readonly"
		if i == 0 {
			podRole = "leader"
			accessMode = "ReadWrite"
		}
		// mock StatefulSet to create all pods
		pod := MockConsensusComponentStsPod(testCtx, sts, clusterName, consensusCompName, podName, podRole, accessMode)
		podList[i] = pod
	}
	return podList
}
