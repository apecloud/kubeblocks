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

package dbaas

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// InitConsensusMysql initializes a cluster environment which only contains a component of ConsensusSet type for testing,
// includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitConsensusMysql(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	consensusCompName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateConsensusMysqlClusterDef(testCtx, clusterDefName)
	clusterVersion := CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName)
	cluster := CreateConsensusMysqlCluster(testCtx, clusterDefName, clusterVersionName, clusterName, consensusCompName)
	return clusterDef, clusterVersion, cluster
}

// CreateConsensusMysqlCluster creates a mysql cluster with a component of ConsensusSet type.
func CreateConsensusMysqlCluster(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	consensusCompName string) *dbaasv1alpha1.Cluster {
	return CreateCustomizedObj(&testCtx, "consensusset/wesql.yaml",
		&dbaasv1alpha1.Cluster{}, CustomizeObjYAML(clusterVersionName, clusterDefName, clusterName,
			clusterVersionName, clusterDefName, consensusCompName))
}

// CreateConsensusMysqlClusterDef creates a mysql clusterDefinition with a component of ConsensusSet type.
func CreateConsensusMysqlClusterDef(testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	return CreateCustomizedObj(&testCtx, "consensusset/wesql_cd.yaml",
		&dbaasv1alpha1.ClusterDefinition{}, CustomizeObjYAML(clusterDefName))
}

// CreateConsensusMysqlClusterVersion creates a mysql clusterVersion with a component of ConsensusSet type.
func CreateConsensusMysqlClusterVersion(testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	return CreateCustomizedObj(&testCtx, "consensusset/wesql_cv.yaml",
		&dbaasv1alpha1.ClusterVersion{}, CustomizeObjYAML(clusterVersionName, clusterDefName))
}

// MockConsensusComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(
	testCtx testutil.TestContext,
	clusterName,
	consensusCompName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + consensusCompName
	return CreateCustomizedObj(&testCtx, "consensusset/stateful_set.yaml",
		&appsv1.StatefulSet{}, CustomizeObjYAML(consensusCompName, clusterName,
			stsName, consensusCompName, clusterName, consensusCompName, clusterName, "%"))
}

// MockConsensusComponentStsPod mocks to create the pod of the consensus StatefulSet, just using in envTest
func MockConsensusComponentStsPod(
	testCtx testutil.TestContext,
	sts *appsv1.StatefulSet,
	clusterName,
	consensusCompName,
	podName,
	podRole, accessMode string) *corev1.Pod {
	if sts == nil {
		sts = &appsv1.StatefulSet{}
		sts.Name = "NotFound"
		sts.UID = "7d43843d-7015-428b-a36b-972ca4b9509c"
	}
	pod := CreateCustomizedObj(&testCtx, "consensusset/stateful_set_pod.yaml",
		&corev1.Pod{}, CustomizeObjYAML(consensusCompName, clusterName,
			clusterName, consensusCompName, accessMode, podRole, podName, sts.Name, sts.UID, "%"))
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
