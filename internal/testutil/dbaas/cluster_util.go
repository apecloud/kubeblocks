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
	"github.com/apecloud/kubeblocks/internal/testutil"
)

func InitClusterWithHybridComps(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateClusterDefWithHybridComps(testCtx, clusterDefName)
	clusterVersion := CreateClusterVersionWithHybridComps(testCtx, clusterDefName,
		clusterVersionName, []string{"docker.io/apecloud/wesql-server:latest", "nginx:latest"})
	cluster := CreateClusterWithHybridComps(testCtx, clusterDefName, clusterVersionName, clusterName)
	return clusterDef, clusterVersion, cluster
}

// CreateClusterWithHybridComps create a mysql cluster with hybrid components
func CreateClusterWithHybridComps(testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string) *dbaasv1alpha1.Cluster {
	clusterYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  labels:
    clusterversion.kubeblocks.io/name: %s
    clusterdefinition.kubeblocks.io/name: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  clusterVersionRef: %s
  clusterDefinitionRef: %s
  components:
  - name: %s
    type: proxy
    replicas: 1
    monitor: false
  - monitor: false
    name: %s
    replicas: 3
    type: consensus
    enabledLogs: 
    - error
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 2Gi
  terminationPolicy: WipeOut
`, clusterVersionName, clusterDefName, clusterName, clusterVersionName,
		clusterDefName, StatelessComponentName, ConsensusComponentName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), cluster)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(func() error {
		return testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
	}, timeout, interval).Should(gomega.Succeed())
	return cluster
}

// CreateClusterDefWithHybridComps create a mysql clusterDefinition with hybrid components
func CreateClusterDefWithHybridComps(testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  components:
  - antiAffinity: false
    componentType: Stateless
    defaultReplicas: 1
    minReplicas: 0
    podSpec:
      containers:
      - name: nginx
    typeName: proxy
  - antiAffinity: false
    componentType: Consensus
    typeName: consensus
    logConfigs:
    - filePathPattern: /data/mysql/log/mysqld.err
      name: error
    podSpec:
      containers:
      - name: mysql
    consensusSpec:
      followers:
      - accessMode: Readonly
        name: follower
      leader:
        accessMode: ReadWrite
        name: leader
      updateStrategy: BestEffortParallel
    defaultReplicas: 3
  type: state.mysql-8`, clusterDefName)
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), clusterDef)).Should(gomega.Succeed())
	// wait until clusterDef created
	gomega.Eventually(func() error {
		return testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterDefName}, clusterDef)
	}, timeout, interval).Should(gomega.Succeed())
	return clusterDef
}

// CreateClusterVersionWithHybridComps create a mysql clusterVersion with hybrid components
func CreateClusterVersionWithHybridComps(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string,
	images []string) *dbaasv1alpha1.ClusterVersion {
	clusterVersionYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:  %s
spec:
  clusterDefinitionRef: %s
  components:
  - type: consensus
    podSpec:
      containers:
      - name: mysql
        image: %s
  - podSpec:
      containers:
      - image: %s
        imagePullPolicy: IfNotPresent
        name: nginx
    type: proxy
`, clusterVersionName, clusterDefName, images[0], images[1])
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(gomega.Succeed())
	gomega.Eventually(func() error {
		return testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterVersionName, Namespace: testCtx.DefaultNamespace}, clusterVersion)
	}, timeout, interval).Should(gomega.Succeed())
	return clusterVersion
}

func CreateHybridCompsClusterVersionForUpgrade(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	return CreateClusterVersionWithHybridComps(testCtx, clusterDefName, clusterVersionName,
		[]string{"docker.io/apecloud/wesql-server:8.0.30", "nginx:1.14.2"})
}

// ExpectClusterComponentPhase check the component phase of cluster is the expected phase.
func ExpectClusterComponentPhase(testCtx testutil.TestContext, clusterName, componentName string, expectPhase dbaasv1alpha1.Phase) bool {
	tmpCluster := &dbaasv1alpha1.Cluster{}
	err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterName,
		Namespace: testCtx.DefaultNamespace}, tmpCluster)
	if err != nil {
		return false
	}
	statusComponent := tmpCluster.Status.Components[componentName]
	return statusComponent.Phase == expectPhase
}

// ExpectClusterPhase check the cluster phase is the expected phase.
func ExpectClusterPhase(testCtx testutil.TestContext, clusterName string, expectPhase dbaasv1alpha1.Phase) bool {
	cluster := &dbaasv1alpha1.Cluster{}
	err := testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
	if err != nil {
		return false
	}
	return cluster.Status.Phase == expectPhase
}
