/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

func generateComponents(component appsv1alpha1.ClusterComponentSpec, count int) []map[string]interface{} {
	var componentVals []map[string]interface{}
	byteVal, err := json.Marshal(component)
	Expect(err).ShouldNot(HaveOccurred())
	for i := 0; i < count; i++ {
		var componentVal map[string]interface{}
		err = json.Unmarshal(byteVal, &componentVal)
		Expect(err).ShouldNot(HaveOccurred())
		componentVals = append(componentVals, componentVal)
	}
	Expect(len(componentVals)).To(Equal(count))
	return componentVals
}

func getResource(res corev1.ResourceRequirements, name corev1.ResourceName) interface{} {
	return res.Requests[name].ToUnstructured()
}

var _ = Describe("create", func() {
	var componentClasses map[string]map[string]*appsv1alpha1.ComponentClassInstance

	Context("setMonitor", func() {
		var components []map[string]interface{}
		BeforeEach(func() {
			var component appsv1alpha1.ClusterComponentSpec
			component.Monitor = true
			components = generateComponents(component, 3)
		})

		It("set monitor param to false", func() {
			setMonitor(false, components)
			for _, c := range components {
				Expect(c[monitorKey]).ShouldNot(BeTrue())
			}
		})

		It("set monitor param to true", func() {
			setMonitor(true, components)
			for _, c := range components {
				Expect(c[monitorKey]).Should(BeTrue())
			}
		})
	})

	Context("setEnableAllLogs Test", func() {
		var cluster *appsv1alpha1.Cluster
		var clusterDef *appsv1alpha1.ClusterDefinition
		BeforeEach(func() {
			cluster = testing.FakeCluster("log", "test")
			clusterDef = testing.FakeClusterDef()
			Expect(cluster.Spec.ComponentSpecs[0].EnabledLogs).Should(BeNil())
		})
		It("no logConfigs in ClusterDef", func() {
			setEnableAllLogs(cluster, clusterDef)
			Expect(len(cluster.Spec.ComponentSpecs[0].EnabledLogs)).Should(Equal(0))
		})
		It("set logConfigs in ClusterDef", func() {
			clusterDef.Spec.ComponentDefs[0].LogConfigs = []appsv1alpha1.LogConfig{
				{
					Name:            "error",
					FilePathPattern: "/log/mysql/mysqld.err",
				},
				{
					Name:            "slow",
					FilePathPattern: "/log/mysql/*slow.log",
				},
			}
			setEnableAllLogs(cluster, clusterDef)
			Expect(cluster.Spec.ComponentSpecs[0].EnabledLogs).Should(Equal([]string{"error", "slow"}))
		})
	})

	Context("multipleSourceComponent test", func() {
		defer GinkgoRecover()
		streams := genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stdout,
		}
		It("target file stored in website", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte("OK"))
				Expect(err).ShouldNot(HaveOccurred())
			}))
			defer ts.Close()
			fileURL := ts.URL + "/docs/file"
			bytes, err := MultipleSourceComponents(fileURL, streams.In)
			Expect(bytes).Should(Equal([]byte("OK")))
			Expect(err).ShouldNot(HaveOccurred())
		})
		It("target file doesn't exist", func() {
			fileName := "no-existing-file"
			bytes, err := MultipleSourceComponents(fileName, streams.In)
			Expect(bytes).Should(BeNil())
			Expect(err).Should(HaveOccurred())
		})
	})

	checkComponent := func(comps []*appsv1alpha1.ClusterComponentSpec, storage string, replicas int32, cpu string, memory string, storageClassName string) {
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(Equal(2))

		comp := comps[0]
		Expect(getResource(comp.VolumeClaimTemplates[0].Spec.Resources, corev1.ResourceStorage)).Should(Equal(storage))
		Expect(comp.Replicas).Should(BeEquivalentTo(replicas))

		resources := comp.Resources
		Expect(resources).ShouldNot(BeNil())
		Expect(getResource(resources, corev1.ResourceCPU)).Should(Equal(cpu))
		Expect(getResource(resources, corev1.ResourceMemory)).Should(Equal(memory))

		if storageClassName == "" {
			Expect(comp.VolumeClaimTemplates[0].Spec.StorageClassName).Should(BeNil())
		} else {
			Expect(*comp.VolumeClaimTemplates[0].Spec.StorageClassName).Should(Equal(storageClassName))
		}

	}

	It("build default cluster component without environment", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		comps, err := buildClusterComp(cd, nil, componentClasses)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "20Gi", 1, "1", "1Gi", "")
	})

	It("build default cluster component with environment", func() {
		viper.Set("CLUSTER_DEFAULT_STORAGE_SIZE", "5Gi")
		viper.Set("CLUSTER_DEFAULT_REPLICAS", 1)
		viper.Set("CLUSTER_DEFAULT_CPU", "2000m")
		viper.Set("CLUSTER_DEFAULT_MEMORY", "2Gi")
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		comps, err := buildClusterComp(cd, nil, componentClasses)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "5Gi", 1, "2", "2Gi", "")
	})

	It("build cluster component with set values", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		setsMap := map[string]map[setKey]string{
			testing.ComponentDefName: {
				keyCPU:          "10",
				keyMemory:       "2Gi",
				keyStorage:      "10Gi",
				keyReplicas:     "10",
				keyStorageClass: "test",
			},
		}
		comps, err := buildClusterComp(cd, setsMap, componentClasses)
		Expect(err).Should(Succeed())
		checkComponent(comps, "10Gi", 10, "10", "2Gi", "test")

		setsMap[testing.ComponentDefName][keySwitchPolicy] = "invalid"
		cd.Spec.ComponentDefs[0].WorkloadType = appsv1alpha1.Replication
		_, err = buildClusterComp(cd, setsMap, componentClasses)
		Expect(err).Should(HaveOccurred())
	})

	It("build component and set values map", func() {
		mockCD := func(compDefNames []string) *appsv1alpha1.ClusterDefinition {
			cd := &appsv1alpha1.ClusterDefinition{}
			var comps []appsv1alpha1.ClusterComponentDefinition
			for _, n := range compDefNames {
				comp := appsv1alpha1.ClusterComponentDefinition{
					Name:         n,
					WorkloadType: appsv1alpha1.Replication,
				}
				comps = append(comps, comp)
			}
			cd.Spec.ComponentDefs = comps
			return cd
		}

		testCases := []struct {
			values       []string
			compDefNames []string
			expected     map[string]map[setKey]string
			success      bool
		}{
			{
				nil,
				nil,
				map[string]map[setKey]string{},
				true,
			},
			{
				[]string{"cpu=1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU: "1",
					},
				},
				true,
			},
			{
				[]string{"cpu=1,memory=2Gi,storage=10Gi,class=general-1c2g"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
						keyClass:   "general-1c2g",
					},
				},
				true,
			},
			// values with unknown set key that will be ignored
			{
				[]string{"cpu=1,memory=2Gi,storage=10Gi,t1,t1=v1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
				false,
			},
			// values with type
			{
				[]string{"type=comp,cpu=1,memory=2Gi,storage=10Gi,t1,t1=v1"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"comp": {
						keyType:    "comp",
						keyCPU:     "1",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
				false,
			},
			// set more than one time
			{
				[]string{"cpu=1,memory=2Gi", "storage=10Gi,cpu=2"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyCPU:     "2",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
				true,
			},
			{
				[]string{"type=my-comp,cpu=1,memory=2Gi", "storage=10Gi,cpu=2"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyType:    "my-comp",
						keyCPU:     "2",
						keyMemory:  "2Gi",
						keyStorage: "10Gi",
					},
				},
				true,
			},
			{
				[]string{"type=comp1,cpu=1,memory=2Gi,class=general-2c4g", "type=comp2,storage=10Gi,cpu=2,class=mo-1c8g,replicas=3"},
				[]string{"comp1", "comp2"},
				map[string]map[setKey]string{
					"comp1": {
						keyType:   "comp1",
						keyCPU:    "1",
						keyMemory: "2Gi",
						keyClass:  "general-2c4g",
					},
					"comp2": {
						keyType:     "comp2",
						keyCPU:      "2",
						keyStorage:  "10Gi",
						keyClass:    "mo-1c8g",
						keyReplicas: "3",
					},
				},
				true,
			},
			{
				[]string{"switchPolicy=MaximumAvailability"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keySwitchPolicy: "MaximumAvailability",
					},
				},
				true,
			},
			{
				[]string{"storageClass=test"},
				[]string{"my-comp"},
				map[string]map[setKey]string{
					"my-comp": {
						keyStorageClass: "test",
					},
				},
				true,
			},
		}

		for _, t := range testCases {
			By(strings.Join(t.values, " "))
			res, err := buildCompSetsMap(t.values, mockCD(t.compDefNames))
			if t.success {
				Expect(err).Should(Succeed())
				Expect(reflect.DeepEqual(res, t.expected)).Should(BeTrue())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		}
	})

	It("build tolerations", func() {
		raw := []string{"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"}
		res := buildTolerations(raw)
		Expect(len(res)).Should(Equal(1))
	})

	It("generate random cluster name", func() {
		dynamic := testing.FakeDynamicClient()
		name, err := generateClusterName(dynamic, "")
		Expect(err).Should(Succeed())
		Expect(name).ShouldNot(BeEmpty())
	})

	It("set backup", func() {
		backupName := "test-backup"
		clusterName := "test-cluster"
		backup := testing.FakeBackup(backupName)
		cluster := testing.FakeCluster("clusterName", testing.Namespace)
		dynamic := testing.FakeDynamicClient(backup, cluster)
		o := &CreateOptions{}
		o.Dynamic = dynamic
		o.Namespace = testing.Namespace
		o.Backup = backupName
		components := []map[string]interface{}{
			{
				"name": "mysql",
			},
		}
		Expect(setBackup(o, components).Error()).Should(ContainSubstring("is not completed"))

		By("test backup is completed")
		mockBackupInfo(dynamic, backupName, clusterName, nil)
		Expect(setBackup(o, components)).Should(Succeed())
	})
})
