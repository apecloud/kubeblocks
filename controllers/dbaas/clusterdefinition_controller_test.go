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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("ClusterDefinition Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 5

	var ctx = context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, the existence of old ones shall be found, which causes
		// new objects fail to create.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	clusterDefYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     mysql-cluster-definition
spec:
  type: state.mysql
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
      - name: mysql_exporter
        imagePullPolicy: IfNotPresent
        env:
          - name: MYSQL_ROOT_PASSWORD
            valueFrom:
              secretKeyRef:
                name: $(CONN_CREDENTIAL_SECRET_NAME)
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

	clusterVersionYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterVersion
metadata:
  name:     clusterversion-mysql-latest
spec:
  clusterDefinitionRef: mysql-cluster-definition
  components:
  - type: replicasets
    podSpec: 
      containers:
      - name: mysql
        image: docker.io/apecloud/wesql-server:latest
      - name: mysql_exporter
        image: "prom/mysqld-exporter:v0.14.0"
`

	assureCfgTplConfigMapObj := func(cmName, cmNs string) *corev1.ConfigMap {
		By("By assure an cm obj")

		By("Assuring an cm obj")
		cfgCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("config/configcm.yaml",
			testdata.WithNamespacedName(cmName, cmNs))
		Expect(err).Should(Succeed())
		cfgTpl, err := testdata.GetResourceFromTestData[dbaasv1alpha1.ConfigConstraint]("config/configtpl.yaml",
			testdata.WithNamespacedName(cmName, cmNs))
		Expect(err).Should(Succeed())

		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		// update phase
		patch := client.MergeFrom(cfgTpl.DeepCopy())
		cfgTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
		Expect(k8sClient.Status().Patch(context.Background(), cfgTpl, patch)).Should(Succeed())

		return cfgCM
	}

	Context("When updating clusterDefinition", func() {
		It("Should update status of clusterVersion at the same time", func() {
			By("By creating a clusterDefinition")
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
			// check reconciled finalizer and status
			Eventually(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Expect(k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterDefinition), cd)).To(Succeed())
				g.Expect(len(cd.Finalizers) > 0 &&
					cd.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("creating an clusterVersion")
			clusterVersion := &dbaasv1alpha1.ClusterVersion{}
			Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
			Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(Succeed())
			// check reconciled finalizer
			Eventually(func(g Gomega) {
				cv := &dbaasv1alpha1.ClusterVersion{}
				g.Expect(k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterVersion), cv)).To(Succeed())
				g.Expect(len(cv.Finalizers) > 0 &&
					cv.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("updating clusterDefinition's spec which then mark clusterVersion's status as OutOfSync")
			Expect(testdbaas.ChangeSpec(&testCtx, intctrlutil.GetNamespacedName(clusterDefinition),
				func(cd *dbaasv1alpha1.ClusterDefinition) {
					cd.Spec.Type = "state.redis"
				})).Should(Succeed())
			// check ClusterVersion.Status.ClusterDefSyncStatus to be OutOfSync
			Eventually(func(g Gomega) {
				cv := &dbaasv1alpha1.ClusterVersion{}
				g.Expect(k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterVersion), cv)).To(Succeed())
				g.Expect(cv.Status.ClusterDefSyncStatus == dbaasv1alpha1.OutOfSyncStatus).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When configmap template refs in clusterDefinition is invalid", func() {
		It("Should stop proceeding the status of clusterDefinition", func() {
			By("creating a clusterDefinition")
			cmName := "mysql-tree-node-template-8.0-test2"
			clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
			clusterDefinition.Name += "-for-test"
			clusterDefinition.Spec.Components[0].ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{
				ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:                cmName,
						ConfigTplRef:        cmName,
						ConfigConstraintRef: cmName,
						Namespace:           testCtx.DefaultNamespace,
						VolumeName:          "xxx",
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())

			By("check the reconciler won't update Status.ObservedGeneration if configmap doesn't exist.")

			// should use Consistently here, since cd.Status.ObservedGeneration is initialized to be zero,
			// we must watch the value for a while to tell it's not changed by the reconciler.
			Consistently(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Eventually(func() error {
					return k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterDefinition), cd)
				}, timeout, interval).Should(Succeed())
				g.Expect(cd.Status.ObservedGeneration == 0).To(BeTrue())
			}, waitDuration, interval).Should(Succeed())

			By("check the reconciler update Status.ObservedGeneration after configmap is created.")
			// create configmap
			assureCfgTplConfigMapObj(cmName, testCtx.DefaultNamespace)
			Eventually(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Expect(k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterDefinition), cd)).To(Succeed())
				g.Expect(cd.Status.ObservedGeneration == 1).To(BeTrue())

				// check labels and finalizers
				g.Expect(cd.Finalizers).ShouldNot(BeEmpty())
				configCMLabel := cfgcore.GenerateTPLUniqLabelKeyWithConfig(cmName)
				configTPLLabel := cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(cmName)
				g.Expect(cd.Labels[configCMLabel]).Should(BeEquivalentTo(cmName))
				g.Expect(cd.Labels[configTPLLabel]).Should(BeEquivalentTo(cmName))
			}, timeout, interval).Should(Succeed())

			By("check the reconciler update configmap.Finalizer after configmap is created.")
			Eventually(func(g Gomega) {
				cmObj := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      cmName,
				}, cmObj)).Should(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})

	// Validate the parameters by ClusterDefinition webhook
	// Context("When configmap template in clusterDefinition contains invalid parameter(e.g. VolumeName not exist)", func() {
	//	It("Should stop proceeding the status of clusterDefinition", func() {
	//		By("creating a clusterDefinition and an invalid configmap")
	//		clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
	//		Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
	//		cmName := "mysql-tree-node-template-8.0-volumename-not-exist"
	//		clusterDefinition.Name += "-volumename-not-exist"
	//		// missing VolumeName
	//		clusterDefinition.Spec.Components[0].ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{
	//			ConfigTemplateRefs: []dbaasv1alpha1.ConfigTemplate{
	//				{
	//					Name:         cmName,
	//					ConfigTplRef: cmName,
	//					Namespace:    testCtx.DefaultNamespace,
	//				},
	//			},
	//		}
	//		// create configmap
	//		assureCfgTplConfigMapObj(cmName, testCtx.DefaultNamespace)
	//		Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
	//
	//		By("check the reconciler won't update Status.ObservedGeneration")
	//		// should use Consistently here, since cd.Status.ObservedGeneration is initialized to be zero,
	//		// we must watch the value for a while to tell it's not changed by the reconciler.
	//		Consistently(func(g Gomega) {
	//			cd := &dbaasv1alpha1.ClusterDefinition{}
	//			g.Eventually(func() error {
	//				return k8sClient.Get(ctx, intctrlutil.GetNamespacedName(clusterDefinition), cd)
	//			}, timeout, interval).Should(Succeed())
	//			g.Expect(cd.Status.ObservedGeneration == 0).To(BeTrue())
	//		}, waitDuration, interval).Should(Succeed())
	//
	//		By("By deleting clusterDefinition")
	//		Expect(k8sClient.Delete(ctx, clusterDefinition)).Should(Succeed())
	//	})
	// })
})
