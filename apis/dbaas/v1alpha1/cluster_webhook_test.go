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

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("cluster webhook", func() {
	var (
		clusterName                     = "cluster-webhook-mysql"
		replicationSetClusterName       = "cluster-webhook-rs"
		clusterDefinitionName           = "cluster-webhook-mysql-definition"
		secondClusterDefinition         = "cluster-webhook-mysql-definition2"
		replicationSetClusterDefinition = "cluster-webhook-rs-definition"
		appVersionName                  = "cluster-webhook-mysql-appversion"
		replicationSetAppVersionName    = "cluster-webhook-rs-appversion"
		timeout                         = time.Second * 10
		interval                        = time.Second
	)
	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})
	Context("When cluster create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing creating a new clusterDefinition when no appVersion and clusterDefinition")
			cluster, _ := createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(testCtx.CreateObj(ctx, cluster)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())

			clusterDefSecond, _ := createTestClusterDefinitionObj(secondClusterDefinition)
			Expect(testCtx.CreateObj(ctx, clusterDefSecond)).Should(Succeed())

			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By creating a new appVersion")
			appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
			Expect(testCtx.CreateObj(ctx, appVersion)).Should(Succeed())
			// wait until AppVersion created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: appVersionName}, appVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By creating a new Cluster")
			cluster, _ = createTestCluster(clusterDefinitionName, appVersionName, clusterName)
			Expect(testCtx.CreateObj(ctx, cluster)).Should(Succeed())

			By("By testing update spec.clusterDefinitionRef")
			cluster.Spec.ClusterDefRef = secondClusterDefinition
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())
			// restore
			cluster.Spec.ClusterDefRef = clusterDefinitionName

			By("By testing spec.components[?].type not found in clusterDefinitionRef")
			cluster.Spec.Components[0].Type = "replicaSet"
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())
			// restore
			cluster.Spec.Components[0].Type = "replicaSets"

			By("By testing spec.components[?].name is duplicated")
			cluster.Spec.Components[0].Name = "proxy"
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())
			// restore
			cluster.Spec.Components[0].Name = "replicaSets"

			By("By updating spec.components[?].volumeClaimTemplates storage size, expect succeed")
			cluster.Spec.Components[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())

			By("By add a component, expect succeed")
			cluster.Spec.Components = append(cluster.Spec.Components, ClusterComponent{
				Name:     "replicaSets2",
				Type:     "replicaSets",
				Replicas: 1,
				VolumeClaimTemplates: []ClusterComponentVolumeClaimTemplate{
					{
						Name: "log",
					},
				},
			})
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())

			By("By updating spec.components[?].volumeClaimTemplates[?].name, expect not succeed")
			cluster.Spec.Components[0].VolumeClaimTemplates[0].Name = "test"
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())

			// A series of tests on clusters when componentType=replication
			By("By creating a new clusterDefinition when componentType=replication")
			rsClusterDef, _ := createTestReplicationSetClusterDefinitionObj(replicationSetClusterDefinition)
			Expect(testCtx.CreateObj(ctx, rsClusterDef)).Should(Succeed())

			By("By creating a new appVersion when componentType=replication")
			rsAppVersion := createTestReplicationSetAppVersionObj(replicationSetClusterDefinition, replicationSetAppVersionName)
			Expect(testCtx.CreateObj(ctx, rsAppVersion)).Should(Succeed())

			By("By creating a new cluster when componentType=replication")
			rsCluster, _ := createTestReplicationSetCluster(replicationSetClusterDefinition, replicationSetAppVersionName, replicationSetClusterName)
			Expect(testCtx.CreateObj(ctx, rsCluster)).Should(Succeed())

			By("By updating cluster.Spec.Components[0].PrimaryStsIndex larger than cluster.Spec.Components[0].Replicas, expect not succeed")
			*rsCluster.Spec.Components[0].PrimaryStsIndex = int32(3)
			rsCluster.Spec.Components[0].Replicas = int32(3)
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())

			By("By updating cluster.Spec.Components[0].PrimaryStsIndex less than cluster.Spec.Components[0].Replicas, expect succeed")
			*rsCluster.Spec.Components[0].PrimaryStsIndex = int32(1)
			rsCluster.Spec.Components[0].Replicas = int32(2)
			Expect(k8sClient.Update(ctx, cluster)).ShouldNot(Succeed())
		})
	})
})

func createTestCluster(clusterDefinitionName, appVersionName, clusterName string) (*Cluster, error) {
	clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  appVersionRef: %s
  components:
  - name: replicaSets
    type: replicaSets
    replicas: 1
    volumeClaimTemplates: 
    - name: data
      spec:
        resources:
          requests:
            storage: 1Gi
  - name: proxy
    type: proxy
    replicas: 1
`, clusterName, clusterDefinitionName, appVersionName)
	cluster := &Cluster{}
	err := yaml.Unmarshal([]byte(clusterYaml), cluster)
	cluster.Spec.TerminationPolicy = WipeOut
	return cluster, err
}

func createTestReplicationSetCluster(clusterDefinitionName, appVersionName, clusterName string) (*Cluster, error) {
	clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  appVersionRef: %s
  components:
  - name: replication
    type: replication
    monitor: false
    primaryStsIndex: 0
    replicas: 2
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
`, clusterName, clusterDefinitionName, appVersionName)
	cluster := &Cluster{}
	err := yaml.Unmarshal([]byte(clusterYaml), cluster)
	cluster.Spec.TerminationPolicy = WipeOut
	return cluster, err
}
