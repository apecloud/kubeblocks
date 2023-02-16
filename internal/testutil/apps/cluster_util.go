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

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// InitClusterWithHybridComps initializes a cluster environment for testing, includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitClusterWithHybridComps(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	statelessComName,
	statefulComName,
	consensusComName string) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, *appsv1alpha1.Cluster) {
	clusterDef := CreateClusterDefWithHybridComps(testCtx, clusterDefName)
	clusterVersion := CreateClusterVersionWithHybridComps(testCtx, clusterDefName,
		clusterVersionName, []string{"docker.io/apecloud/wesql-server:latest", "busybox:latest", "docker.io/apecloud/wesql-server:latest"})
	cluster := CreateClusterWithHybridComps(testCtx, clusterDefName,
		clusterVersionName, clusterName, statelessComName, consensusComName, statefulComName)
	return clusterDef, clusterVersion, cluster
}

func CreateK8sResource(testCtx testutil.TestContext, obj client.Object) client.Object {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, obj)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(&testCtx, client.ObjectKeyFromObject(obj),
		obj, true)).Should(gomega.Succeed())
	return obj
}

// CreateClusterWithHybridComps creates a cluster with hybrid components for testing.
func CreateClusterWithHybridComps(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	statelessComName,
	consensusComName,
	statefulComName string) *appsv1alpha1.Cluster {
	return CreateCustomizedObj(&testCtx, "hybrid/hybrid_cluster.yaml",
		&appsv1alpha1.Cluster{}, CustomizeObjYAML(clusterVersionName, clusterDefName, clusterName, clusterVersionName,
			clusterDefName, statelessComName, consensusComName, statefulComName))
}

// CreateClusterDefWithHybridComps creates a clusterDefinition with hybrid components for testing.
func CreateClusterDefWithHybridComps(testCtx testutil.TestContext, clusterDefName string) *appsv1alpha1.ClusterDefinition {
	return CreateCustomizedObj(&testCtx, "hybrid/hybrid_cd.yaml",
		&appsv1alpha1.ClusterDefinition{}, CustomizeObjYAML(clusterDefName))
}

// CreateClusterVersionWithHybridComps creates a clusterVersion with hybrid components for testing.
func CreateClusterVersionWithHybridComps(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string,
	images []string) *appsv1alpha1.ClusterVersion {
	return CreateCustomizedObj(&testCtx, "hybrid/hybrid_cv.yaml",
		&appsv1alpha1.ClusterVersion{}, CustomizeObjYAML(clusterVersionName, clusterDefName, images[0], images[1], images[2]))
}

// CreateHybridCompsClusterVersionForUpgrade creates a clusterVersion with hybrid components for upgrading test.
func CreateHybridCompsClusterVersionForUpgrade(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string) *appsv1alpha1.ClusterVersion {
	return CreateClusterVersionWithHybridComps(testCtx, clusterDefName, clusterVersionName,
		[]string{"docker.io/apecloud/wesql-server:8.0.30", "busybox:1.30.0", "docker.io/apecloud/wesql-server:8.0.30"})
}

// GetClusterComponentPhase gets the component phase of testing cluster for verification.
func GetClusterComponentPhase(testCtx testutil.TestContext, clusterName, componentName string) func(g gomega.Gomega) appsv1alpha1.Phase {
	return func(g gomega.Gomega) appsv1alpha1.Phase {
		tmpCluster := &appsv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterName,
			Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(gomega.Succeed())
		return tmpCluster.Status.Components[componentName].Phase
	}
}

// GetClusterPhase gets the testing cluster's phase in status for verification.
func GetClusterPhase(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) appsv1alpha1.Phase {
	return func(g gomega.Gomega) appsv1alpha1.Phase {
		cluster := &appsv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.Phase
	}
}

// GetClusterObservedGeneration gets the testing cluster's ObservedGeneration in status for verification.
func GetClusterObservedGeneration(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) int64 {
	return func(g gomega.Gomega) int64 {
		cluster := &appsv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.ObservedGeneration
	}
}
