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
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

var (
	ReplicationComponentName = "redis-rsts"
)

func InitReplicationRedis(testCtx testutil.TestContext,
	clusterDefName,
	clusterVersionName,
	clusterName string) (*dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, *dbaasv1alpha1.Cluster) {
	clusterDef := CreateReplicationRedisClusterDef(testCtx, clusterDefName)
	clusterVersion := CreateReplicationRedisClusterVersion(testCtx, clusterDefName, clusterVersionName)
	cluster := CreateReplicationCluster(testCtx, clusterDefName, clusterVersionName, clusterName)
	return clusterDef, clusterVersion, cluster
}

func CreateReplicationCluster(testCtx testutil.TestContext, clusterDefName, clusterVersionName, clusterName string) *dbaasv1alpha1.Cluster {
	clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  clusterVersionRef: %s
  terminationPolicy: WipeOut
  components:
  - name: replication
    type: replication
    monitor: false
    primaryIndex: 0
    replicas: 2
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
`, clusterName, clusterDefName, clusterVersionName)
	cluster := &dbaasv1alpha1.Cluster{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), cluster)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, &dbaasv1alpha1.Cluster{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return cluster
}

func CreateReplicationRedisClusterDef(testCtx testutil.TestContext, clusterDefName string) *dbaasv1alpha1.ClusterDefinition {
	clusterDefYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: %s
spec:
  type: state.redis-7
  components:
    - typeName: replication
      defaultReplicas: 2
      minReplicas: 1
      maxReplicas: 16
      componentType: Replication
`, clusterDefName)
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), clusterDef)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, &dbaasv1alpha1.ClusterDefinition{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return clusterDef
}

func CreateReplicationRedisClusterVersion(testCtx testutil.TestContext, clusterDefName, clusterVersionName string) *dbaasv1alpha1.ClusterVersion {
	clusterVerYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:     %s
spec:
  clusterDefinitionRef: %s
  components:
  - type: replication
    podSpec:
      containers:
      - name: redis
        image: registry.hub.docker.com/library/redis:7.0.5
`, clusterVersionName, clusterDefName)
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	gomega.Expect(yaml.Unmarshal([]byte(clusterVerYAML), clusterVersion)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: clusterVersionName, Namespace: testCtx.DefaultNamespace}, clusterVersion)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return clusterVersion
}

// MockReplicationComponentStatefulSet mock the component statefulSet, just using in envTest
func MockReplicationComponentStatefulSet(testCtx testutil.TestContext, clusterName string) *appsv1.StatefulSet {
	stsName := clusterName + "-" + ConsensusComponentName
	statefulSetYaml := fmt.Sprintf(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: state.redis-7-apecloud-redis
    app.kubernetes.io/managed-by: kubeblocks
    kubeblocks.io/role: primary
  name: %s
  namespace: default
spec:
  podManagementPolicy: Parallel
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component-name: %s
      app.kubernetes.io/instance: %s
      app.kubernetes.io/name: state.redis-7-apecloud-redis
      kubeblocks.io/test: test
      app.kubernetes.io/managed-by: kubeblocks
  template:
    metadata:
      labels:
        app.kubernetes.io/component-name: %s
        app.kubernetes.io/instance: %s
        app.kubernetes.io/name: state.redis-7-apecloud-redis
        kubeblocks.io/test: test
        app.kubernetes.io/managed-by: kubeblocks
    spec:
      containers:
      - args:
        - /etc/conf/redis.conf
        command:
        - /bin/bash
        - -c
        env:
        - name: KB_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: KB_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: KB_SA_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.serviceAccountName
        - name: KB_NODENAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: KB_HOSTIP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.hostIP
        - name: KB_PODIP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: KB_PODIPS
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIPs
        - name: KB_CLUSTER_NAME
          value: redis
        - name: KB_COMP_NAME
          value: redis-rsts
        - name: KB_CLUSTER_COMP_NAME
          value: redis-redis-rsts
        - name: KB_PRIMARY_POD_NAME
          value: redis-redis-rsts-1-0.redis-redis-rsts-headless
        envFrom:
        - configMapRef:
            name: redis-redis-rsts-env
        image: registry.hub.docker.com/library/redis:7.0.5
        imagePullPolicy: IfNotPresent
        lifecycle:
          postStart:
            exec:
              command:
              - /bin/sh
              - -c
              - |
                set -ex
                SECONDARY_ROLE=secondary
                KB_ROLE_NAME=primary
                if [ "$KB_ROLE_NAME" = "$SECONDARY_ROLE" ]; then
                  until redis-cli -h $KB_PRIMARY_POD_NAME -p 6379 ping; do sleep 1; done
                  redis-cli -h 127.0.0.1 -p 6379 replicaof $KB_PRIMARY_POD_NAME 6379 || exit 1
                else
                  echo "primary instance skip create a replication relationship."
                  exit 0
                fi
        name: redis
        resources:
          limits:
            cpu: 280m
            memory: 380Mi
        ports:
        - containerPort: 6379
          name: redis
          protocol: TCP
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /data
          name: data
        - mountPath: /etc/conf
          name: conf
        - mountPath: /etc/conf/primary
          name: primary
        - mountPath: /etc/conf/secondary
          name: secondary
        - mountPath: /etc/conf/role
          name: pod-role
  updateStrategy:
    type: OnDelete`, ReplicationComponentName, clusterName, stsName, ReplicationComponentName, clusterName, ReplicationComponentName, clusterName)
	sts := &appsv1.StatefulSet{}
	gomega.Expect(yaml.Unmarshal([]byte(statefulSetYaml), sts)).Should(gomega.Succeed())
	gomega.Expect(testCtx.CreateObj(context.Background(), sts)).Should(gomega.Succeed())
	gomega.Eventually(func() bool {
		err := testCtx.Cli.Get(context.Background(), client.ObjectKey{Name: stsName, Namespace: testCtx.DefaultNamespace}, &appsv1.StatefulSet{})
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue())
	return sts
}
