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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("OpsRequest Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-ops
spec:
  type: state.mysql-8
  connectionCredential:
    password: "$(RANDOM_PASSWD)"
  components:
  - typeName: replicasets
    componentType: Consensus
    defaultReplicas: 1
    podSpec:
      containers:
      - name: mysql
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /var/lib/mysql
            name: data
          - mountPath: /var/log
            name: log
        env:
          - name: "MYSQL_ROOT_PASSWORD"
            valueFrom:
              secretKeyRef:
                name: $(CONN_CREDENTIAL_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$KB_REPLICASETS_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$KB_REPLICASETS_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$KB_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            mkdir -p /data/mysql/data;
            mkdir -p /data/mysql/log;
            chmod +777 -R /data/mysql;
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  - typeName: proxy
    componentType: Stateless
    defaultReplicas: 2
    podSpec:
      containers:
      - name: nginx
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}

		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureClusterVersionObj := func() *dbaasv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:     cluster-version-ops
spec:
  clusterDefinitionRef: cluster-definition-ops
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		clusterVersion := &dbaasv1alpha1.ClusterVersion{}
		Expect(yaml.Unmarshal([]byte(clusterVersionYAML), clusterVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
		return clusterVersion
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = assureClusterDefObj()
		}
		if clusterVersionObj == nil {
			clusterVersionObj = assureClusterVersionObj()
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}

		return &dbaasv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef:     clusterDefObj.GetName(),
				ClusterVersionRef: clusterVersionObj.GetName(),
				TerminationPolicy: dbaasv1alpha1.WipeOut,
			},
		}, clusterDefObj, clusterVersionObj, key
	}

	deleteClusterNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.Cluster{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.Cluster{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dbaasv1alpha1.Cluster{}
		}
		return client.IgnoreNotFound(err)
	}

	createOpsRequest := func(opsRequestName, clusterName string, opsType dbaasv1alpha1.OpsType) *dbaasv1alpha1.OpsRequest {
		clusterYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: %s
  namespace: default
spec:
  clusterRef: %s
  type: %s
`, opsRequestName, clusterName, opsType)
		opsRequest := &dbaasv1alpha1.OpsRequest{}
		_ = yaml.Unmarshal([]byte(clusterYaml), opsRequest)
		return opsRequest
	}

	mockSetClusterStatusPhaseToRunning := func(namespacedName types.NamespacedName) error {
		return changeStatus(namespacedName,
			func(c *dbaasv1alpha1.Cluster) {
				c.Status.Phase = dbaasv1alpha1.RunningPhase
				for componentKey, componentStatus := range c.Status.Components {
					componentStatus.Phase = dbaasv1alpha1.RunningPhase
					c.Status.Components[componentKey] = componentStatus
				}
			})
	}

	Context("with Cluster running", func() {
		It("issue an VerticalScalingOpsRequest should change Cluster's resource requirements successfully", func() {
			const compName = "wesql"

			By("create cluster")
			clusterObj, clusterDef, _, key := newClusterObj(nil, nil)
			clusterObj.Spec.Components = []dbaasv1alpha1.ClusterComponent{
				{
					Name: compName,
					Type: clusterDef.Spec.Components[0].TypeName, // "replicasets"
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":    resource.MustParse("800m"),
							"memory": resource.MustParse("512Mi"),
						},
						Requests: corev1.ResourceList{
							"cpu":    resource.MustParse("500m"),
							"memory": resource.MustParse("256Mi"),
						},
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, clusterObj)).Should(Succeed())
			Eventually(checkObj(key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration == 1).To(BeTrue())
			}), timeout, interval).Should(Succeed())

			By("check(or mock maybe) cluster status running")
			if !testCtx.UsingExistingCluster() {
				// MOCK pods are created and running, so as the cluster
				Eventually(mockSetClusterStatusPhaseToRunning(key), timeout, interval).Should(Succeed())
			}
			// TODO The following assert doesn't pass in a real K8s cluster (with UseExistingCluster set).
			// TODO After all pods(both proxy and wesql) enter `Running` state,
			// TODO Cluster.Status.Phase is still in `Creating` status.
			// TODO It seems the Cluster Reconciler doesn't be triggered to run properly,
			// TODO an additional invoke of `kubectl apply` explicitly ask it will workaround,
			// TODO I'll look into this problem later.
			Eventually(checkObj(key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
				g.Expect(cluster.Status.Phase == dbaasv1alpha1.RunningPhase).To(BeTrue())
			}), timeout, interval).Should(Succeed())

			By("send VerticalScalingOpsRequest successfully")
			verticalScalingOpsRequest := createOpsRequest("mysql-verticalscaling", clusterObj.Name, dbaasv1alpha1.VerticalScalingType)
			verticalScalingOpsRequest.Spec.TTLSecondsAfterSucceed = 1
			verticalScalingOpsRequest.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
				{
					ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: compName}, // "wesql"
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu":    resource.MustParse("400m"),
							"memory": resource.MustParse("300Mi"),
						},
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, verticalScalingOpsRequest)).Should(Succeed())

			By("check VerticalScalingOpsRequest running")
			Eventually(checkObj(intctrlutil.GetNamespacedName(verticalScalingOpsRequest),
				func(g Gomega, ops *dbaasv1alpha1.OpsRequest) {
					g.Expect(ops.Status.Phase == dbaasv1alpha1.RunningPhase).To(BeTrue())
				}), timeout, interval).Should(Succeed())

			By("check Cluster and changed component updating")
			Eventually(checkObj(key, func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
				g.Expect(cluster.Status.Phase == dbaasv1alpha1.UpdatingPhase).To(BeTrue())
				g.Expect(cluster.Status.Components[compName].Phase == dbaasv1alpha1.UpdatingPhase).To(BeTrue())
			}), timeout, interval).Should(Succeed())

			By("mock bring Cluster and changed component back to running status")
			Eventually(mockSetClusterStatusPhaseToRunning(key), timeout, interval).Should(Succeed())

			By("patch opsrequest controller to run")
			Eventually(changeSpec(intctrlutil.GetNamespacedName(verticalScalingOpsRequest),
				func(opsRequest *dbaasv1alpha1.OpsRequest) {
					opsRequest.Annotations[intctrlutil.OpsRequestReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
				}), timeout, interval).Should(Succeed())

			By("check VerticalScalingOpsRequest succeed")
			Eventually(checkObj(intctrlutil.GetNamespacedName(verticalScalingOpsRequest),
				func(g Gomega, ops *dbaasv1alpha1.OpsRequest) {
					g.Expect(ops.Status.Phase == dbaasv1alpha1.SucceedPhase).To(BeTrue())
				}), timeout, interval).Should(Succeed())

			By("check cluster resource requirements changed")
			Eventually(checkObj(key, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
				g.Expect(fetched.Spec.Components[0].Resources.Requests).To(Equal(
					verticalScalingOpsRequest.Spec.VerticalScalingList[0].Requests))
			}), timeout, interval).Should(Succeed())

			By("test deleteClusterOpsRequestAnnotation function")
			opsReconciler := OpsRequestReconciler{Client: k8sClient}
			Expect(opsReconciler.deleteClusterOpsRequestAnnotation(intctrlutil.RequestCtx{Ctx: ctx}, verticalScalingOpsRequest)).Should(Succeed())

			By("OpsRequest reclaimed after ttl")
			Eventually(func() error {
				return k8sClient.Get(ctx, intctrlutil.GetNamespacedName(verticalScalingOpsRequest), verticalScalingOpsRequest)
			}, timeout, interval).Should(Satisfy(apierrors.IsNotFound))

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})
})
