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

package dbaas

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

const (
	timeout  = 10 * time.Second
	interval = time.Second
)

// InitConsensusMysql initializes a cluster environment which only contains a component of ConsensusSet type for testing,
// includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitConsensusMysql(ctx context.Context, testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	consensusCompName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateConsensusMysqlClusterDef(ctx, testCtx, clusterDefName)
	clusterVersion := CreateConsensusMysqlClusterVersion(ctx, testCtx, clusterDefName, clusterVersionName)
	cluster := CreateConsensusMysqlCluster(ctx, testCtx, clusterDefName, clusterVersionName, clusterName, consensusCompName)
	return clusterDef, clusterVersion, cluster
}

// CreateConsensusMysqlCluster creates a mysql cluster with a component of ConsensusSet type.
func CreateConsensusMysqlCluster(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	consensusCompName string) *dbaasv1alpha1.Cluster {
	clusterBytes, err := testdata.GetTestDataFileContent("consensusset/wesql.yaml")
	if err != nil {
		return nil
	}
	clusterYaml := fmt.Sprintf(string(clusterBytes), clusterVersionName, clusterDefName, clusterName,
		clusterVersionName, clusterDefName, consensusCompName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, cluster).(*dbaasv1alpha1.Cluster)
}

// CreateConsensusMysqlClusterDef creates a mysql clusterDefinition with a component of ConsensusSet type.
func CreateConsensusMysqlClusterDef(ctx context.Context, testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	return MockClusterDefinition(ctx, testCtx, clusterDefName, "consensusset/wesql_cd.yaml")
}

// CreateConsensusMysqlClusterVersion creates a mysql clusterVersion with a component of ConsensusSet type.
func CreateConsensusMysqlClusterVersion(ctx context.Context, testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	clusterVersionBytes, err := testdata.GetTestDataFileContent("consensusset/wesql_cv.yaml")
	if err != nil {
		return nil
	}
	clusterVersionYAML := fmt.Sprintf(string(clusterVersionBytes), clusterVersionName, clusterDefName)
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterVersion).(*dbaasv1alpha1.ClusterVersion)
}

// MockConsensusComponentStatefulSet mocks the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(ctx context.Context,
	testCtx testutil.TestContext,
	clusterName,
	consensusCompName string) *appsv1.StatefulSet {
	stsBytes, err := testdata.GetTestDataFileContent("consensusset/stateful_set.yaml")
	if err != nil {
		return nil
	}
	stsName := clusterName + "-" + consensusCompName
	statefulSetYaml := fmt.Sprintf(string(stsBytes), consensusCompName, clusterName,
		stsName, consensusCompName, clusterName, consensusCompName, clusterName, "%")
	sts := &appsv1.StatefulSet{}
	gomega.Expect(yaml.Unmarshal([]byte(statefulSetYaml), sts)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, sts).(*appsv1.StatefulSet)
}

// MockConsensusComponentStsPod mocks to create the pod of the consensus StatefulSet, just using in envTest
func MockConsensusComponentStsPod(ctx context.Context,
	testCtx testutil.TestContext,
	clusterName,
	consensusCompName,
	podName,
	podRole, accessMode string) *corev1.Pod {
	podBytes, err := testdata.GetTestDataFileContent("consensusset/stateful_set_pod.yaml")
	if err != nil {
		return nil
	}
	podYaml := fmt.Sprintf(string(podBytes), consensusCompName, clusterName, clusterName, consensusCompName, accessMode, podRole, podName, "%")
	pod := &corev1.Pod{}
	gomega.Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(gomega.Succeed())
	pod = CreateK8sResource(ctx, testCtx, pod).(*corev1.Pod)
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
func MockConsensusComponentPods(ctx context.Context,
	testCtx testutil.TestContext,
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
		pod := MockConsensusComponentStsPod(ctx, testCtx, clusterName, consensusCompName, podName, podRole, accessMode)
		podList[i] = pod
	}
	return podList
}
