/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("cluster webhook", func() {
	var (
		randomStr               string
		clusterName             string
		clusterDefinitionName   string
		secondClusterDefinition string
		clusterVersionName      string
	)

	initParams := func() {
		randomStr = testCtx.GetRandomStr()
		clusterName = "cluster-webhook-mysql-" + randomStr
		clusterDefinitionName = "cluster-webhook-mysql-definition-" + randomStr
		secondClusterDefinition = "cluster-webhook-mysql-definition2-" + randomStr
		clusterVersionName = "cluster-webhook-mysql-clusterversion-" + randomStr
	}
	cleanupObjects := func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}
	BeforeEach(func() {
		initParams()
		cleanupObjects()
	})

	AfterEach(func() {
		cleanupObjects()
	})

	Context("When cluster create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing creating a new clusterDefinition when no clusterVersion and clusterDefinition")
			cluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			Expect(testCtx.CreateObj(ctx, cluster).Error()).To(ContainSubstring("not found"))

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())

			clusterDefSecond, _ := createTestClusterDefinitionObj(secondClusterDefinition)
			Expect(testCtx.CreateObj(ctx, clusterDefSecond)).Should(Succeed())

			// wait until ClusterDefinition created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)).Should(Succeed())

			By("By creating a new clusterVersion")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(Succeed())
			// wait until ClusterVersion created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterVersionName}, clusterVersion)).Should(Succeed())

			By("By creating a new Cluster")
			cluster, _ = createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			Expect(testCtx.CreateObj(ctx, cluster)).Should(Succeed())

			By("By testing update spec.clusterDefinitionRef")
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ClusterDefRef = secondClusterDefinition
			Expect(k8sClient.Patch(ctx, cluster, patch).Error()).To(ContainSubstring("spec.clusterDefinitionRef"))
			// restore
			cluster.Spec.ClusterDefRef = clusterDefinitionName

			By("By testing spec.components[?].type not found in clusterDefinitionRef")
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].ComponentDefRef = "replicaset"
			Expect(k8sClient.Patch(ctx, cluster, patch).Error()).To(ContainSubstring("componentDefRef is immutable"))
			// restore
			cluster.Spec.ComponentSpecs[0].ComponentDefRef = "replicasets"

			// restore
			cluster.Spec.ComponentSpecs[0].Name = "replicasets"

			By("By updating spec.components[?].volumeClaimTemplates storage size, expect succeed")
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			Expect(k8sClient.Patch(ctx, cluster, patch)).Should(Succeed())

			By("By updating spec.components[?].volumeClaimTemplates[?].name, expect not succeed")
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Name = "test"
			Expect(k8sClient.Patch(ctx, cluster, patch).Error()).To(ContainSubstring("volumeClaimTemplates is forbidden modification except for storage size."))

			By("By updating component resources")
			// restore test volume claim template name to data
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Name = "data"
			cluster.Spec.ComponentSpecs[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("200Mi"),
				},
			}
			Expect(k8sClient.Patch(ctx, cluster, patch)).Should(Succeed())
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":     resource.MustParse("100m"),
					"memory1": resource.MustParse("200Mi"),
				},
			}
			Expect(k8sClient.Patch(ctx, cluster, patch).Error()).To(ContainSubstring("resource key is not cpu or memory or hugepages- "))
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("200Mi"),
				},
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse("100m"),
					"memory": resource.MustParse("100Mi"),
				},
			}
			Expect(k8sClient.Patch(ctx, cluster, patch).Error()).To(ContainSubstring("must be less than or equal to memory limit"))
			patch = client.MergeFrom(cluster.DeepCopy())
			cluster.Spec.ComponentSpecs[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("80Mi")
			Expect(k8sClient.Patch(ctx, cluster, patch)).Should(Succeed())
		})
	})

	Context("tls validation", func() {
		BeforeEach(func() {
			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())

			// wait until ClusterDefinition created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefinitionName}, clusterDef)).Should(Succeed())

			By("By creating a new clusterVersion")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(Succeed())
			// wait until ClusterVersion created
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterVersionName}, clusterVersion)).Should(Succeed())
		})
		It("should assure tls fields setting properly", func() {
			By("creating cluster with nil issuer")
			cluster, _ := createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			cluster.Spec.ComponentSpecs[0].TLS = true
			Expect(testCtx.CreateObj(ctx, cluster)).ShouldNot(Succeed())

			By("creating cluster with nil secret ref")
			cluster.Spec.ComponentSpecs[0].Issuer = &Issuer{Name: IssuerUserProvided}
			Expect(testCtx.CreateObj(ctx, cluster)).ShouldNot(Succeed())

			By("creating cluster with KubeBlocks issuer")
			cluster.Spec.ComponentSpecs[0].Issuer = &Issuer{Name: IssuerKubeBlocks}
			Expect(testCtx.CreateObj(ctx, cluster)).Should(Succeed())

			By("creating cluster with UserProvided issuer and secret ref provided")
			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &Cluster{})
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			cluster, _ = createTestCluster(clusterDefinitionName, clusterVersionName, clusterName)
			cluster.Spec.ComponentSpecs[0].TLS = true
			cluster.Spec.ComponentSpecs[0].Issuer = &Issuer{
				Name: IssuerUserProvided,
				SecretRef: &TLSSecretRef{
					Name: "test-tls-secret",
					CA:   "ca.crt",
					Cert: "cert.crt",
					Key:  "key.crt",
				},
			}
			Expect(testCtx.CreateObj(ctx, cluster)).Should(Succeed())
		})
	})
})

func createTestCluster(clusterDefinitionName, clusterVersionName, clusterName string) (*Cluster, error) {
	clusterYaml := fmt.Sprintf(`
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: %s
  namespace: default
spec:
  clusterDefinitionRef: %s
  clusterVersionRef: %s
  componentSpecs:
  - name: replicasets
    componentDefRef: replicasets
    replicas: 1
    volumeClaimTemplates:
    - name: data
      spec:
        storageClassName: standard
        resources:
          requests:
            storage: 1Gi
  - name: proxy
    componentDefRef: proxy
    replicas: 1
`, clusterName, clusterDefinitionName, clusterVersionName)
	cluster := &Cluster{}
	err := yaml.Unmarshal([]byte(clusterYaml), cluster)
	cluster.Spec.TerminationPolicy = WipeOut
	return cluster, err
}
