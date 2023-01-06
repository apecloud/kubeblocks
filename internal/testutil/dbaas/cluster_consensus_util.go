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
)

var (
	timeout                = 10 * time.Second
	interval               = time.Second
	ctx                    = context.Background()
	ConsensusComponentName = "mysql-test"
	ConsensusComponentType = "consensus"
	RevisionID             = "6fdd48d9cd"
)

func InitConsensusMysql(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateConsensusMysqlClusterDef(testCtx, clusterDefName)
	clusterVersion := CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName)
	cluster := CreateConsensusMysqlCluster(testCtx, clusterDefName, clusterVersionName, clusterName)
	return clusterDef, clusterVersion, cluster
}

// CreateConsensusMysqlCluster create a mysql cluster with a consensus component
func CreateConsensusMysqlCluster(testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string) *dbaasv1alpha1.Cluster {
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
`, clusterVersionName, clusterDefName, clusterName, clusterVersionName, clusterDefName, ConsensusComponentName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), cluster)).Should(gomega.Succeed())
	// wait until cluster created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return cluster
}

// CreateConsensusMysqlClusterDef create a mysql clusterDefinition with a consensus component
func CreateConsensusMysqlClusterDef(testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefYaml := fmt.Sprintf(`apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  components:
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
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(ctx, client.ObjectKey{Name: clusterDefName}, clusterDef)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return clusterDef
}

// CreateConsensusMysqlClusterVersion create a mysql clusterVersion with a consensus component
func CreateConsensusMysqlClusterVersion(testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	clusterVersionYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: %s
spec:
  clusterDefinitionRef: %s
  components:
  - type: consensus
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
`, clusterVersionName, clusterDefName)
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterVersionName, Namespace: testCtx.DefaultNamespace}, clusterVersion)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return clusterVersion
}

// MockConsensusComponentStatefulSet mock the component statefulSet, just using in envTest
func MockConsensusComponentStatefulSet(testCtx testutil.TestContext, clusterName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + ConsensusComponentName
	statefulSetYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  podManagementPolicy: Parallel
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/component-name: %s
      app.kubernetes.io/instance: %s
      app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
      kubeblocks.io/test: test
      app.kubernetes.io/managed-by: kubeblocks
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: %s
        app.kubernetes.io/instance: %s
        app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
        kubeblocks.io/test: test
        app.kubernetes.io/managed-by: kubeblocks
    spec:
      containers:
      - args:
        - |
          cluster_info=""; for (( i=0; i<$KB_consensus_N; i++ )); do
            if [ $i -ne 0 ]; then
              cluster_info="$cluster_info;";
            fi;
            host=$(eval echo \$KB_consensus_"$i"_HOSTNAME)
            cluster_info="$cluster_info$host:13306";
          done; idx=0; while IFS='-' read -ra ADDR; do
            for i in "${ADDR[@]}"; do
              idx=$i;
            done;
          done <<< "$KB_POD_NAME"; echo $idx; cluster_info="$cluster_info@$(($idx+1))"; echo $cluster_info; docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
        command:
        - /bin/bash
        - -c
        env:
        - name: MYSQL_ROOT_HOST
          value: '%s'
        - name: MYSQL_ROOT_USER
          value: root
        - name: MYSQL_ROOT_PASSWORD
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "yes"
        - name: MYSQL_DATABASE
          value: mydb
        - name: MYSQL_USER
          value: u1
        - name: MYSQL_PASSWORD
          value: u1
        - name: CLUSTER_ID
          value: "1"
        - name: CLUSTER_START_INDEX
          value: "1"
        - name: REPLICATIONUSER
          value: replicator
        - name: REPLICATION_PASSWORD
        - name: MYSQL_TEMPLATE_CONFIG
        - name: MYSQL_CUSTOM_CONFIG
        - name: MYSQL_DYNAMIC_CONFIG
        - name: KB_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: KB_consensus_N
          value: "3"
        image: docker.io/apecloud/wesql-server:latest
        imagePullPolicy: IfNotPresent
        name: mysql
        resources:
          limits:
            cpu: 280m
            memory: 380Mi
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        - containerPort: 13306
          name: paxos
          protocol: TCP
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
  updateStrategy:
    type: OnDelete`, ConsensusComponentName, clusterName, stsName, ConsensusComponentName, clusterName, ConsensusComponentName, clusterName, "%")
	sts := &appsv1.StatefulSet{}
	gomega.Expect(yaml.Unmarshal([]byte(statefulSetYaml), sts)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), sts)).Should(gomega.Succeed())
	// wait until statefulset created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace}, &appsv1.StatefulSet{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return sts
}

