/*
Copyright 2022 The KubeBlocks Authors

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

	"github.com/sethvargo/go-password/password"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("Cluster Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 3
	clusterObjKey := types.NamespacedName{
		Name:      "my-cluster",
		Namespace: "default",
	}

	checkedCreateObj := func(obj client.Object) error {
		if err := k8sClient.Create(context.Background(), obj); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}

	assureDefaultStorageClassObj := func() *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := `
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-hostpath-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
`
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(checkedCreateObj(sc)).Should(Succeed())
		return sc
	}

	assureCfgTplConfigMapObj := func(cmName string) *corev1.ConfigMap {
		By("By assure an cm obj")
		appVerYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-tree-node-template-8.0
  namespace: default
data:
  my.cnf: |-
    [mysqld]
    innodb-buffer-pool-size=512M
    log-bin=master-bin
    gtid_mode=OFF
    consensus_auto_leader_transfer=ON
    
    pid-file=/var/run/mysqld/mysqld.pid
    socket=/var/run/mysqld/mysqld.sock

    port=3306
    general_log=0
    server-id=1
    slow_query_log=0
    
    [client]
    socket=/var/run/mysqld/mysqld.sock
    host=localhost
`
		cfgCM := &corev1.ConfigMap{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), cfgCM)).Should(Succeed())
		Expect(checkedCreateObj(cfgCM)).Should(Succeed())
		return cfgCM
	}

	// config template对于了container的mountPath
	// configTemplateRefs:
	// 	 - name: mysql-tree-node-template-8.0
	//     volumeName: config1
	//   - name: mysql-tree-node2
	//     volumeName: config2
	// for containner
	// volumeMounts:
	//   #将my.cnf configmap mount到pod的指定目录下，/data/config
	//   #在pod中，会存在file: /data/config/my.cnf.override
	//   #polardb-x在entrypoint的脚本会将my.cnf.override合并到/data/mysql/conf/my.cnf文件中
	//   - mountPath: /data/config
	//     name: config1
	//   - mountPath: /etc/config
	//	   name: config2
	assureClusterDefObj := func() *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    configTemplateRefs: 
    - name: mysql-tree-node-template-8.0 
      volumeName: mysql-config
    roleGroups:
    - primary
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
                name: $(OPENDBAAS_MY_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$OPENDBAAS_REPLICASETS_PRIMARY_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$OPENDBAAS_REPLICASETS_PRIMARY_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$OPENDBAAS_MY_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            docker-entrypoint.sh mysqld --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
  - typeName: proxy
    roleGroups: ["proxy"]
    defaultReplicas: 1
    isStateless: true
    podSpec:
      containers:
      - name: nginx
  roleGroupTemplates:
  - typeName: primary
    defaultReplicas: 3
    updateStrategy:
      maxUnavailable: 1
  - typeName: proxy
    defaultReplicas: 2
`
		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
		Expect(yaml.Unmarshal([]byte(clusterDefYAML), clusterDefinition)).Should(Succeed())
		Expect(checkedCreateObj(clusterDefinition)).Should(Succeed())
		return clusterDefinition
	}

	assureAppVersionObj := func() *dbaasv1alpha1.AppVersion {
		By("By assure an appVersion obj")
		appVerYAML := `
apiVersion: dbaas.infracreate.com/v1alpha1
kind:       AppVersion
metadata:
  name:     app-version
spec:
  clusterDefinitionRef: cluster-definition
  components:
  - type: replicasets
    configTemplateRefs: 
    - name: mysql-tree-node-template-8.0 
      volumeName: mysql-config
    podSpec:
      containers:
      - name: mysql
        image: registry.jihulab.com/infracreate/mysql-server/mysql/wesql-server-arm:latest
  - type: proxy
    podSpec: 
      containers:
      - name: nginx
        image: nginx
`
		appVersion := &dbaasv1alpha1.AppVersion{}
		Expect(yaml.Unmarshal([]byte(appVerYAML), appVersion)).Should(Succeed())
		Expect(checkedCreateObj(appVersion)).Should(Succeed())
		return appVersion
	}

	newClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		appVersionObj *dbaasv1alpha1.AppVersion,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.AppVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and AppVersion objects if not provided
		if clusterDefObj == nil {
			assureCfgTplConfigMapObj("")
			clusterDefObj = assureClusterDefObj()
		}
		if appVersionObj == nil {
			appVersionObj = assureAppVersionObj()
		}

		randomStr, _ := password.Generate(6, 0, 0, true, false)
		key := types.NamespacedName{
			Name:      "cluster" + randomStr,
			Namespace: "default",
		}

		return &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "dbaas.infracreate.com/v1alpha1",
				Kind:       "Cluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{
				ClusterDefRef: clusterDefObj.GetName(),
				AppVersionRef: appVersionObj.GetName(),
			},
		}, clusterDefObj, appVersionObj, key
	}

	deleteClusterNWait := func(key types.NamespacedName) error {
		Expect(func() error {
			f := &dbaasv1alpha1.Cluster{}
			if err := k8sClient.Get(context.Background(), key, f); err != nil {
				return client.IgnoreNotFound(err)
			}
			return k8sClient.Delete(context.Background(), f)
		}()).Should(Succeed())

		var err error
		f := &dbaasv1alpha1.Cluster{}
		eta := time.Now().Add(waitDuration)
		for err = k8sClient.Get(context.Background(), key, f); err == nil && time.Now().Before(eta); err = k8sClient.Get(context.Background(), key, f) {
			f = &dbaasv1alpha1.Cluster{}
		}
		return client.IgnoreNotFound(err)
	}

	isCMAvailable := func() bool {
		csList := &corev1.ComponentStatusList{}
		_ = k8sClient.List(context.Background(), csList)
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

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		By("AfterEach scope")
		Eventually(func() error {
			return deleteClusterNWait(clusterObjKey)
		}, timeout, interval).Should(Succeed())
	})

	Context("When creating cluster", func() {
		It("Should success with no error", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When deleting cluster", func() {
		It("Should delete cluster resources according to termination policy", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)

			toCreate.Spec.TerminationPolicy = dbaasv1alpha1.DoNotTerminate

			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			fetchedG1.Spec.TerminationPolicy = dbaasv1alpha1.Halt
			Expect(k8sClient.Update(context.Background(), fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout, interval).Should(BeTrue())

			By("Deleting the cluster")
			Eventually(func() bool {
				if err := deleteClusterNWait(key); err != nil {
					return false
				}
				tmp := &dbaasv1alpha1.Cluster{}
				err := k8sClient.Get(context.Background(), key, tmp)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When updating cluster replicas", func() {
		It("Should create/delete pod to the replicas number", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(stsList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("By updating replica")
			if fetchedG1.Spec.Components == nil {
				fetchedG1.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			}
			updatedReplicas := 5
			fetchedG1.Spec.Components = append(fetchedG1.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "replicasets",
				Type: "replicasets",
				RoleGroups: []dbaasv1alpha1.ClusterRoleGroup{
					{
						Name:     "primary",
						Type:     "primary",
						Replicas: updatedReplicas,
					},
				},
			})
			Expect(k8sClient.Update(context.Background(), fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
				return fetchedG2.Status.ObservedGeneration == 2
			}, timeout*2, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) != 0).Should(BeTrue())
				return int(*stsList.Items[0].Spec.Replicas) == updatedReplicas
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create deployment if component is stateless", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			deployList := &appsv1.DeploymentList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), deployList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(deployList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create pdb if updateStrategy exists", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			pdbList := &policyv1.PodDisruptionBudgetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), pdbList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pdbList.Items) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should create service if service configured", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "proxy",
				Type: "proxy",
				RoleGroups: []dbaasv1alpha1.ClusterRoleGroup{
					{
						Name: "proxy",
						Type: "proxy",
						Service: corev1.ServiceSpec{
							Ports: []corev1.ServicePort{
								{
									Protocol:   "TCP",
									Port:       80,
									TargetPort: intstr.FromInt(8080),
								},
							},
							Type: "LoadBalancer",
						},
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			svcList := &corev1.ServiceList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), svcList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				for _, svc := range svcList.Items {
					if svc.Spec.Type == "LoadBalancer" {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When updating cluster", func() {
		It("Should update PVC request storage size accordingly", func() {
			By("Check available storageclasses")
			scList := &storagev1.StorageClassList{}
			defaultStorageClass := &storagev1.StorageClass{}
			hasDefaultSC := false
			_ = k8sClient.List(context.Background(), scList)
			for _, sc := range scList.Items {
				annot := sc.Annotations
				if annot == nil {
					continue
				}
				if v, ok := annot["storageclass.kubernetes.io/is-default-class"]; ok && v == "true" {
					defaultStorageClass = &sc
					hasDefaultSC = true
					break
				}
			}
			if !hasDefaultSC {
				defaultStorageClass = assureDefaultStorageClassObj()
			}

			By("By creating a cluster with volume claim")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = make([]dbaasv1alpha1.ClusterComponent, 1)
			toCreate.Spec.Components[0] = dbaasv1alpha1.ClusterComponent{
				Name: "replicasets1",
				Type: "replicasets",
				VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
					{
						Name: "data",
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
			Expect(k8sClient.Create(ctx, toCreate)).Should(Succeed())

			fetchedG1 := &dbaasv1alpha1.Cluster{}

			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG1)
				return fetchedG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			// this test required controller-manager component
			By("Check available controller-manager status")
			if !isCMAvailable() {
				By("The controller-manager is not available, test skipped")
				return
			}
			// TODO test the following contents in a real K8S cluster. testEnv is no controller-manager and scheduler components
			Eventually(func() bool {
				stsList := &appsv1.StatefulSetList{}
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())

				Expect(len(stsList.Items) == 1).Should(BeTrue())

				sts := &stsList.Items[0]
				Expect(sts.Spec.Replicas).ShouldNot(BeNil())
				return sts.Status.AvailableReplicas == *sts.Spec.Replicas
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				pvcList := &corev1.PersistentVolumeClaimList{}
				Expect(k8sClient.List(context.Background(), pvcList, client.InNamespace(key.Namespace))).Should(Succeed())
				return len(pvcList.Items) != 0
			}, timeout*6, interval).Should(BeTrue())

			comp := &fetchedG1.Spec.Components[0]
			newStorageValue := resource.MustParse("2Gi")
			comp.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue
			comp.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = newStorageValue

			Expect(k8sClient.Update(ctx, fetchedG1)).Should(Succeed())

			fetchedG2 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedG2)
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
			// Expect(k8sClient.Get(context.Background(), stsKey, sts)).Should(Succeed())

			stsList := &appsv1.StatefulSetList{}
			Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
				"app.kubernetes.io/instance": key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())

			for _, sts := range stsList.Items {
				for _, vct := range sts.Spec.VolumeClaimTemplates {
					for i := *sts.Spec.Replicas - 1; i >= 0; i-- {
						pvc := &corev1.PersistentVolumeClaim{}
						pvcKey := types.NamespacedName{
							Namespace: key.Namespace,
							Name:      fmt.Sprintf("%s-%s-%d", vct.Name, sts.Name, i),
						}
						Expect(k8sClient.Get(context.Background(), pvcKey, pvc)).Should(Succeed())
						Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(newStorageValue))
					}
				}
			}

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout*2, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should PodSpec affinity be nil if cluster affinity is nil", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)

			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) == 1).Should(BeTrue())
				sts := stsList.Items[0]
				return sts.Spec.Template.Spec.Affinity == nil
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should RequiredDuringSchedulingIgnoredDuringExecution and NoSchedule exist if podAntiAffinity is required", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Required,
				TopologyKeys:    []string{"testTopologyKey"},
				NodeLabels: map[string]string{
					"testNodeKey": "testLabelValue",
				},
			}
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) == 1).Should(BeTrue())
				podSpec := stsList.Items[0].Spec.Template.Spec
				Expect(podSpec.Affinity.NodeAffinity).ShouldNot(BeNil())
				Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable == corev1.DoNotSchedule).Should(BeTrue())
				return len(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should PreferredDuringSchedulingIgnoredDuringExecution and ScheduleAnyway exist if podAntiAffinity is preferred", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Preferred,
				TopologyKeys:    []string{"testTopologyKey"},
			}
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) == 1).Should(BeTrue())
				podSpec := stsList.Items[0].Spec.Template.Spec
				Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable == corev1.ScheduleAnyway).Should(BeTrue())
				return len(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should PreferredDuringSchedulingIgnoredDuringExecution and ScheduleAnyway exist if podAntiAffinity is preferred", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Preferred,
				TopologyKeys:    []string{"testTopologyKey"},
			}
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) == 1).Should(BeTrue())
				podSpec := stsList.Items[0].Spec.Template.Spec
				Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable == corev1.ScheduleAnyway).Should(BeTrue())
				return len(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When creating cluster", func() {
		It("Should compoment affinity overrider cluster affinity if component affinity exists", func() {
			By("By creating a cluster")
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Affinity = &dbaasv1alpha1.Affinity{
				PodAntiAffinity: dbaasv1alpha1.Preferred,
				TopologyKeys:    []string{"testTopologyKey"},
			}
			toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "replicasets",
				Type: "replicasets",
				Affinity: &dbaasv1alpha1.Affinity{
					PodAntiAffinity: dbaasv1alpha1.Required,
					TopologyKeys:    []string{"testTopologyKey"},
				},
			})
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())

			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				Expect(k8sClient.List(context.Background(), stsList, client.MatchingLabels{
					"app.kubernetes.io/instance": key.Name,
				}, client.InNamespace(key.Namespace))).Should(Succeed())
				Expect(len(stsList.Items) == 1).Should(BeTrue())
				podSpec := stsList.Items[0].Spec.Template.Spec
				Expect(len(podSpec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) == 0).Should(BeTrue())
				Expect(podSpec.TopologySpreadConstraints[0].WhenUnsatisfiable == corev1.DoNotSchedule).Should(BeTrue())
				return len(podSpec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 0
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("testing cluster status", func() {
		It("this test required controller-manager component", func() {
			if !isCMAvailable() {
				By("The controller-manager is not available, test skipped")
				return
			}
			// TODO test the following contents in a real K8S cluster. testEnv is no controller-manager and scheduler components
			toCreate, _, _, key := newClusterObj(nil, nil)
			toCreate.Spec.Components = []dbaasv1alpha1.ClusterComponent{}
			toCreate.Spec.Components = append(toCreate.Spec.Components, dbaasv1alpha1.ClusterComponent{
				Name: "replicasets",
				Type: "replicasets",
				RoleGroups: []dbaasv1alpha1.ClusterRoleGroup{
					{
						Name:     "primary",
						Type:     "primary",
						Replicas: 1,
					},
				},
			})
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			fetchedClusterG1 := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.ObservedGeneration == 1
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				_ = k8sClient.Get(context.Background(), key, fetchedClusterG1)
				return fetchedClusterG1.Status.Components["replicasets"].Phase == dbaasv1alpha1.RunningPhase &&
					fetchedClusterG1.Status.Phase == dbaasv1alpha1.RunningPhase
			}, timeout, interval).Should(BeTrue())

			By("Deleting the scope")
			Eventually(func() error {
				return deleteClusterNWait(key)
			}, timeout, interval).Should(Succeed())
		})
	})
})
