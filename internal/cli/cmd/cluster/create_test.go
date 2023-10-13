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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
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
	var clsMgr = &class.Manager{}

	Context("setMonitor", func() {
		var components []map[string]interface{}
		BeforeEach(func() {
			var component appsv1alpha1.ClusterComponentSpec
			component.Monitor = true
			components = generateComponents(component, 3)
		})

		It("set monitoring interval to 0 to disable monitor", func() {
			setMonitor(0, components)
			for _, c := range components {
				Expect(c[monitorKey]).ShouldNot(BeTrue())
			}
		})

		It("set monitoring interval to 15 to enable monitor", func() {
			setMonitor(15, components)
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
		streams := genericiooptions.IOStreams{
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

	checkComponent := func(comps []*appsv1alpha1.ClusterComponentSpec, storage string, replicas int32, cpu string, memory string, storageClassName string, compIndex int) {
		Expect(comps).ShouldNot(BeNil())
		Expect(len(comps)).Should(BeNumerically(">=", compIndex))

		comp := comps[compIndex]
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
		comps, err := buildClusterComp(cd, nil, clsMgr)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "20Gi", 1, "1", "1Gi", "", 0)
	})

	It("build default cluster component with environment", func() {
		viper.Set(types.CfgKeyClusterDefaultStorageSize, "5Gi")
		viper.Set(types.CfgKeyClusterDefaultReplicas, 1)
		viper.Set(types.CfgKeyClusterDefaultCPU, "2000m")
		viper.Set(types.CfgKeyClusterDefaultMemory, "2Gi")
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		comps, err := buildClusterComp(cd, nil, clsMgr)
		Expect(err).ShouldNot(HaveOccurred())
		checkComponent(comps, "5Gi", 1, "2", "2Gi", "", 0)
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
		comps, err := buildClusterComp(cd, setsMap, clsMgr)
		Expect(err).Should(Succeed())
		checkComponent(comps, "10Gi", 10, "10", "2Gi", "test", 0)

		setsMap[testing.ComponentDefName][keySwitchPolicy] = "invalid"
		cd.Spec.ComponentDefs[0].WorkloadType = appsv1alpha1.Replication
		_, err = buildClusterComp(cd, setsMap, clsMgr)
		Expect(err).Should(HaveOccurred())
	})

	It("build multiple cluster component with set values", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		cd, _ := cluster.GetClusterDefByName(dynamic, testing.ClusterDefName)
		setsMap := map[string]map[setKey]string{
			testing.ComponentDefName: {
				keyCPU:          "10",
				keyMemory:       "2Gi",
				keyStorage:      "10Gi",
				keyReplicas:     "10",
				keyStorageClass: "test",
			}, testing.ExtraComponentDefName: {
				keyCPU:          "5",
				keyMemory:       "1Gi",
				keyStorage:      "5Gi",
				keyReplicas:     "5",
				keyStorageClass: "test-other",
			},
		}
		comps, err := buildClusterComp(cd, setsMap, clsMgr)
		Expect(err).Should(Succeed())
		checkComponent(comps, "10Gi", 10, "10", "2Gi", "test", 0)
		checkComponent(comps, "5Gi", 5, "5", "1Gi", "test-other", 1)
		setsMap[testing.ComponentDefName][keySwitchPolicy] = "invalid"
		cd.Spec.ComponentDefs[0].WorkloadType = appsv1alpha1.Replication
		_, err = buildClusterComp(cd, setsMap, clsMgr)
		Expect(err).Should(HaveOccurred())
	})

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
	It("build component and set values map", func() {
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
		raw := []string{"engineType=mongo:NoSchedule"}
		res, err := util.BuildTolerations(raw)
		Expect(err).Should(BeNil())
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
		mockBackupInfo(dynamic, backupName, clusterName, nil, "")
		Expect(setBackup(o, components)).Should(Succeed())
	})

	It("test fillClusterMetadataFromBackup", func() {
		baseBackupName := "test-backup"
		logBackupName := "test-logfile-backup"
		clusterName := testing.ClusterName
		baseBackup := testing.FakeBackup(baseBackupName)
		logfileBackup := testing.FakeBackup(logBackupName)
		cluster := testing.FakeCluster("clusterName", testing.Namespace)
		dynamic := testing.FakeDynamicClient(baseBackup, logfileBackup, cluster)

		o := &CreateOptions{}
		o.Dynamic = dynamic
		o.Namespace = testing.Namespace
		o.RestoreTime = "Jun 16,2023 18:57:01 UTC+0800"
		o.Backup = logBackupName
		backupLogTime, _ := util.TimeParse(o.RestoreTime, time.Second)
		buildBackupLogTime := func(d time.Duration) string {
			return backupLogTime.Add(d).Format(time.RFC3339)
		}
		buildTimeRange := func(startTime, stopTime string) map[string]any {
			return map[string]any{
				"start": startTime,
				"end":   stopTime,
			}
		}
		mockBackupInfo(dynamic, baseBackupName, clusterName, buildTimeRange(buildBackupLogTime(-30*time.Second), buildBackupLogTime(-10*time.Second)), "snapshot")
		mockBackupInfo(dynamic, logBackupName, clusterName, buildTimeRange(buildBackupLogTime(-1*time.Minute), buildBackupLogTime(time.Minute)), "logfile")
		By("fill cluster from backup success")
		Expect(fillClusterInfoFromBackup(o, &cluster)).Should(Succeed())
		Expect(cluster.Spec.ClusterDefRef).Should(Equal(testing.ClusterDefName))
		Expect(cluster.Spec.ClusterVersionRef).Should(Equal(testing.ClusterVersionName))

		By("fill cluster definition does not match")
		o.ClusterDefRef = "test-not-match-cluster-definition"
		Expect(fillClusterInfoFromBackup(o, &cluster)).Should(HaveOccurred())
		o.ClusterDefRef = ""

		By("fill cluster version does not match")
		o.ClusterVersionRef = "test-not-match-cluster-version"
		Expect(fillClusterInfoFromBackup(o, &cluster)).Should(HaveOccurred())
	})

	It("test build backup config", func() {
		backupPolicyTemplate := testing.FakeBackupPolicyTemplate("backupPolicyTemplate-test", testing.ClusterDefName)
		backupPolicy := appsv1alpha1.BackupPolicy{
			BackupMethods: []v1alpha1.BackupMethod{
				{
					Name:            "volume-snapshot",
					SnapshotVolumes: boolptr.True(),
				},
				{
					Name: "xtrabackup",
				},
			},
		}
		backupPolicyTemplate.Spec.BackupPolicies = append(backupPolicyTemplate.Spec.BackupPolicies, backupPolicy)
		dynamic := testing.FakeDynamicClient(backupPolicyTemplate)

		o := &CreateOptions{}
		o.Cmd = NewCreateCmd(o.Factory, o.IOStreams)
		o.Dynamic = dynamic
		o.ClusterDefRef = testing.ClusterDefName
		cluster := testing.FakeCluster("clusterName", testing.Namespace)

		By("test backup is not set")
		Expect(o.buildBackupConfig(cluster)).To(Succeed())

		By("test backup enable")
		o.BackupEnabled = true
		Expect(o.Cmd.Flags().Set("backup-enabled", "true")).To(Succeed())
		Expect(o.buildBackupConfig(cluster)).To(Succeed())
		Expect(*o.BackupConfig.Enabled).Should(BeTrue())
		Expect(o.BackupConfig.Method).Should(Equal("volume-snapshot"))

		By("test backup with invalid method")
		o.BackupMethod = "invalid-method"
		Expect(o.Cmd.Flags().Set("backup-method", "invalid-method")).To(Succeed())
		Expect(o.buildBackupConfig(cluster)).To(HaveOccurred())

		By("test backup with xtrabackup method")
		o.BackupMethod = "xtrabackup"
		Expect(o.Cmd.Flags().Set("backup-method", "xtrabackup")).To(Succeed())
		Expect(o.buildBackupConfig(cluster)).To(Succeed())
		Expect(o.BackupConfig.Method).Should(Equal("xtrabackup"))

		By("test backup is with wrong cron expression")
		o.BackupCronExpression = "wrong-cron-expression"
		Expect(o.Cmd.Flags().Set("backup-cron-expression", "wrong-corn-expression"))
		Expect(o.buildBackupConfig(cluster)).To(HaveOccurred())

		By("test backup is with correct corn expression")
		o.BackupCronExpression = "0 0 * * *"
		Expect(o.Cmd.Flags().Set("backup-cron-expression", "0 0 * * *")).To(Succeed())
		Expect(o.buildBackupConfig(cluster)).To(Succeed())
		Expect(o.BackupConfig.CronExpression).Should(Equal("0 0 * * *"))
	})

	It("build multiple pvc in one cluster component", func() {
		testCases := []struct {
			pvcs         []string
			compDefNames []string
			expected     map[string][]map[storageKey]string
			success      bool
		}{
			{
				nil,
				nil,
				map[string][]map[storageKey]string{},
				true,
			},
			// --pvc all key
			{
				[]string{"type=comp1,name=data,size=10Gi,storageClass=localPath,mode=ReadWriteOnce"},
				[]string{"comp1", "comp2"},
				map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyType:         "comp1",
							storageKeyName:         "data",
							storageKeySize:         "10Gi",
							storageKeyStorageClass: "localPath",
							storageAccessMode:      "ReadWriteOnce",
						},
					},
				}, true,
			},
			// multiple components and don't specify the type,
			// the default type will be the first component.
			{
				[]string{"name=data,size=10Gi,storageClass=localPath,mode=ReadWriteOnce"},
				[]string{"comp1", "comp2"},
				map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyName:         "data",
							storageKeySize:         "10Gi",
							storageKeyStorageClass: "localPath",
							storageAccessMode:      "ReadWriteOnce",
						},
					},
				}, true,
			},
			// wrong key
			{
				[]string{"cpu=1,memory=2Gi"},
				[]string{"comp1"},
				nil,
				false,
			},
			// wrong component
			{

				[]string{"type=comp3,name=data,size=10Gi,storageClass=localPath,mode=ReadWriteOnce"},
				[]string{"comp1", "comp2"},
				nil,
				false,
			},
			// one component with multiple pvc
			{
				[]string{"type=comp1,name=data,size=10Gi,storageClass=localPath,mode=ReadWriteOnce", "type=comp1,name=log,size=5Gi,storageClass=localPath,mode=ReadWriteMany"},
				[]string{"comp1"},
				map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyType:         "comp1",
							storageKeyName:         "data",
							storageKeySize:         "10Gi",
							storageKeyStorageClass: "localPath",
							storageAccessMode:      "ReadWriteOnce",
						},
						map[storageKey]string{
							storageKeyType:         "comp1",
							storageKeyName:         "log",
							storageKeySize:         "5Gi",
							storageKeyStorageClass: "localPath",
							storageAccessMode:      "ReadWriteMany",
						},
					},
				}, true,
			},
			// multiple components with one pvc
			// it has the same effect as "--set type=comp1,storage=10Gi --set type=comp2,storage=5Gi"
			{
				[]string{"type=comp1,name=data,size=10Gi", "type=comp2,name=data,size=5Gi"},
				[]string{"comp1", "comp2", "comp3"},
				map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyType: "comp1",
							storageKeyName: "data",
							storageKeySize: "10Gi",
						},
					},
					"comp2": {
						map[storageKey]string{
							storageKeyType: "comp2",
							storageKeyName: "data",
							storageKeySize: "5Gi",
						},
					},
				}, true,
			},
			// multiple components, and some component with multiple pvcs
			{
				[]string{"type=comp1,name=data,size=10Gi", "type=comp1,name=log,size=5Gi", "type=comp2,name=data,size=5Gi"},
				[]string{"comp1", "comp2", "comp3"}, map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyType: "comp1",
							storageKeyName: "data",
							storageKeySize: "10Gi",
						},
						map[storageKey]string{
							storageKeyType: "comp1",
							storageKeyName: "log",
							storageKeySize: "5Gi",
						},
					},
					"comp2": {
						map[storageKey]string{
							storageKeyType: "comp2",
							storageKeyName: "data",
							storageKeySize: "5Gi",
						},
					},
				}, true,
			},
		}

		for _, t := range testCases {
			By(strings.Join(t.pvcs, " "))
			res, err := buildCompStorages(t.pvcs, mockCD(t.compDefNames))
			if t.success {
				Expect(err).Should(Succeed())
				Expect(reflect.DeepEqual(res, t.expected)).Should(BeTrue())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		}
	})

	It("rebuild clusterComponentSpec VolumeClaimTemplates by --pvc", func() {
		comps, err := buildClusterComp(mockCD([]string{"comp1", "comp2"}), nil, clsMgr)

		Expect(err).Should(Succeed())
		Expect(comps).ShouldNot(BeNil())
		testCases := []struct {
			describe             string
			pvcMaps              map[string][]map[storageKey]string
			clusterComponentSpec []*appsv1alpha1.ClusterComponentSpec
			expected             map[string][]appsv1alpha1.ClusterComponentVolumeClaimTemplate
		}{
			{"rebuild multiple pvc in one component",
				map[string][]map[storageKey]string{
					"comp1": {
						map[storageKey]string{
							storageKeyType: "comp1",
							storageKeyName: "data",
							storageKeySize: "10Gi",
						},
						map[storageKey]string{
							storageKeyType: "comp1",
							storageKeyName: "log",
							storageKeySize: "5Gi",
						},
					},
					"comp2": {
						map[storageKey]string{
							storageKeyType: "comp2",
							storageKeyName: "data",
							storageKeySize: "5Gi",
						},
					}},
				comps,
				map[string][]appsv1alpha1.ClusterComponentVolumeClaimTemplate{
					"comp1": {
						{
							Name: "data",
							Spec: appsv1alpha1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("10Gi"),
									},
								},
							},
						},
						{
							Name: "log",
							Spec: appsv1alpha1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("5Gi"),
									},
								},
							},
						},
					},
					"comp2": {
						{
							Name: "data",
							Spec: appsv1alpha1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("5Gi"),
									},
								},
							},
						},
					},
				},
			},
		}

		for _, t := range testCases {
			By(t.describe)
			res := rebuildCompStorage(t.pvcMaps, t.clusterComponentSpec)
			for _, spec := range res {
				Expect(reflect.DeepEqual(spec.VolumeClaimTemplates, t.expected[spec.Name])).Should(BeTrue())
			}
		}

	})

})
