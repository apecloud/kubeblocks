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
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("Cluster Controller", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3

	const leader = "leader"
	const follower = "follower"
	const volumeName = "data"

	clusterObjKey := types.NamespacedName{
		Name:      "my-cluster",
		Namespace: "default",
	}
	var deleteClusterNWait func(key types.NamespacedName) error
	var deleteClusterVersionNWait func(key types.NamespacedName) error
	var deleteClusterDefNWait func(key types.NamespacedName) error
	ctx := context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.MatchingLabels{
				intctrlutil.AppNameLabelKey: "state.mysql-8-cluster-definition",
			})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		Eventually(func() error {
			return deleteClusterNWait(clusterObjKey)
		}, timeout, interval).Should(Succeed())
	})

	assureDefaultStorageClassObj := func(sc *storagev1.StorageClass) error {
		By("Assuring an default storageClass")
		patch := client.MergeFrom(sc)
		if sc.Annotations == nil {
			sc.Annotations = map[string]string{}
		}
		sc.Annotations["storageclass.kubernetes.io/is-default-class"] = "true"
		return k8sClient.Patch(ctx, sc, patch)
	}

	assureCfgTplConfigMapObj := func() *corev1.ConfigMap {
		By("Assuring an cm obj")
		cfgCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("config/configcm.yaml",
			testdata.WithNamespace(testCtx.DefaultNamespace))
		Expect(err).Should(Succeed())
		cfgTpl, err := testdata.GetResourceFromTestData[dbaasv1alpha1.ConfigConstraint]("config/configtpl.yaml")
		Expect(err).Should(Succeed())

		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		// update phase status
		patch := client.MergeFrom(cfgTpl.DeepCopy())
		cfgTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
		Expect(k8sClient.Status().Patch(context.Background(), cfgTpl, patch)).Should(Succeed())
		return cfgCM
	}

	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("Assuring an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Stateful
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        configTplRef: mysql-tree-node-template-8.0
        configConstraintRef: mysql-tree-node-template-8.0
        namespace: default
        volumeName: mysql-config
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
          - mountPath: /data/config
            name: mysql-config
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
            for (( i=0; i<$KB_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$KB_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
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
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  - typeName: proxy
    componentType: Stateless
    defaultReplicas: 1
    podSpec:
      containers:
      - name: nginx
    service:
      ports:
      - protocol: TCP
        port: 80
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureClusterVersionObj := func() *dbaasv1alpha1.ClusterVersion {
		By("Assuring an clusterVersion obj")
		clusterVersionYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: cluster-version
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    configSpec:
      configTemplateRefs:
      - name: mysql-tree-node-template-8.0
        configTplRef: mysql-tree-node-template-8.0
        configConstraintRef: mysql-tree-node-template-8.0
        namespace: default
        volumeName: mysql-config
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
		Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
		return clusterVersion
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj()
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

	deleteClusterNWait = func(key types.NamespacedName) error {
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

	deleteClusterVersionNWait = func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.ClusterVersion{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.ClusterVersion{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dbaasv1alpha1.ClusterVersion{}
		}
		return client.IgnoreNotFound(err)
	}

	deleteClusterDefNWait = func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.ClusterDefinition{}
			if err := k8sClient.Get(ctx, key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(ctx, f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.ClusterDefinition{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(ctx, key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(ctx, key, f) {
			f = &dbaasv1alpha1.ClusterDefinition{}
		}
		return client.IgnoreNotFound(err)
	}

	// Consensus associate objs
	// ClusterDefinition with componentType = Consensus
	assureClusterDefWithConsensusObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("Assuring an clusterDefinition obj with componentType = Consensus")
		clusterDefYAML := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-consensus
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Consensus
    consensusSpec:
      leader:
        name: "leader"
        accessMode: ReadWrite
      followers:
      - name: "follower"
        accessMode: Readonly
      updateStrategy: BestEffortParallel
    service:
      ports:
      - protocol: TCP
        port: 3306
    defaultReplicas: 3
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
        env:
        - name: MYSQL_ROOT_HOST
          value: '%'
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
        command: ["/bin/bash", "-c"]
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
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  connectionCredential:
    user: root
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureClusterVersionWithConsensusObj := func() *dbaasv1alpha1.ClusterVersion {
		By("Assuring an clusterVersion obj with componentType = Consensus")
		clusterVersionYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: cluster-version-consensus
spec:
  clusterDefinitionRef: cluster-definition-consensus
  components:
  - type: replicasets
    podSpec:
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
        imagePullPolicy: IfNotPresent
`
		clusterVersion := &dbaasv1alpha1.ClusterVersion{}
		Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())
		return clusterVersion
	}

	newClusterWithConsensusObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj()
			clusterDefObj = assureClusterDefWithConsensusObj()
		}
		if clusterVersionObj == nil {
			clusterVersionObj = assureClusterVersionWithConsensusObj()
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
				Components: []dbaasv1alpha1.ClusterComponent{
					{
						Name: "wesql-test",
						Type: "replicasets",
						VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
							{
								Name: volumeName,
								Spec: &corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: resource.MustParse("1Gi"),
										},
									},
								},
							},
						},
					},
				},
			},
		}, clusterDefObj, clusterVersionObj, key
	}

	isCMAvailable := func() bool {
		csList := &corev1.ComponentStatusList{}
		_ = k8sClient.List(ctx, csList)
		isCMAvailable := false
		for _, cs := range csList.Items {
			if cs.Name != "controller-manager" {
				continue
			}
			for _, cond := range cs.Conditions {
				if cond.Type == "Healthy" && cond.Status == "True" {
					isCMAvailable = true
					break
				}
			}
		}
		return isCMAvailable
	}

	listAndCheckStatefulSet := func(key types.NamespacedName) *appsv1.StatefulSetList {
		By("Check statefulset workload has been created")
		stsList := &appsv1.StatefulSetList{}
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			return len(stsList.Items) > 0
		}, timeout, interval).Should(BeTrue())
		return stsList
	}

	createClusterNCheck := func() (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		By("Creating a cluster")
		toCreate, cd, clusterVersion, key := newClusterObj(nil, nil)
		Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

		fetchedG1 := &dbaasv1alpha1.Cluster{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, fetchedG1)).To(Succeed())
			g.Expect(fetchedG1.Status.ObservedGeneration == 1).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		return fetchedG1, cd, clusterVersion, key
	}

	Context("When creating cluster with normal", func() {
		It("Should create cluster successfully", func() {
			_, _, _, key := createClusterNCheck()

			By("Check deployment workload has been created")
			Eventually(func() bool {
				deployList := &appsv1.DeploymentList{}
				Expect(k8sClient.List(ctx, deployList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(deployList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			stsList := listAndCheckStatefulSet(key)

			By("Check statefulset pod's volumes")
			for _, sts := range stsList.Items {
				podSpec := sts.Spec.Template
				volumeNames := map[string]struct{}{}
				for _, v := range podSpec.Spec.Volumes {
					volumeNames[v.Name] = struct{}{}
				}

				for _, cc := range [][]corev1.Container{
					podSpec.Spec.Containers,
					podSpec.Spec.InitContainers,
				} {
					for _, c := range cc {
						for _, vm := range c.VolumeMounts {
							_, ok := volumeNames[vm.Name]
							Expect(ok).Should(BeTrue())
						}
					}
				}
			}

			By("Check associated PDB has been created")
			Eventually(func() bool {
				pdbList := &policyv1.PodDisruptionBudgetList{}
				Expect(k8sClient.List(ctx, pdbList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey: key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pdbList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			By("Check created sts pods template without tolerations")
			Expect(len(stsList.Items[0].Spec.Template.Spec.Tolerations) == 0).Should(BeTrue())

			By("Checking the Affinity and the TopologySpreadConstraints")
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Affinity).Should(BeNil())
			Expect(len(podSpec.TopologySpreadConstraints) == 0).Should(BeTrue())

			By("Check should create env configmap")
			cmList := &corev1.ConfigMapList{}
			Eventually(func() bool {
				Expect(k8sClient.List(ctx, cmList, client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:   key.Name,
					intctrlutil.AppConfigTypeLabelKey: "kubeblocks-env",
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(cmList.Items) == 2
			}, timeout, interval).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When deleting cluster with default termination policy", func() {
		It("Should delete cluster resources immediately", func() {
			By("Create a cluster")
			_, _, _, key := createClusterNCheck()

			By("Delete the cluster")
			Eventually(func(g Gomega) {
				g.Expect(deleteClusterNWait(key)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Wait for the cluster to terminate")
			Eventually(func(g Gomega) {
				tmp := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, tmp)).To(Satisfy(apierrors.IsNotFound))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When deleting cluster with DoNotTerminate termination policy", func() {
		It("Should not terminate immediately", func() {
			By("Create a cluster")
			_, _, _, key := createClusterNCheck()

			By("Update the cluster's termination policy to DoNotTerminate")
			Expect(changeSpec(key, func(cluster *dbaasv1alpha1.Cluster) {
				cluster.Spec.TerminationPolicy = dbaasv1alpha1.DoNotTerminate
			})).Should(Succeed())

			By("Delete the cluster")
			Eventually(func(g Gomega) {
				g.Expect(deleteClusterNWait(key)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Check the cluster do not terminate immediately")
			checkClusterDoNotTerminate := func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(strings.Contains(fetched.Status.Message,
					fmt.Sprintf("spec.terminationPolicy %s is preventing deletion.", fetched.Spec.TerminationPolicy)))
				g.Expect(len(fetched.Finalizers) > 0).To(BeTrue())
			}
			Eventually(checkClusterDoNotTerminate, timeout, interval).Should(Succeed())
			Consistently(checkClusterDoNotTerminate, waitDuration, interval).Should(Succeed())

			By("Update the cluster's termination policy to WipeOut")
			Expect(changeSpec(key, func(cluster *dbaasv1alpha1.Cluster) {
				cluster.Spec.TerminationPolicy = dbaasv1alpha1.WipeOut
			})).Should(Succeed())

			By("Wait for the cluster to terminate")
			Eventually(func(g Gomega) {
				tmp := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, tmp)).To(Satisfy(apierrors.IsNotFound))
			}, timeout, interval).Should(Succeed())
		})
	})

	changeClusterReplicas := func(clusterName types.NamespacedName, replicas int32) error {
		return changeSpec(clusterName, func(cluster *dbaasv1alpha1.Cluster) {
			if cluster.Spec.Components == nil || len(cluster.Spec.Components) == 0 {
				cluster.Spec.Components = []dbaasv1alpha1.ClusterComponent{
					{
						Name:     "replicasets",
						Type:     "replicasets",
						Replicas: &replicas,
					}}
			} else {
				*cluster.Spec.Components[0].Replicas = replicas
			}
		})
	}

	Context("When updating cluster's replica number to a valid value", func() {
		It("Should create/delete pods to match the desired replica number", func() {
			By("Creating a cluster")
			_, _, _, key := createClusterNCheck()

			replicasSeq := []int32{5, 3, 1, 0, 2, 4}
			expectedOG := int64(1)
			for _, replicas := range replicasSeq {
				By(fmt.Sprintf("Change replicas to %d", replicas))
				Expect(changeClusterReplicas(key, replicas)).Should(Succeed())
				expectedOG++

				By("Checking cluster status and the number of replicas changed")
				Eventually(func(g Gomega) {
					fetched := &dbaasv1alpha1.Cluster{}
					g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
					g.Expect(fetched.Status.ObservedGeneration == expectedOG).To(BeTrue())
				}, timeout, interval).Should(Succeed())
				stsList := listAndCheckStatefulSet(key)
				Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(replicas))
			}

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When updating cluster's replica number to an invalid value", func() {
		It("Should not success", func() {
			By("Creating a cluster")
			_, _, _, key := createClusterNCheck()

			invalidReplicas := int32(-1)
			By(fmt.Sprintf("Change replicas to %d", invalidReplicas))
			Expect(changeClusterReplicas(key, invalidReplicas)).Should(Succeed())

			By("Checking cluster status and the number of replicas unchanged")
			Consistently(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.ObservedGeneration == 1).To(BeTrue())
			}, waitDuration, interval).Should(Succeed())
			stsList := listAndCheckStatefulSet(key)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(1))

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	createCustomizedClusterNCheck := func(customizeCluster func(toCreate *dbaasv1alpha1.Cluster)) (
		*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		By("Creating a cluster")
		toCreate, cd, clusterVersion, key := newClusterObj(nil, nil)
		customizeCluster(toCreate)
		Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

		fetchedG1 := &dbaasv1alpha1.Cluster{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, fetchedG1)).To(Succeed())
			g.Expect(fetchedG1.Status.ObservedGeneration == 1).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		return fetchedG1, cd, clusterVersion, key
	}

	Context("When horizontal scaling out a cluster", func() {
		It("Should trigger a backup process(snapshot) and "+
			"create pvcs from backup for newly created replicas", func() {
			compName := "replicasets"

			By("Creating a cluster with VolumeClaimTemplate")
			var pvcSpec corev1.PersistentVolumeClaimSpec
			pvcSpec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			pvcSpec.Resources.Requests = corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			}
			initialReplicas := int32(1)
			_, clusterDef, _, key := createCustomizedClusterNCheck(func(toCreate *dbaasv1alpha1.Cluster) {
				toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{{
					Name:     compName,
					Type:     compName,
					Replicas: &initialReplicas,
					VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
						Name: volumeName,
						Spec: &pvcSpec,
					}},
				}}
			})

			By("Set HorizontalScalePolicy")
			Expect(changeSpec(intctrlutil.GetNamespacedName(clusterDef),
				func(clusterDef *dbaasv1alpha1.ClusterDefinition) {
					clusterDef.Spec.Components[0].HorizontalScalePolicy =
						&dbaasv1alpha1.HorizontalScalePolicy{Type: dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot}
				}))

			By("Creating a BackupPolicyTemplate")
			backupPolicyTplKey := types.NamespacedName{Name: "test-backup-policy-template-mysql"}
			backupPolicyTemplateYaml := fmt.Sprintf(`
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: %s
  labels:
    clusterdefinition.kubeblocks.io/name: %s
spec:
  schedule: "0 2 * * *"
  ttl: 168h0m0s
  # !!DISCUSS Number of backup retries on fail.
  onFailAttempted: 3
  hooks:
    ContainerName: mysql
    image: rancher/kubectl:v1.23.7
    preCommands:
    - touch /data/mysql/data/.restore; sync
  backupToolName: mysql-xtrabackup
`, backupPolicyTplKey.Name, clusterDef.Name)
			backupPolicyTemplate := dataprotectionv1alpha1.BackupPolicyTemplate{}
			Expect(yaml.Unmarshal([]byte(backupPolicyTemplateYaml), &backupPolicyTemplate)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, &backupPolicyTemplate)).Should(Succeed())

			By("Creating PVC for the first replica")
			for i := 0; i < int(initialReplicas); i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-%s-%s-%d
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: test-sc
  volumeMode: Filesystem
  volumeName: test-pvc
`, volumeName, key.Name, compName, i)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			}

			stsList := listAndCheckStatefulSet(key)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(initialReplicas))

			updatedReplicas := int32(3)
			By(fmt.Sprintf("Changing replicas to %d", updatedReplicas))
			Expect(changeClusterReplicas(key, updatedReplicas)).Should(Succeed())

			By("Checking BackupJob created")
			Eventually(func() bool {
				backupJobList := dataprotectionv1alpha1.BackupJobList{}
				Expect(k8sClient.List(ctx, &backupJobList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(backupJobList.Items) == 1
			}, timeout, interval).Should(BeTrue())

			By("Mocking VolumeSnapshot and set it as ReadyToUse")
			snapshotKey := types.NamespacedName{Name: fmt.Sprintf("%s-%s-scaling",
				key.Name, compName), Namespace: "default"}
			pvcName := fmt.Sprintf("%s-%s-%s-0", volumeName, key.Name, compName)
			volumeSnapshotYaml := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/created-by: kubeblocks
    app.kubernetes.io/instance: %s
    app.kubernetes.io/component-name: %s
spec:
  source:
    persistentVolumeClaimName: %s
`, snapshotKey.Name, snapshotKey.Namespace, key.Name, compName, pvcName)
			volumeSnapshot := snapshotv1.VolumeSnapshot{}
			Expect(yaml.Unmarshal([]byte(volumeSnapshotYaml), &volumeSnapshot)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, &volumeSnapshot)).Should(Succeed())
			readyToUse := true
			volumeSnapshotStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
			volumeSnapshot.Status = &volumeSnapshotStatus
			Expect(k8sClient.Status().Update(ctx, &volumeSnapshot)).Should(Succeed())

			By("Checking cluster status and the number of replicas changed")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.ObservedGeneration == 2).To(BeTrue())
			}, timeout, interval).Should(Succeed())
			stsList = listAndCheckStatefulSet(key)
			Expect(int(*stsList.Items[0].Spec.Replicas)).To(BeEquivalentTo(updatedReplicas))

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	// TODO move integration tests(which relies on a real K8s cluster) out of UT
	Context("When horizontal scaling in real env", func() {
		It("Should create backup resources accordingly", func() {
			useExistingCluster, _ := strconv.ParseBool(os.Getenv("USE_EXISTING_CLUSTER"))
			if !useExistingCluster {
				return
			}

			configTplKey := types.NamespacedName{Name: "test-mysql-3node-tpl-8.0", Namespace: "default"}
			configTplYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    meta.helm.sh/release-name: kubeblocks
    meta.helm.sh/release-namespace: default
  labels:
    app.kubernetes.io/managed-by: Helm
  name: %s
  namespace: %s
data:
  my.cnf: |-
    [mysqld]
    # aliyun buffer pool: https://help.aliyun.com/document_detail/162326.html?utm_content=g_1000230851&spm=5176.20966629.toubu.3.f2991ddcpxxvD1#title-rey-j7j-4dt
    
    {{- $log_root := getVolumePathByName ( index $.podSpec.containers 0 ) "log" }}
    {{- $data_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
    {{- $mysql_port_info := getPortByName ( index $.podSpec.containers 0 ) "mysql" }}
    {{- $pool_buffer_size := ( callBufferSizeByResource ( index $.podSpec.containers 0 ) ) }}


    {{- if $pool_buffer_size }}
    innodb-buffer-pool-size={{ $pool_buffer_size }}
    {{- end }}

    # require port
    {{- $mysql_port := 3306 }}

    log-bin=master-bin
    gtid_mode=OFF
    consensus_auto_leader_transfer=ON

    port={{ $mysql_port }}

    datadir={{ $data_root }}/data
    {{ if $log_root }}
    # Mysql error log
    log-error={{ $log_root }}/mysqld.err
    # SQL access log
    general_log=1
    general_log_file={{ $log_root }}/mysqld.log
    {{- end }}

    pid-file=/var/run/mysqld/mysqld.pid
    socket=/var/run/mysqld/mysqld.sock

    [client]
    port={{ $mysql_port }}
    socket=/var/run/mysqld/mysqld.sock
`, configTplKey.Name, configTplKey.Namespace)
			cm := &corev1.ConfigMap{}
			Expect(yaml.Unmarshal([]byte(configTplYAML), cm)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, cm)).Should(Succeed())

			By("Create real clusterdefinition")
			clusterDefKey := types.NamespacedName{Name: "test-apecloud-wesql"}
			clusterDefYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name: %s
spec:
  type: stat.mysql
  components:
    - typeName: replicasets
      characterType: mysql
      monitor:
        builtIn: false
      configTemplateRefs:
        - name: %s
          configTplRef: %s
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
        serviceAccountName: kubeblocks
        initContainers:
          - name: init
            image: lynnleelhl/kubectl:latest
            imagePullPolicy: IfNotPresent
            command: ["sh", "-c"]
            args:
            - |
              leader=$(cat /etc/podinfo/annotations | grep "cs.dbaas.kubeblocks.io/leader" | awk -F'"' '{print $2}')
              followers=$(cat /etc/podinfo/annotations | grep "cs.dbaas.kubeblocks.io/followers" | awk -F'"' '{print $2}')
              echo $leader
              echo $followers
              sub_follower=$(echo "$followers" | grep "$KB_POD_NAME")
              echo $KB_POD_NAME
              echo $sub_follower
              if [ -z "$leader" -o "$KB_POD_NAME" = "$leader" -o ! -z "$sub_follower" ]; then 
                exit 0;
              else 
                idx=${KB_POD_NAME##*-}
                host=$(eval echo \$KB_REPLICASETS_"$idx"_HOSTNAME)
                echo "$host"
                echo "kubectl exec -i $leader -c mysql -- bash -c \"mysql -e \"call dbms_consensus.add_follower('$host:13306');\" & pid=\$!; sleep 1; if ! ps \$pid > /dev/null; then wait \$pid; code=\$?; exit \$code; fi\""
                kubectl exec -i $leader -c mysql -- bash -c "mysql -e \"call dbms_consensus.add_follower('$host:13306');\" & pid=\$!; sleep 1; if ! ps \$pid > /dev/null; then wait \$pid; code=\$?; exit \$code; fi"
              fi
            volumeMounts:
              - mountPath: /etc/podinfo
                name: podinfo
        containers:
          - args:
              - |
                cluster_info=""; for (( i=0; i< $KB_REPLICASETS_N; i++ )); do
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
                host=$(eval echo \$KB_REPLICASETS_"$idx"_HOSTNAME)
                cluster_info="$cluster_info@$(($idx+1))"; 
                echo $cluster_info; 
                mkdir -p /data/mysql/data; 
                chmod +777 -R /data/mysql; 
                leader=$(cat /etc/podinfo/annotations | grep "cs.dbaas.kubeblocks.io/leader" | awk -F'"' '{print $2}')
                echo $leader
                if [ -z "$leader" ]; then
                  echo "docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info=\"$cluster_info\" --cluster-id=$CLUSTER_ID"
                  docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info="$cluster_info" --cluster-id=$CLUSTER_ID
                elif [ "$KB_POD_NAME" != "$leader" ]; then
                  echo "docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info=\"$host:13306\" --cluster-id=$CLUSTER_ID"
                  docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info="$host:13306" --cluster-id=$CLUSTER_ID
                else 
                  echo "docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info=\"$host:13306@1\" --cluster-id=$CLUSTER_ID"
                  docker-entrypoint.sh mysqld --defaults-file=/opt/mysql/my.cnf --cluster-start-index=$CLUSTER_START_INDEX --cluster-info="$host:13306@1" --cluster-id=$CLUSTER_ID
                fi
            command:
              - /bin/bash
              - -c
            env:
              - name: MYSQL_ROOT_USER
                value: root
              - name: MYSQL_ROOT_PASSWORD
                value: ""
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
                value: ""
              - name: MYSQL_TEMPLATE_CONFIG
                value: ""
              - name: MYSQL_CUSTOM_CONFIG
                value: ""
              - name: MYSQL_DYNAMIC_CONFIG
                value: ""
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
            volumeMounts:
              - mountPath: /data/mysql
                name: data
              - mountPath: /opt/mysql
                name: mysql-config
              - mountPath: /etc/podinfo
                name: podinfo
        volumes:
          - name: podinfo
            downwardAPI:
              items:
                - path: "annotations"
                  fieldRef:
                    fieldPath: metadata.annotations
`, clusterDefKey.Name, configTplKey.Name, configTplKey.Name)
			clusterDef := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDef)).Should(Succeed())
			clusterDef.Spec.Components[0].HorizontalScalePolicy =
				&dbaasv1alpha1.HorizontalScalePolicy{Type: dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot}
			Expect(testCtx.CheckedCreateObj(ctx, clusterDef)).Should(Succeed())

			By("Create real ClusterVersion")
			clusterVersionKey := types.NamespacedName{Name: "test-wesql-8.0.30"}
			clusterVersionYaml := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  labels:
    app.kubernetes.io/instance: kubeblocks
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: wesql
    app.kubernetes.io/version: 8.0.30
    clusterdefinition.kubeblocks.io/name: apecloud-wesql
    helm.sh/chart: wesql-0.1.1
  name: %s
spec:
  clusterDefinitionRef: %s
  components:
  - podSpec:
      containers:
      - image: apecloud/wesql-server:8.0.30-4.alpha1.20221031.g1aa54a3
        imagePullPolicy: IfNotPresent
        name: mysql
        resources: {}
    type: replicasets
`, clusterVersionKey.Name, clusterDefKey.Name)
			clusterVersion := &dbaasv1alpha1.ClusterVersion{}
			Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).Should(Succeed())

			clusterVersionList := dbaasv1alpha1.ClusterVersionList{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, &clusterVersionList, client.MatchingLabels{
					"clusterdefinition.kubeblocks.io/name": clusterDefKey.Name,
				}, client.InNamespace(clusterVersionKey.Namespace))).Should(Succeed())
				g.Expect(len(clusterVersionList.Items) > 0).To(BeTrue())
				g.Expect(clusterVersionList.Items[0].Status.Phase == "Available").To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("By creating a cluster")
			key := types.NamespacedName{Name: "test-wesql-01", Namespace: "default"}
			clusterYAML := fmt.Sprintf(`
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: %s
spec:
  clusterDefinitionRef: %s
  clusterVersionRef: %s
  components:
  - name: replicasets
    type: replicasets
    replicas: 1
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  terminationPolicy: Delete
`, key.Name, key.Namespace, clusterDefKey.Name, clusterVersionKey.Name)
			cluster := &dbaasv1alpha1.Cluster{}
			Expect(yaml.Unmarshal([]byte(clusterYAML), cluster)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, cluster)).Should(Succeed())

			backupPolicyTplKey := types.NamespacedName{Name: "test-backup-policy-template-mysql"}
			backupPolicyTemplateYaml := fmt.Sprintf(`
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupPolicyTemplate
metadata:
  name: %s
  labels:
    clusterdefinition.kubeblocks.io/name: %s
spec:
  schedule: "0 2 * * *"
  ttl: 168h0m0s
  # !!DISCUSS Number of backup retries on fail.
  onFailAttempted: 3
  hooks:
    ContainerName: mysql
    image: rancher/kubectl:v1.23.7
    preCommands:
    - touch /data/mysql/data/.restore; sync
  backupToolName: mysql-xtrabackup
`, backupPolicyTplKey.Name, clusterDefKey.Name)
			backupPolicyTemplate := dataprotectionv1alpha1.BackupPolicyTemplate{}
			Expect(yaml.Unmarshal([]byte(backupPolicyTemplateYaml), &backupPolicyTemplate)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, &backupPolicyTemplate)).Should(Succeed())

			for i := 0; i < 1; i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-%s-replicasets-%d
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ebs-sc
  volumeMode: Filesystem
  volumeName: pvc-6302ba88-ac70-4939-ac53-78a4bab00094
`, key.Name, i)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			}

			fetchedG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(stsList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			if useExistingCluster {
				podList := corev1.PodList{}
				Eventually(func() bool {
					Expect(k8sClient.List(ctx, &podList, client.MatchingLabels{
						"app.kubernetes.io/instance": key.Name,
					}, client.InNamespace(key.Namespace))).Should(Succeed())
					return len(podList.Items) == 1
				}, timeout, interval).Should(BeTrue())
			}

			By("By updating replica")
			updatedReplicas := int32(3)
			for i := *fetchedG1.Spec.Components[0].Replicas; i < updatedReplicas; i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-%s-replicasets-%d
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ebs-sc
  volumeMode: Filesystem
  volumeName: pvc-6302ba88-ac70-4939-ac53-78a4bab00094
`, key.Name, i)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			}
			fetchedG1.Spec.Components[0].Replicas = &updatedReplicas
			Expect(k8sClient.Update(ctx, fetchedG1)).Should(Succeed())

			Eventually(func() bool {
				backupJobList := dataprotectionv1alpha1.BackupJobList{}
				Expect(k8sClient.List(ctx, &backupJobList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(backupJobList.Items) == 1
			}, timeout, interval).Should(BeTrue())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout*2, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) != 0).Should(BeTrue())
				return *stsList.Items[0].Spec.Replicas == updatedReplicas
			}, timeout, interval).Should(BeTrue())

			updatedReplicas = 5
			for i := *fetchedG2.Spec.Components[0].Replicas; i < updatedReplicas; i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-%s-replicasets-%d
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ebs-sc
  volumeMode: Filesystem
  volumeName: pvc-6302ba88-ac70-4939-ac53-78a4bab00094
