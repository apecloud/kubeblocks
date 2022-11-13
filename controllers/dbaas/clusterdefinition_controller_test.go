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
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var _ = Describe("ClusterDefinition Controller", func() {

	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.AppVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	clusterDefYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     mysql-cluster-definition
spec:
  type: state.mysql-8
  components:
  - typeName: replicasets
    componentType: Stateful
    defaultReplicas: 3
    characterType: mysql
    monitor:
      builtIn: false
      exporterConfig:
        scrapePort: 9104
        scrapePath: /metrics
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
                name: $(KB_SECRET_NAME)
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
      - name: mysql_exporter
        imagePullPolicy: IfNotPresent
        env:
          - name: MYSQL_ROOT_PASSWORD
            valueFrom:
              secretKeyRef:
                name: $(KB_SECRET_NAME)
                key: password
          - name: DATA_SOURCE_NAME
            value: "root:$(MYSQL_ROOT_PASSWORD)@(localhost:3306)/"
        ports:
          - containerPort: 9104
            protocol: TCP
            name: scrape
        livenessProbe:
          httpGet:
            path: /
            port: 9104
        readinessProbe:
          httpGet:
            path: /
            port: 9104
        resources:
          {}
`

	appVerYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     appversion-mysql-latest
spec:
  clusterDefinitionRef: mysql-cluster-definition
  components:
  - type: replicasets
    podSpec: 
      containers:
      - name: mysql
        image: registry.jihulab.com/apecloud/mysql-server/mysql/wesql-server-arm:latest
      - name: mysql_exporter
        image: "prom/mysqld-exporter:v0.14.0"
`

	assureCfgTplConfigMapObj := func(cmName, cmNs string) *corev1.ConfigMap {
		By("By assure an cm obj")

		configmapYAML, err := os.ReadFile("./testdata/mysql_configmap.yaml")
		Expect(err).Should(BeNil())
		Expect(configmapYAML).ShouldNot(BeNil())
		configTemplateYaml, err := os.ReadFile("./testdata/mysql_config_template.yaml")
		Expect(err).Should(BeNil())
		Expect(configTemplateYaml).ShouldNot(BeNil())

		cfgCM := &corev1.ConfigMap{}
		cfgTpl := &dbaasv1alpha1.ConfigurationTemplate{}
		Expect(yaml.Unmarshal(configmapYAML, cfgCM)).Should(Succeed())
		Expect(yaml.Unmarshal(configTemplateYaml, cfgTpl)).Should(Succeed())

		cfgCM.Name = cmName
		cfgCM.Namespace = cmNs
		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())

		cfgTpl.Name = cmName
		cfgTpl.Spec.TplRef = cmName
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())
		return cfgCM
	}

	Context("When updating clusterDefinition", func() {
		It("Should update status of appVersion at the same time", func() {
			By("By creating a clusterDefinition")
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
			createdClusterDef := &dbaasv1alpha1.ClusterDefinition{}
			// check reconciled finalizer and status
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterDefinition.Namespace,
					Name:      clusterDefinition.Name,
				}, createdClusterDef)
				if err != nil {
					return false
				}
				return len(createdClusterDef.Finalizers) > 0 &&
					createdClusterDef.Status.ObservedGeneration == 1
			}, time.Second*10, time.Second*1).Should(BeTrue())
			By("By creating an appVersion")
			appVersion := &dbaasv1alpha1.AppVersion{}
			Expect(yaml.Unmarshal([]byte(appVerYaml), appVersion)).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, appVersion)).Should(Succeed())
			createdAppVersion := &dbaasv1alpha1.AppVersion{}
			// check reconciled finalizer
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: appVersion.Namespace,
					Name:      appVersion.Name,
				}, createdAppVersion)
				if err != nil {
					return false
				}
				return len(createdAppVersion.Finalizers) > 0
			}, time.Second*10, time.Second*1).Should(BeTrue())
			By("By updating clusterDefinition's spec")
			createdClusterDef.Spec.Type = "state.mysql-7"
			Expect(k8sClient.Update(ctx, createdClusterDef)).Should(Succeed())
			// check appVersion.Status.ClusterDefSyncStatus to be OutOfSync
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: appVersion.Namespace,
					Name:      appVersion.Name,
				}, createdAppVersion)
				if err != nil {
					return false
				}
				return createdAppVersion.Status.ClusterDefSyncStatus == "OutOfSync"
			}, time.Second*10, time.Second*1).Should(BeTrue())

			By("By deleting clusterDefinition")
			Expect(k8sClient.Delete(ctx, createdAppVersion)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, createdClusterDef)).Should(Succeed())
		})
	})

	Context("When configmap template invalid", func() {
		It("Should invalid status of clusterDefinition", func() {
			By("By creating a clusterDefinition")

			cmName := "mysql-tree-node-template-8.0-test2"
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())

			clusterDefinition.Name += "-for-test"
			clusterDefinition.Spec.Components[0].ConfigTemplateRefs = []dbaasv1alpha1.ConfigTemplate{
				{
					Name:       cmName,
					Namespace:  testCtx.DefaultNamespace,
					VolumeName: "xxx",
				},
			}
			Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
			createdClusterDef := &dbaasv1alpha1.ClusterDefinition{}
			// check reconciled finalizer and status

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterDefinition.Namespace,
					Name:      clusterDefinition.Name,
				}, createdClusterDef)
				if err != nil {
					return false
				}
				return len(createdClusterDef.Finalizers) > 0 &&
					createdClusterDef.Status.ObservedGeneration == 1
			}, time.Second*10, time.Second*1).Should(BeFalse())

			// create configmap
			assureCfgTplConfigMapObj(cmName, testCtx.DefaultNamespace)

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterDefinition.Namespace,
					Name:      clusterDefinition.Name,
				}, createdClusterDef)
				if err != nil {
					return false
				}
				return len(createdClusterDef.Finalizers) > 0 &&
					createdClusterDef.Status.ObservedGeneration == 1
			}, time.Second*10, time.Second*1).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdClusterDef)).Should(Succeed())
		})
	})

	Context("When configmap template invalid parameter", func() {
		It("Should invalid status of clusterDefinition", func() {
			By("By creating a clusterDefinition")
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())

			cmName := "mysql-tree-node-template-8.0-test-failed"
			clusterDefinition.Name += "-for-failed-test"
			clusterDefinition.Spec.Components[0].ConfigTemplateRefs = []dbaasv1alpha1.ConfigTemplate{
				{
					Name:       cmName,
					Namespace:  testCtx.DefaultNamespace,
					VolumeName: "",
				},
			}

			// create configmap
			assureCfgTplConfigMapObj(cmName, testCtx.DefaultNamespace)

			Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
			createdClusterDef := &dbaasv1alpha1.ClusterDefinition{}
			// check reconciled finalizer and status
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Namespace: clusterDefinition.Namespace,
					Name:      clusterDefinition.Name,
				}, createdClusterDef)
				if err != nil {
					return false
				}
				return len(createdClusterDef.Finalizers) > 0 &&
					createdClusterDef.Status.ObservedGeneration == 1
			}, time.Second*10, time.Second*1).Should(BeFalse())

			Expect(k8sClient.Delete(ctx, clusterDefinition)).Should(Succeed())
		})
	})

})
