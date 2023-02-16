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

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// InitClusterWithHybridComps initializes a cluster environment for testing, includes ClusterDefinition/ClusterVersion/Cluster resources.
func InitClusterWithHybridComps(
	testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName,
	statelessComp,
	statefulComp,
	consensusComp string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := NewClusterDefFactory(clusterDefName, MySQLType).
		AddComponent(StatelessNginxComponent, statelessComp).SetDefaultReplicas(1).
		AddComponent(ConsensusMySQLComponent, consensusComp).SetDefaultReplicas(3).
		AddComponent(StatefulMySQLComponent, statefulComp).SetDefaultReplicas(3).
		Create(&testCtx).GetObject()
	clusterVersion := NewClusterVersionFactory(clusterVersionName, clusterDefName).
		AddComponent(statelessComp).AddContainerShort(DefaultNginxContainerName, NginxImage).
		AddComponent(consensusComp).AddContainerShort(DefaultMySQLContainerName, NginxImage).
		AddComponent(statefulComp).AddContainerShort(DefaultMySQLContainerName, NginxImage).
		Create(&testCtx).GetObject()
	cluster := NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
		AddComponent(statelessComp, statelessComp).
		AddComponent(consensusComp, consensusComp).
		AddComponent(statefulComp, statefulComp).
		Create(&testCtx).GetObject()
	return clusterDef, clusterVersion, cluster
}

func CreateK8sResource(testCtx testutil.TestContext, obj client.Object) client.Object {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, obj)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(CheckObjExists(&testCtx, client.ObjectKeyFromObject(obj),
		obj, true)).Should(gomega.Succeed())
	return obj
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

// GetClusterPhase gets the testing cluster's phase in status for verification.
func GetClusterPhase(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) dbaasv1alpha1.Phase {
	return func(g gomega.Gomega) dbaasv1alpha1.Phase {
		cluster := &dbaasv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.Phase
	}
}

// GetClusterObservedGeneration gets the testing cluster's ObservedGeneration in status for verification.
func GetClusterObservedGeneration(testCtx *testutil.TestContext, clusterKey types.NamespacedName) func(gomega.Gomega) int64 {
	return func(g gomega.Gomega) int64 {
		cluster := &dbaasv1alpha1.Cluster{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, clusterKey, cluster)).Should(gomega.Succeed())
		return cluster.Status.ObservedGeneration
	}
}
