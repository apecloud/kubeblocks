/*
Copyright ApeCloud, Inc.

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

package apps

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	constant "github.com/apecloud/kubeblocks/internal/constant"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

const (
	errorLogName = "error"
	leader       = "leader"
	follower     = "follower"
	learner      = "learner"
)

// InitConsensusMysql initializes a cluster environment which only contains a component of ConsensusSet type for testing,
// includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitConsensusMysql(testCtx testutil.TestContext,
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
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	workloadType,
	consensusCompName string) *appsv1alpha1.Cluster {
	pvcSpec := NewPVC("2Gi")
	return NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
		AddComponent(consensusCompName, workloadType).SetReplicas(3).SetEnabledLogs(errorLogName).
		AddVolumeClaimTemplate("data", &pvcSpec).Create(&testCtx).GetObject()
}

// CreateConsensusMysqlClusterDef creates a mysql clusterDefinition with a component of ConsensusSet type.
func CreateConsensusMysqlClusterDef(testCtx testutil.TestContext, clusterDefName, workloadType string) *appsv1alpha1.ClusterDefinition {
	filePathPattern := "/data/mysql/log/mysqld.err"
	return NewClusterDefFactory(clusterDefName).AddComponent(ConsensusMySQLComponent, workloadType).
		AddLogConfig(errorLogName, filePathPattern).Create(&testCtx).GetObject()
}

// CreateConsensusMysqlClusterVersion creates a mysql clusterVersion with a component of ConsensusSet type.
func CreateConsensusMysqlClusterVersion(testCtx testutil.TestContext, clusterDefName, clusterVersionName, workloadType string) *appsv1alpha1.ClusterVersion {
	return NewClusterVersionFactory(clusterVersionName, clusterDefName).AddComponent(workloadType).AddContainerShort("mysql", ApeCloudMySQLImage).
		Create(&testCtx).GetObject()
}

// MockConsensusComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(
	testCtx testutil.TestContext,
	clusterName,
	consensusCompName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + consensusCompName
	return NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, consensusCompName).SetReplicas(int32(3)).
		AddContainer(corev1.Container{Name: DefaultMySQLContainerName, Image: ApeCloudMySQLImage}).Create(&testCtx).GetObject()
}

// MockConsensusComponentStsPod mocks to create the pod of the consensus StatefulSet, just using in envTest
func MockConsensusComponentStsPod(
	testCtx testutil.TestContext,
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
		Create(&testCtx).GetObject()
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
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	consensusCompName string) []*corev1.Pod {
	podList := make([]*corev1.Pod, 3)
	for i := 0; i < 3; i++ {
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