`, key.Name, i)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
			}
			fetchedG2.Spec.Components[0].Replicas = &updatedReplicas
			Expect(k8sClient.Update(ctx, fetchedG2)).Should(Succeed())

			fetchedG3 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedG3)
				return fetchedG3.Status.ObservedGeneration == 3
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				backupJobList := dataprotectionv1alpha1.BackupJobList{}
				Expect(k8sClient.List(ctx, &backupJobList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(backupJobList.Items) == 1
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) != 0).Should(BeTrue())
				return *stsList.Items[0].Spec.Replicas == updatedReplicas
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())

			By("Deleting ClusterVersion")
			Eventually(func() error {
				return deleteClusterVersionNWait(key)
			}, timeout, interval).Should(Succeed())

			By("Deleting ClusterDefinition")
			Eventually(func() error {
				return deleteClusterDefNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster with services", func() {
		It("Should create corresponding services correctly", func() {
			By("Creating a cluster")
			_, _, _, key := createClusterNCheck()

			By("Checking proxy should have external ClusterIP service")
			svcList1 := &corev1.ServiceList{}
			Expect(k8sClient.List(ctx, svcList1, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  key.Name,
				intctrlutil.AppComponentLabelKey: "proxy",
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			// TODO fix me later, proxy should not have internal headless service
			// Expect(len(svcList1.Items) == 1).Should(BeTrue())
			Expect(len(svcList1.Items) > 0).Should(BeTrue())
			var existsExternalClusterIP bool
			for _, svc := range svcList1.Items {
				Expect(svc.Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
				if svc.Spec.ClusterIP == corev1.ClusterIPNone {
					continue
				}
				existsExternalClusterIP = true
			}
			Expect(existsExternalClusterIP).To(BeTrue())

			By("Checking replicasets should have internal headless service")
			getHeadlessSvcPorts := func(name string) []corev1.ServicePort {
				fetched := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())

				comp, err := util.GetComponentDefByCluster(ctx, k8sClient, fetched, name)
				Expect(err).ShouldNot(HaveOccurred())

				var headlessSvcPorts []corev1.ServicePort
				for _, container := range comp.PodSpec.Containers {
					for _, port := range container.Ports {
						// be consistent with headless_service_template.cue
						headlessSvcPorts = append(headlessSvcPorts, corev1.ServicePort{
							Name:       port.Name,
							Protocol:   port.Protocol,
							Port:       port.ContainerPort,
							TargetPort: intstr.FromString(port.Name),
						})
					}
				}
				return headlessSvcPorts
			}

			svcList2 := &corev1.ServiceList{}
			Expect(k8sClient.List(ctx, svcList2, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey:  key.Name,
				intctrlutil.AppComponentLabelKey: "replicasets",
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			Expect(len(svcList2.Items) == 1).Should(BeTrue())
			Expect(svcList2.Items[0].Spec.Type == corev1.ServiceTypeClusterIP).To(BeTrue())
			Expect(svcList2.Items[0].Spec.ClusterIP == corev1.ClusterIPNone).To(BeTrue())
			Expect(reflect.DeepEqual(svcList2.Items[0].Spec.Ports,
				getHeadlessSvcPorts("replicasets"))).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When updating cluster PVC storage size", func() {
		It("Should update PVC request storage size accordingly", func() {

			By("Mock a StorageClass which allows resize")
			StorageClassYaml := `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
   name: sc-mock
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
`
			storageClass := &storagev1.StorageClass{}
			Expect(yaml.Unmarshal([]byte(StorageClassYaml), storageClass)).Should(Succeed())
			Expect(testCtx.CheckedCreateObj(ctx, storageClass)).Should(Succeed())

			By("Creating a cluster with volume claim")
			replicas := int32(2)
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			toCreate.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name:     "replicasets",
				Type:     "replicasets",
				Replicas: &replicas,
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{{
					Name: volumeName,
					Spec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						StorageClassName: &storageClass.Name,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					}},
				},
			}
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			Eventually(func(g Gomega) {
				fetchedG1 := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetchedG1)).To(Succeed())
				g.Expect(fetchedG1.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Checking the replicas")
			stsList := listAndCheckStatefulSet(key)
			sts := &stsList.Items[0]
			Expect(*sts.Spec.Replicas == replicas).Should(BeTrue())

			By("Mock PVCs in Bound Status")
			for i := 0; i < int(replicas); i++ {
				pvcYAML := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s-%s-%d
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: %s
`, volumeName, sts.Name, i, storageClass.Name)
				pvc := corev1.PersistentVolumeClaim{}
				Expect(yaml.Unmarshal([]byte(pvcYAML), &pvc)).Should(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())
				pvc.Status.Phase = corev1.ClaimBound // only bound pvc allows resize
				Expect(k8sClient.Status().Update(ctx, &pvc)).Should(Succeed())
			}

			By("Updating the PVC storage size")
			newStorageValue := resource.MustParse("2Gi")
			Expect(changeSpec(key, func(cluster *dbaasv1alpha1.Cluster) {
				comp := &cluster.Spec.Components[0]
				comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			})).Should(Succeed())

			By("Checking the resize operation finished")
			Eventually(func(g Gomega) {
				fetchedG2 := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetchedG2)).To(Succeed())
				g.Expect(fetchedG2.Status.ObservedGeneration == 2).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())

			By("Checking PVCs are resized")
			stsList = listAndCheckStatefulSet(key)
			for _, sts := range stsList.Items {
				for _, vct := range sts.Spec.VolumeClaimTemplates {
					for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
						pvc := &corev1.PersistentVolumeClaim{}
						pvcKey := types.NamespacedName{
							Namespace: key.Namespace,
							Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
						}
						Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
					}
				}
			}

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})

	// TODO move integration tests(which relies on a real K8s cluster) out of UT
	Context("When updating cluster PVC storage size in real K8s cluster", func() {
		It("Should update PVC request storage size accordingly", func() {
			By("Checking available storageclasses")
			scList := &storagev1.StorageClassList{}
			defaultStorageClass := &storagev1.StorageClass{}
			hasDefaultSC := false
			_ = k8sClient.List(ctx, scList)
			if len(scList.Items) == 0 {
				return
			}

			for _, sc := range scList.Items {
				annot := sc.Annotations
				if annot == nil {
					continue
				}
				if isDefaultStorageClassAnnotation(&sc) {
					defaultStorageClass = &sc
					hasDefaultSC = true
					break
				}
			}
			if !hasDefaultSC {
				defaultStorageClass = &scList.Items[0]
				err := assureDefaultStorageClassObj(defaultStorageClass)
				Expect(err).NotTo(HaveOccurred())
			}

			By("Creating a cluster with volume claim")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			toCreate.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name: "replicasets1",
				Type: "replicasets",
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
					{
						Name: volumeName,
						Spec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
					{
						Name: "log",
						Spec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							StorageClassName: &defaultStorageClass.Name,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			// this test required controller-manager component
			By("Checking controller-manager status")
			if !isCMAvailable() {
				By("The controller-manager is not available, test skipped")
				return
			}
			// TODO test the following contents in a real K8S cluster. testEnv is no controller-manager and scheduler components
			By("Checking the replicas")
			stsList := listAndCheckStatefulSet(key)
			sts := &stsList.Items[0]
			Expect(sts.Spec.Replicas).ShouldNot(BeNil())
			Expect(sts.Status.AvailableReplicas).To(Equal(*sts.Spec.Replicas))

			Eventually(func() bool {
				pvcList := &corev1.PersistentVolumeClaimList{}
				Expect(k8sClient.List(ctx, pvcList, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pvcList.Items) != 0
			}, timeout*6, interval).Should(BeTrue())

			comp := &fetchedG1.Spec.Components[0]
			newStorageValue := resource.MustParse("2Gi")
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			comp.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue

			Expect(k8sClient.Update(ctx, fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout*2, interval).Should(BeTrue())

			// sts := &appsv1.StatefulSet{}
			// stsKey := types.NamespacedName{
			// 	Namespace: key.Namespace,
			// 	Name: fmt.Sprintf("%s-%s-%s",
			// 		key.Name,
			// 		fetchedG2.Spec.Components[0].Type,
			// 		fetchedG2.Spec.Components[0].Name),
			// }
			// Expect(k8sClient.Get(ctx, stsKey, sts)).Should(Succeed())

			By("Checking the PVC")
			stsList = listAndCheckStatefulSet(key)
			for _, sts := range stsList.Items {
				for _, vct := range sts.Spec.VolumeClaimTemplates {
					for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
						pvc := &corev1.PersistentVolumeClaim{}
						pvcKey := types.NamespacedName{
							Namespace: key.Namespace,
							Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
						}
						Expect(k8sClient.Get(ctx, pvcKey, pvc)).Should(Succeed())
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
					}
				}
			}

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})

	mockPodsForConsensusTest := func(cluster *dbaasv1alpha1.Cluster, number int) []corev1.Pod {
		podYaml := `
apiVersion: v1
kind: Pod
metadata:
  labels:
    controller-revision-hash: mock-version
  name: my-name
  namespace: default
spec:
  containers:
  - args:
    command:
    - /bin/bash
    - -c
    env:
    - name: KB_POD_NAME
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: metadata.name
    - name: KB_REPLICASETS_N
      value: "3"
    - name: KB_REPLICASETS_0_HOSTNAME
      value: clusterepuglf-wesql-test-0
    - name: KB_REPLICASETS_1_HOSTNAME
      value: clusterepuglf-wesql-test-1
    - name: KB_REPLICASETS_2_HOSTNAME
      value: clusterepuglf-wesql-test-2
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
    volumeMounts:
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: kube-api-access-2rhsb
      readOnly: true
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  restartPolicy: Always
  serviceAccount: default
  serviceAccountName: default

  volumes:
  - name: kube-api-access-2rhsb
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          expirationSeconds: 3607
          path: token
      - configMap:
          items:
          - key: ca.crt
            path: ca.crt
          name: kube-root-ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
`
		pods := make([]corev1.Pod, 0)
		componentName := cluster.Spec.Components[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		for i := 0; i < number; i++ {
			pod := corev1.Pod{}
			Expect(yaml.Unmarshal([]byte(podYaml), &pod)).Should(Succeed())
			pod.Name = stsName + "-" + strconv.Itoa(i)
			pod.Labels[intctrlutil.AppInstanceLabelKey] = clusterName
			pod.Labels[intctrlutil.AppComponentLabelKey] = componentName
			pods = append(pods, pod)
		}

		return pods
	}

	mockRoleChangedEvent := func(key types.NamespacedName, sts *appsv1.StatefulSet) []corev1.Event {
		eventYaml := `
apiVersion: v1
kind: Event
metadata:
  name: myevent
  namespace: default
type: Warning
reason: Unhealthy
reportingComponent: ""
message: 'Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Follower"}'
involvedObject:
  apiVersion: v1
  fieldPath: spec.containers{kb-rolechangedcheck}
  kind: Pod
  name: wesql-main-2
  namespace: default
`
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		events := make([]corev1.Event, 0)
		for _, pod := range pods {
			event := corev1.Event{}
			Expect(yaml.Unmarshal([]byte(eventYaml), &event)).Should(Succeed())
			event.Name = pod.Name + "-event"
			event.InvolvedObject.Name = pod.Name
			event.InvolvedObject.UID = pod.UID
			events = append(events, event)
		}
		events[0].Message = `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Leader"}`
		return events
	}

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		names := make([]string, 0)
		for _, pod := range pods {
			names = append(names, pod.Name)
		}
		return names
	}

	Context("When creating cluster with componentType = Consensus", func() {
		It("Should success with: "+
			"1 pod with 'leader' role label set, "+
			"2 pods with 'follower' role label set,"+
			"1 service routes to 'leader' pod", func() {
			By("Creating a cluster with componentType = Consensus")
			replicas := 3

			toCreate, _, _, key := newClusterWithConsensusObj(nil, nil)
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Waiting for cluster creation")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			stsList := listAndCheckStatefulSet(key)
			sts := &stsList.Items[0]

			By("Creating mock pods in StatefulSet")
			pods := mockPodsForConsensusTest(toCreate, replicas)
			for _, pod := range pods {
				Expect(testCtx.CreateObj(ctx, &pod)).Should(Succeed())
				// mock the status to pass the isReady(pod) check in consensus_set
				pod.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
				Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
			}

			By("Creating mock role changed events")
			// pod.Labels[intctrlutil.RoleLabelKey] will be filled with the role
			events := mockRoleChangedEvent(key, sts)
			for _, event := range events {
				Expect(testCtx.CreateObj(ctx, &event)).Should(Succeed())
			}

			By("Checking pods' role are changed accordingly")
			Eventually(func(g Gomega) {
				pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
				g.Expect(err).To(Succeed())
				// should have 3 pods
				g.Expect(len(pods)).To(Equal(3))
				// 1 leader
				// 2 followers
				leaderCount, followerCount := 0, 0
				for _, pod := range pods {
					switch pod.Labels[intctrlutil.RoleLabelKey] {
					case leader:
						leaderCount++
					case follower:
						followerCount++
					}
				}
				g.Expect(leaderCount).Should(Equal(1))
				g.Expect(followerCount).Should(Equal(2))
			}, timeout, interval).Should(Succeed())

			By("Updating StatefulSet's status")
			sts.Status.UpdateRevision = "mock-version"
			sts.Status.Replicas = int32(replicas)
			sts.Status.AvailableReplicas = int32(replicas)
			sts.Status.CurrentReplicas = int32(replicas)
			sts.Status.ReadyReplicas = int32(replicas)
			sts.Status.ObservedGeneration = sts.Generation
			Expect(k8sClient.Status().Update(ctx, sts)).Should(Succeed())

			By("Checking pods' role are updated in cluster status")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				compName := fetched.Spec.Components[0].Name
				g.Expect(fetched.Status.Components != nil).To(BeTrue())
				g.Expect(fetched.Status.Components).To(HaveKey(compName))
				consensusStatus := fetched.Status.Components[compName].ConsensusSetStatus
				g.Expect(consensusStatus != nil).To(BeTrue())
				g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(len(consensusStatus.Followers) == 2).To(BeTrue())
				g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
			}, timeout, interval).Should(Succeed())

			By("Waiting the cluster be running")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.Phase == dbaasv1alpha1.RunningPhase).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	// TODO move integration tests(which relies on a real K8s cluster) out of UT
	Context("When creating cluster with componentType = Consensus in real K8s cluster", func() {
		It("Should success with: "+
			"1 pod with 'leader' role label set, "+
			"2 pods with 'follower' role label set,"+
			"1 service routes to 'leader' pod", func() {
			if !testCtx.UsingExistingCluster() {
				return
			}

			By("Creating a cluster with componentType = Consensus")
			toCreate, _, _, key := newClusterWithConsensusObj(nil, nil)
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Waiting the cluster is created")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).To(Succeed())
				g.Expect(fetched.Status.Phase == dbaasv1alpha1.RunningPhase).To(BeTrue())
			}, timeout*3, interval*5).Should(Succeed())

			By("Checking pods' role label")
			ip := getLocalIP()
			Expect(ip).ShouldNot(BeEmpty())

			observeRoleOfServiceLoop := func(svc *corev1.Service) string {
				kind := "svc"
				name := svc.Name
				port := svc.Spec.Ports[0].Port
				role := ""
				Eventually(func() bool {
					err := startPortForward(kind, name, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					time.Sleep(interval)
					role, err = observeRole(ip, port)
					if err != nil {
						_ = stopPortForward(name)
						return false
					}
					_ = stopPortForward(name)

					return true
				}, timeout*2, interval*1).Should(BeTrue())

				return role
			}

			stsList := listAndCheckStatefulSet(key)
			sts := &stsList.Items[0]
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
			Expect(err).To(Succeed())
			// should have 3 pods
			Expect(len(pods)).Should(Equal(3))
			// 1 leader
			// 2 followers
			leaderCount, followerCount := 0, 0
			for _, pod := range pods {
				switch pod.Labels[intctrlutil.RoleLabelKey] {
				case leader:
					leaderCount++
				case follower:
					followerCount++
				}
			}
			Expect(leaderCount).Should(Equal(1))
			Expect(followerCount).Should(Equal(2))

			By("Checking services status")
			// we should have 1 services
			svcList := &corev1.ServiceList{}
			Expect(k8sClient.List(ctx, svcList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			Expect(len(svcList.Items)).Should(Equal(1))
			svc := svcList.Items[0]
			// getRole should be leader through service
			Expect(observeRoleOfServiceLoop(&svc)).Should(Equal(leader))

			By("Deleting leader pod")
			leaderPod := &corev1.Pod{}
			for _, pod := range pods {
				if pod.Labels[intctrlutil.RoleLabelKey] == leader {
					leaderPod = &pod
					break
				}
			}
			Expect(k8sClient.Delete(ctx, leaderPod)).Should(Succeed())
			time.Sleep(interval * 2)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: sts.Namespace,
					Name:      sts.Name,
				}, sts)).To(Succeed())
				g.Expect(sts.Status.AvailableReplicas == 3).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			time.Sleep(interval * 2)
			Expect(observeRoleOfServiceLoop(&svc)).Should(Equal(leader))

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster with cluster affinity set", func() {
		It("Should create pod with cluster affinity", func() {
			By("Creating a cluster")
			topologyKey := "testTopologyKey"
			lableKey := "testNodeLabelKey"
			labelValue := "testLabelValue"
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Required,
				TopologyKeys:    []string{topologyKey},
				NodeLabels: map[string]string{
					lableKey: labelValue,
				},
			}
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Checking the Affinity and TopologySpreadConstraints")
			stsList := listAndCheckStatefulSet(key)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal(lableKey))
			Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.DoNotSchedule))
			Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(topologyKey))
			Expect(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).To(Equal(topologyKey))

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster with both cluster affinity and component affinity set", func() {
		It("Should observe the component affinity will override the cluster affinity", func() {
			By("Creating a cluster")
			clusterTopologyKey := "testClusterTopologyKey"
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Required,
				TopologyKeys:    []string{clusterTopologyKey},
			}
			compTopologyKey := "testComponentTopologyKey"
			toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "replicasets",
				Type: "replicasets",
				Affinity: &dbaasv1alpha1.Affinity{
					PodAntiAffinity: dbaasv1alpha1.Preferred,
					TopologyKeys:    []string{compTopologyKey},
				},
			})
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Checking the Affinity and the TopologySpreadConstraints")
			stsList := listAndCheckStatefulSet(key)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable).To(Equal(corev1.ScheduleAnyway))
			Expect(podSpec.TopologySpreadConstraints[0].TopologyKey).To(Equal(compTopologyKey))
			Expect(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster with cluster tolerations set", func() {
		It("Should create pods with cluster tolerations", func() {
			By("Creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			var tolerations []corev1.Toleration
			tolerationKey := "testClusterTolerationKey"
			tolerationValue := "testClusterTolerationValue"
			toCreate.Spec.Tolerations = append(tolerations, corev1.Toleration{
				Key:      tolerationKey,
				Value:    tolerationValue,
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			})
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Checking the tolerations")
			stsList := listAndCheckStatefulSet(key)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
			toleration := podSpec.Tolerations[0]
			Expect(toleration.Key == tolerationKey &&
				toleration.Value == tolerationValue).Should(BeTrue())
			Expect(toleration.Operator == corev1.TolerationOpEqual &&
				toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster with both cluster tolerations and component tolerations set", func() {
		It("Should observe the component tolerations will override the cluster tolerations", func() {
			By("Creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			var clusterTolerations []corev1.Toleration
			clusterTolerationKey := "testClusterTolerationKey"
			toCreate.Spec.Tolerations = append(clusterTolerations, corev1.Toleration{
				Key:      clusterTolerationKey,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			})

			var compTolerations []corev1.Toleration
			compTolerationKey := "testcompTolerationKey"
			compTolerationValue := "testcompTolerationValue"
			compTolerations = append(compTolerations, corev1.Toleration{
				Key:      compTolerationKey,
				Value:    compTolerationValue,
				Operator: corev1.TolerationOpEqual,
				Effect:   corev1.TaintEffectNoSchedule,
			})

			toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name:        "replicasets",
				Type:        "replicasets",
				Tolerations: compTolerations,
			})
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			By("Checking the tolerations")
			stsList := listAndCheckStatefulSet(key)
			podSpec := stsList.Items[0].Spec.Template.Spec
			Expect(len(podSpec.Tolerations) == 1).Should(BeTrue())
			toleration := podSpec.Tolerations[0]
			Expect(toleration.Key == compTolerationKey &&
				toleration.Value == compTolerationValue).Should(BeTrue())
			Expect(toleration.Operator == corev1.TolerationOpEqual &&
				toleration.Effect == corev1.TaintEffectNoSchedule).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	// TODO move integration tests(which relies on a real K8s cluster) out of UT
	Context("When creating cluster with components in real K8s", func() {
		It("Should create cluster with running status", func() {
			if !testCtx.UsingExistingCluster() {
				return
			}

			By("Checking the controller-manager status")
			if !isCMAvailable() {
				By("The controller-manager is not available, test skipped")
				return
			}
			// TODO test the following contents in a real K8S cluster. testEnv is no controller-manager and scheduler components
			By("Creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "replicasets",
				Type: "replicasets",
			})
			Expect(testCtx.CreateObj(ctx, toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				_ = k8sClient.Get(ctx, key, fetchedClusterG1)
				return fetchedClusterG1.Status.Components["replicasets"].Phase == dbaasv1alpha1.RunningPhase &&
					fetchedClusterG1.Status.Phase == dbaasv1alpha1.RunningPhase
			}, timeout, interval).Should(BeTrue())

			By("Deleting the Cluster")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})
})

const (
	// configurations to connect to Mysql, either a data source name represent by URL.
	connectionURLKey = "url"

	// other general settings for DB connections.
	maxIdleConnsKey    = "maxIdleConns"
	maxOpenConnsKey    = "maxOpenConns"
	connMaxLifetimeKey = "connMaxLifetime"
	connMaxIdleTimeKey = "connMaxIdleTime"
)

// Mysql represents MySQL output bindings.
type Mysql struct {
	db *sql.DB
}

// Init initializes the MySQL binding.
func (m *Mysql) Init(metadata map[string]string) error {
	p := metadata
	url, ok := p[connectionURLKey]
	if !ok || url == "" {
		return fmt.Errorf("missing MySql connection string")
	}

	db, err := initDB(url)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxIdleConnsKey, db.SetMaxIdleConns)
	if err != nil {
		return err
	}

	err = propertyToInt(p, maxOpenConnsKey, db.SetMaxOpenConns)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxIdleTimeKey, db.SetConnMaxIdleTime)
	if err != nil {
		return err
	}

	err = propertyToDuration(p, connMaxLifetimeKey, db.SetConnMaxLifetime)
	if err != nil {
		return err
	}

	err = db.Ping()
	if err != nil {
		return errors.Wrap(err, "unable to ping the DB")
	}

	m.db = db

	return nil
}

// Close will close the DB.
func (m *Mysql) Close() error {
	if m.db != nil {
		return m.db.Close()
	}

	return nil
}

func (m *Mysql) query(ctx context.Context, sql string) ([]interface{}, error) {
	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()

	result, err := m.jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}

	return result, nil
}

func propertyToInt(props map[string]string, key string, setter func(int)) error {
	if v, ok := props[key]; ok {
		if i, err := strconv.Atoi(v); err == nil {
			setter(i)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to int", key, v)
		}
	}

	return nil
}

func propertyToDuration(props map[string]string, key string, setter func(time.Duration)) error {
	if v, ok := props[key]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			setter(d)
		} else {
			return errors.Wrapf(err, "error converitng %s:%s to time duration", key, v)
		}
	}

	return nil
}

func initDB(url string) (*sql.DB, error) {
	if _, err := mysql.ParseDSN(url); err != nil {
		return nil, errors.Wrapf(err, "illegal Data Source Name (DNS) specified by %s", connectionURLKey)
	}

	db, err := sql.Open("mysql", url)
	if err != nil {
		return nil, errors.Wrap(err, "error opening DB connection")
	}

	return db, nil
}

func (m *Mysql) jsonify(rows *sql.Rows) ([]interface{}, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var ret []interface{}
	for rows.Next() {
		values := prepareValues(columnTypes)
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		r := m.convert(columnTypes, values)
		ret = append(ret, r)
	}

	return ret, nil
}

func prepareValues(columnTypes []*sql.ColumnType) []interface{} {
	types := make([]reflect.Type, len(columnTypes))
	for i, tp := range columnTypes {
		types[i] = tp.ScanType()
	}

	values := make([]interface{}, len(columnTypes))
	for i := range values {
		values[i] = reflect.New(types[i]).Interface()
	}

	return values
}

func (m *Mysql) convert(columnTypes []*sql.ColumnType, values []interface{}) map[string]interface{} {
	r := map[string]interface{}{}

	for i, ct := range columnTypes {
		value := values[i]

		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err == nil {
				value = interface{}(vv)
			}
		case *sql.RawBytes:
			// special case for sql.RawBytes, see https://github.com/go-sql-driver/mysql/blob/master/fields.go#L178
			switch ct.DatabaseTypeName() {
			case "VARCHAR", "CHAR", "TEXT", "LONGTEXT":
				value = string(*v)
			}
		}

		if value != nil {
			r[ct.Name()] = value
		}
	}

	return r
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func observeRole(ip string, port int32) (string, error) {
	url := "root@tcp(" + ip + ":" + strconv.Itoa(int(port)) + ")/information_schema?allowNativePasswords=true"
	sql := "select role from information_schema.wesql_cluster_local"
	mysql := &Mysql{}
	params := map[string]string{connectionURLKey: url}
	err := mysql.Init(params)
	if err != nil {
		return "", err
	}

	result, err := mysql.query(ctx, sql)
	if err != nil {
		return "", err
	}
	if len(result) != 1 {
		return "", errors.New("only one role should be observed")
	}
	row, ok := result[0].(map[string]interface{})
	if !ok {
		return "", errors.New("query result wrong type")
	}
	role, ok := row["role"].(string)
	if !ok {
		return "", errors.New("role parsing error")
	}
	if len(role) == 0 {
		return "", errors.New("got empty role")
	}

	err = mysql.Close()
	role = strings.ToLower(role)
	if err != nil {
		return role, err
	}

	return role, nil
}

func startPortForward(kind, name string, port int32) error {
	portStr := strconv.Itoa(int(port))
	cmd := exec.Command("bash", "-c", "kubectl port-forward "+kind+"/"+name+" --address 0.0.0.0 "+portStr+":"+portStr+" &")
	return cmd.Start()
}

func stopPortForward(name string) error {
	cmd := exec.Command("bash", "-c", "ps aux | grep port-forward | grep -v grep | grep "+name+" | awk '{print $2}' | xargs kill -9")
	return cmd.Run()
}
