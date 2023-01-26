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

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/testutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

// InitClusterWithHybridComps initializes a cluster environment for testing, includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitClusterWithHybridComps(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	statelessComName,
	consensusComName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateClusterDefWithHybridComps(ctx, testCtx, clusterDefName)
	clusterVersion := CreateClusterVersionWithHybridComps(ctx, testCtx, clusterDefName,
		clusterVersionName, []string{"docker.io/apecloud/wesql-server:latest", "busybox:latest"})
	cluster := CreateClusterWithHybridComps(ctx, testCtx, clusterDefName,
		clusterVersionName, clusterName, statelessComName, consensusComName)
	return clusterDef, clusterVersion, cluster
}

func CreateK8sResource(ctx context.Context, testCtx testutil.TestContext, obj client.Object) client.Object {
	gomega.Expect(testCtx.CreateObj(context.Background(), obj)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(&testCtx, intctrlutil.GetNamespacedName(obj),
		obj, true)).Should(gomega.Succeed())
	return obj
}

// CreateClusterWithHybridComps creates a cluster with hybrid components for testing.
func CreateClusterWithHybridComps(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	statelessComName,
	consensusComName string) *dbaasv1alpha1.Cluster {
	clusterBytes, err := testdata.GetTestDataFileContent("hybrid/hybrid_cluster.yaml")
	if err != nil {
		return nil
	}
	clusterYaml := fmt.Sprintf(string(clusterBytes), clusterVersionName, clusterDefName, clusterName, clusterVersionName,
		clusterDefName, statelessComName, consensusComName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, cluster).(*dbaasv1alpha1.Cluster)
}

// CreateClusterDefWithHybridComps creates a clusterDefinition with hybrid components for testing.
func CreateClusterDefWithHybridComps(ctx context.Context, testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefBytes, err := testdata.GetTestDataFileContent("hybrid/hybrid_cd.yaml")
	if err != nil {
		return nil
	}
	clusterDefYaml := fmt.Sprintf(string(clusterDefBytes), clusterDefName)
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterDef).(*dbaasv1alpha1.ClusterDefinition)
}

// CreateClusterVersionWithHybridComps creates a clusterVersion with hybrid components for testing.
func CreateClusterVersionWithHybridComps(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string,
	images []string) *dbaasv1alpha1.ClusterVersion {
	clusterVersionBytes, err := testdata.GetTestDataFileContent("hybrid/hybrid_cv.yaml")
	if err != nil {
		return nil
	}
	clusterVersionYAML := fmt.Sprintf(string(clusterVersionBytes), clusterVersionName, clusterDefName, images[0], images[1])
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterVersion).(*dbaasv1alpha1.ClusterVersion)
}

// CreateHybridCompsClusterVersionForUpgrade creates a clusterVersion with hybrid components for upgrading test.
func CreateHybridCompsClusterVersionForUpgrade(ctx context.Context,
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	return CreateClusterVersionWithHybridComps(ctx, testCtx, clusterDefName, clusterVersionName,
		[]string{"docker.io/apecloud/wesql-server:8.0.30", "busybox:1.30.0"})
}

// GetClusterComponentPhase gets the component phase of testing cluster for verification.
func GetClusterComponentPhase(testCtx testutil.TestContext, clusterName, componentName string) func(g gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		tmpCluster := &dbaasv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterName,
			Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(gomega.Succeed())
		return tmpCluster.Status.Components[componentName].Phase
	}
}

// GetClusterPhase gets the testing cluster phase for verification.
func GetClusterPhase(ctx context.Context, testCtx testutil.TestContext, clusterName string) func(g gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		cluster := &dbaasv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterName,
			Namespace: testCtx.DefaultNamespace}, cluster)).Should(gomega.Succeed())
		return cluster.Status.Phase
	}
}

// MockClusterDefinition creates a clusterDefinition from file.
func MockClusterDefinition(ctx context.Context, testCtx testutil.TestContext, clusterDefName string, filePath string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefBytes, err := testdata.GetTestDataFileContent(filePath)
	if err != nil {
		return nil
	}
	clusterDefYaml := fmt.Sprintf(string(clusterDefBytes), clusterDefName)
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(gomega.Succeed())
	return CreateK8sResource(ctx, testCtx, clusterDef).(*dbaasv1alpha1.ClusterDefinition)
}
