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

package component

import (
	"context"
	"fmt"
	"time"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("ReplicationSet Controller", func() {

	var (
		randomStr      = testCtx.GetRandomStr()
		timeout        = time.Second * 20
		interval       = time.Second
		clusterName    = "rs-cluster-" + randomStr
		clusterDefName = "cluster-def-replication-" + randomStr
		appVersionName = "app-version-replication-" + randomStr
		namespace      = "default"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	createCluster := func() *dbaasv1alpha1.Cluster {
		clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  appVersionRef: %s
  terminationPolicy: WipeOut
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
`, clusterName, clusterDefName, appVersionName)
		cluster := &dbaasv1alpha1.Cluster{}
		Expect(yaml.Unmarshal([]byte(clusterYaml), cluster)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), cluster)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, &dbaasv1alpha1.Cluster{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return cluster
	}

	createClusterDef := func() {
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
		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).Should(Succeed())
		Expect(testCtx.CreateObj(context.Background(), clusterDef)).Should(Succeed())
		// wait until clusterDef created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, &dbaasv1alpha1.ClusterDefinition{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
	}

	createAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
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
`, appVersionName, clusterDefName)
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, appVersion)).Should(Succeed())
		return appVersion
	}

	Context("test replicationSet controller", func() {
		It("", func() {
			createClusterDef()
			createAppVersionObj()
			cluster := createCluster()

			By("init cluster status")
			componentName := "rsts-comp"
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.Components = map[string]*dbaasv1alpha1.ClusterStatusComponent{
				componentName: {
					Phase: dbaasv1alpha1.RunningPhase,
					ReplicationSetStatus: &dbaasv1alpha1.ReplicationSetStatus{
						Primary: dbaasv1alpha1.ReplicationMemberStatus{
							Pod:  clusterName + componentName + "-0-0",
							Role: "primary",
						},
						Secondaries: []dbaasv1alpha1.ReplicationMemberStatus{
							{
								Pod:  clusterName + componentName + "-1-0",
								Role: "secondary",
							},
							{
								Pod:  clusterName + componentName + "-2-0",
								Role: "secondary",
							},
						},
					},
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())

			By("testing sync cluster status with adding pod")
			var podList []*corev1.Pod
			sts := newStatefulSet(clusterName+componentName+"-3", 1)
			pod := newStatefulSetPod(sts, 0)
			pod.Labels = make(map[string]string, 0)
			pod.Labels[intctrlutil.ReplicationSetRoleLabelKey] = "secondary"
			podList = append(podList, pod)
			Expect(needUpdateReplicationSetStatus(cluster.Status.Components[componentName].ReplicationSetStatus, podList)).Should(BeTrue())

			By("testing sync cluster status with remove pod")
			var podRemoveList []corev1.Pod
			sts = newStatefulSet(clusterName+componentName+"-2", 1)
			pod = newStatefulSetPod(sts, 0)
			pod.Labels = make(map[string]string, 0)
			pod.Labels[intctrlutil.ReplicationSetRoleLabelKey] = "secondary"
			podRemoveList = append(podRemoveList, *pod)
			Expect(needRemoveReplicationSetStatus(cluster.Status.Components[componentName].ReplicationSetStatus, podRemoveList)).Should(BeTrue())
		})
	})
})