// MockConsensusComponentStsPod mock create pod, just using in envTest
func MockConsensusComponentStsPod(testCtx testutil.TestContext, clusterName, podName, podRole, accessMode string) {
	podYaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.mysql-8-cluster-definition-consensus
    controller-revision-hash: %s-%s-6fdd48d9cd
    cs.dbaas.kubeblocks.io/access-mode: %s
    cs.dbaas.kubeblocks.io/role: %s
    app.kubernetes.io/managed-by: kubeblocks
  name: %s
  namespace: default
spec:
  containers:
  - args:
    - |
      cluster_info=""; for (( i=0; i<$KB_consensus_N; i++ )); do
        if [ $i -ne 0 ]; then
          cluster_info="$cluster_info;";
        fi;
        host=$(eval echo \$KB_consensus_"$i"_HOSTNAME)
        cluster_info="$cluster_info$host:13306";
      done; idx=0; while IFS='-' read -ra ADDR; do
        for i in "${ADDR[@]}"; do
          idx=$i;
        done;
      done <<< "$KB_POD_NAME"; echo $idx; cluster_info="$cluster_info@$(($idx+1))"; echo $cluster_info; docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
    command:
    - /bin/bash
    - -c
    env:
    - name: MYSQL_ROOT_HOST
      value: '%s'
    - name: MYSQL_ROOT_USER
      value: root
    - name: MYSQL_ROOT_PASSWORD
    - name: MYSQL_ALLOW_EMPTY_PASSWORD
      value: "yes"
    - name: MYSQL_DATABASE
      value: mydb
    - name: MYSQL_USER
      value: u1
    - name: MYSQL_PASSWORD
      value: u1
    - name: CLUSTER_ID
      value: "1"
    - name: CLUSTER_START_INDEX
      value: "1"
    - name: REPLICATIONUSER
      value: replicator
    - name: REPLICATION_PASSWORD
    - name: MYSQL_TEMPLATE_CONFIG
    - name: MYSQL_CUSTOM_CONFIG
    - name: MYSQL_DYNAMIC_CONFIG
    - name: KB_POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: KB_consensus_N
      value: "3"
    image: docker.io/apecloud/wesql-server:latest
    imagePullPolicy: IfNotPresent
    name: mysql
    ports:
    - containerPort: 3306
      name: mysql
      protocol: TCP
    - containerPort: 13306
      name: paxos
      protocol: TCP
    resources: {}
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File`, ConsensusComponentName, clusterName, clusterName, ConsensusComponentName, accessMode, podRole, podName, "%")
	pod := &corev1.Pod{}
	gomega.Expect(yaml.Unmarshal([]byte(podYaml), pod)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), pod)).Should(gomega.Succeed())
	// wait until pod created
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, &corev1.Pod{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	gomega.Expect(testCtx.Cli.Status().Patch(context.Background(), pod, patch)).Should(gomega.Succeed())
}

// MockConsensusComponentPods mock the component pods, just using in envTest
func MockConsensusComponentPods(testCtx testutil.TestContext, clusterName string) {
	for i := 0; i < 3; i++ {
		podName := fmt.Sprintf("%s-%s-%d", clusterName, ConsensusComponentName, i)
		podRole := "follower"
		accessMode := "Readonly"
		if i == 0 {
			podRole = "leader"
			accessMode = "ReadWrite"
		}
		// mock StatefulSet to create all pods
		MockConsensusComponentStsPod(testCtx, clusterName, podName, podRole, accessMode)
	}
}
