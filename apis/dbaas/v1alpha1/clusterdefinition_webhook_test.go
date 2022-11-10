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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("clusterDefinition webhook", func() {
	var (
		clusterDefinitionName  = "clusterdefinition-webhook-mysql-definition"
		clusterDefinitionName2 = "clusterdefinition-webhook-mysql-definition2"
		clusterDefinitionName3 = "clusterdefinition-webhook-mysql-definition3"
		timeout                = time.Second * 10
		interval               = time.Second
	)
	Context("When clusterDefinition create and update", func() {
		It("Should webhook validate passed", func() {

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By creating a new clusterDefinition")
			clusterDef, _ = createTestClusterDefinitionObj3(clusterDefinitionName3)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())
			// wait until ClusterDefinition created
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName3}, clusterDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By creating a new clusterDefinition with componentType==Consensus but consensusSpec not present")
			clusterDef, _ = createTestClusterDefinitionObj2(clusterDefinitionName2)
			Expect(k8sClient.Create(ctx, clusterDef)).ShouldNot(Succeed())

			By("Set Leader.Replicas > 1")
			clusterDef.Spec.Components[0].ConsensusSpec = &ConsensusSetSpec{Leader: DefaultLeader}
			replicas := int32(2)
			clusterDef.Spec.Components[0].ConsensusSpec.Leader.Replicas = &replicas
			Expect(k8sClient.Create(ctx, clusterDef)).ShouldNot(Succeed())
			// restore clusterDef
			clusterDef.Spec.Components[0].ConsensusSpec.Leader.Replicas = nil

			By("Set Followers.Replicas to odd")
			followers := make([]ConsensusMember, 1)
			rel := int32(3)
			followers[0] = ConsensusMember{Name: "follower", AccessMode: "Readonly", Replicas: &rel}
			clusterDef.Spec.Components[0].ConsensusSpec.Followers = followers
			Expect(k8sClient.Create(ctx, clusterDef)).ShouldNot(Succeed())

			By("Set Followers.Replicas to 2, component.defaultReplicas to 4, " +
				"which means Leader.Replicas(1) + Followers.Replicas(2) + Learner.Replicas(0) != component.defaultReplicas")
			rel2 := int32(2)
			followers[0].Replicas = &rel2
			clusterDef.Spec.Components[0].DefaultReplicas = 4
			Expect(k8sClient.Create(ctx, clusterDef)).ShouldNot(Succeed())

			By("Set a 5 nodes cluster with 1 leader, 2 followers and 2 learners")
			clusterDef.Spec.Components[0].DefaultReplicas = 5
			clusterDef.Spec.Components[0].ConsensusSpec.Leader = ConsensusMember{Name: "leader", AccessMode: ReadWrite}
			rel3 := int32(2)
			clusterDef.Spec.Components[0].ConsensusSpec.Learner = &ConsensusMember{Name: "learner", AccessMode: None, Replicas: &rel3}
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())

		})
	})
})

// createTestClusterDefinitionObj  other webhook_test called this function, carefully for modifying the function
func createTestClusterDefinitionObj(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     %s
spec:
  type: state.mysql-8
  components:
  - typeName: replicaSets
    componentType: Stateful
  - typeName: proxy
    componentType: Stateless
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}

// createTestClusterDefinitionObj2 create an invalid obj
func createTestClusterDefinitionObj2(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     %s
spec:
  type: state.mysql-8
  components:
  - typeName: mysql-rafted
    componentType: Consensus
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}

func createTestClusterDefinitionObj3(name string) (*ClusterDefinition, error) {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     %s
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Consensus
    logsConfig:
      - name: error
        filePathPattern: /data/mysql/log/mysqld.err
      - name: slow
        filePathPattern: /data/mysql/mysqld-slow.log
    configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        volumeName: mysql-config
    componentType: Consensus
    consensusSpec:
      leader:
        name: leader
        accessMode: ReadWrite
      followers:
        - name: follower
          accessMode: Readonly
    defaultReplicas: 3
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server-8.0:0.1.2
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /data
            name: data
          - mountPath: /log
            name: log
          - mountPath: /data/config/mysql
            name: mysql-config
        env:
          - name: "MYSQL_ROOT_PASSWORD"
            valueFrom:
              secretKeyRef:
                name: $(KB_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
`, name)
	clusterDefinition := &ClusterDefinition{}
	err := yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)
	return clusterDefinition, err
}
