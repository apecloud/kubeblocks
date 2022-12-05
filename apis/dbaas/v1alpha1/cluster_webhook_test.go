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
		clusterName               = "cluster-webhook-mysql"
		clusterDefinitionName     = "cluster-webhook-mysql-definition"
		sencondeClusterDefinition = "cluster-webhook-mysql-definition2"
		appVersionName            = "cluster-webhook-mysql-appversion"
		timeout                   = time.Second * 10
		interval                  = time.Second
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
			Expect(testCtx.CreateObj(ctx, cluster).Error()).To(ContainSubstring("not found"))

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())

			clusterDefSecond, _ := createTestClusterDefinitionObj(sencondeClusterDefinition)
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
			cluster.Spec.ClusterDefRef = sencondeClusterDefinition
			Expect(k8sClient.Update(ctx, cluster).Error()).To(ContainSubstring("spec.clusterDefinitionRef"))
			// restore
			cluster.Spec.ClusterDefRef = clusterDefinitionName

			By("By testing spec.components[?].type not found in clusterDefinitionRef")
			cluster.Spec.Components[0].Type = "replicaset"
			Expect(k8sClient.Update(ctx, cluster).Error()).To(ContainSubstring("is not found in ClusterDefinition.spec.components[*].typeName"))
			// restore
			cluster.Spec.Components[0].Type = "replicasets"

			// restore
			cluster.Spec.Components[0].Name = "replicasets"

			By("By updating spec.components[?].volumeClaimTemplates storage size, expect succeed")
			cluster.Spec.Components[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())

			By("By updating spec.components[?].volumeClaimTemplates[?].name, expect not succeed")
			cluster.Spec.Components[0].VolumeClaimTemplates[0].Name = "test"
			Expect(k8sClient.Update(ctx, cluster).Error()).To(ContainSubstring("volumeClaimTemplates is forbidden modification except for storage size."))

			By("By updating component resources")
			// restore test volume claim template name to data
			cluster.Spec.Components[0].VolumeClaimTemplates[0].Name = "data"
			cluster.Spec.Components[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("200Mi"),
				},
			}
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())
			cluster.Spec.Components[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":     resource.MustParse("100m"),
					"memory1": resource.MustParse("200Mi"),
				},
			}
			Expect(k8sClient.Update(ctx, cluster).Error()).To(ContainSubstring("resource key is not cpu or memory or hugepages- "))
			cluster.Spec.Components[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("200Mi"),
				},
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
			}
			Expect(k8sClient.Update(ctx, cluster).Error()).To(ContainSubstring("must be less than or equal to memory limit"))
			cluster.Spec.Components[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("80Mi")
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())
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
  - name: replicasets
    type: replicasets
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
