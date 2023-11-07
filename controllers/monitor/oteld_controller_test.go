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

package monitor

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/reconcile"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var OTeldSignature = func(_ v1alpha1.OTeld, _ *v1alpha1.OTeld, _ v1alpha1.OTeldList, _ *v1alpha1.OTeldList) {}

const (
	logsSinkName    = "loki"
	metricsSinkName = "prometheus"

	mysqlCompDefName = "replicasets"
	mysqlCompName    = "mysql"

	clusterDefName     = "test-clusterdef"
	clusterVersionName = "test-clusterversion"
	clusterName        = "test-cluster"
)

var _ = Describe("OTeld Monitor Controller", func() {

	Context("OTeld Controller", func() {

		var cluster *appsv1alpha1.Cluster

		loadExpectConfig := func(file string) map[string]any {
			var m map[string]any
			b, err := testdata.GetTestDataFileContent(file)
			Expect(err).Should(Succeed())
			Expect(yaml.Unmarshal(b, &m)).Should(Succeed())
			return m
		}

		validateEngineConfig := func(content string) {
			var m map[string]any
			Expect(yaml.Unmarshal([]byte(content), &m)).Should(Succeed())
			Expect(reflect.DeepEqual(m, loadExpectConfig("monitor/otel.engine.yaml"))).Should(BeTrue())
		}

		validateOteldConfig := func(content string) {
			var m map[string]any
			Expect(yaml.Unmarshal([]byte(content), &m)).Should(Succeed())
			Expect(reflect.DeepEqual(m, loadExpectConfig("monitor/otel.config.yaml"))).Should(BeTrue())
		}

		BeforeEach(func() {
			clusterDef := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				AddLogConfig("error", "/data/mysql/log/mysqld-error.log").
				AddLogConfig("slow", "/data/mysql/log/mysqld-slowquery.log").
				AddLogConfig("general", "/data/mysql/log/mysqld.log").
				Create(&testCtx).
				GetObject()
			clusterVersion := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				Create(&testCtx).
				GetObject()
		})

		It("reconcile", func() {
			otled := mockOTeldInstance()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, cluster *appsv1alpha1.Cluster) {
					Expect(cluster.Name).Should(Equal(clusterName))
				}), time.Second*30, time.Second*1).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(otled),
				func(g Gomega, oteld *v1alpha1.OTeld) {
					g.Expect(oteld.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}), time.Second*30, time.Second*1).Should(Succeed())

			By("check oteld config.yaml")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      fmt.Sprintf(reconcile.OteldConfigMapNamePattern, v1alpha1.ModeDaemonSet),
			}, func(g Gomega, cm *corev1.ConfigMap) {
				Expect(len(cm.Data)).Should(Equal(1))
				Expect(cm.Data).Should(HaveKey("config.yaml"))
				validateOteldConfig(cm.Data["config.yaml"])
			}), time.Second*30, time.Second*1).Should(Succeed())

			By("check oteld kb_engine.yaml")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      fmt.Sprintf(reconcile.OteldEngineConfigMapNamePattern, v1alpha1.ModeDaemonSet),
			}, func(g Gomega, cm *corev1.ConfigMap) {
				Expect(len(cm.Data)).Should(Equal(1))
				Expect(cm.Data).Should(HaveKey("kb_engine.yaml"))
				validateEngineConfig(cm.Data["kb_engine.yaml"])
			}), time.Second*30, time.Second*1).Should(Succeed())
		})
	})
})

func mockOTeldInstance() *v1alpha1.OTeld {

	oteld := &v1alpha1.OTeld{
		Spec: v1alpha1.OTeldSpec{
			Image: "docker.io/apecloud/oteld:0.1.0-beta.1",
			Batch: &v1alpha1.Batch{
				Enabled: true,
				Config: &v1alpha1.BatchConfig{
					Timeout:       "15s",
					SendBatchSize: 100,
				},
			},
			CollectionInterval: "15s",
			MetricsPort:        8888,
			SystemDataSource: &v1alpha1.SystemDataSource{
				PodLogs: &v1alpha1.PodLogs{
					Enabled: true,
				},
				NodeExporter: &v1alpha1.NodeExporter{
					Enabled: true,
				},
				K8sKubeletExporter: &v1alpha1.K8sKubeletExporter{
					Enabled: true,
				},
				K8sClusterExporter: &v1alpha1.K8sClusterExporter{
					Enabled: true,
				},
				MetricsExporterRef: []string{"prometheus"},
				LogsExporterRef:    []string{"loki"},
				CollectionInterval: "15s",
			},
		},
	}
	oteld.SetName("oteld-test")
	oteld.SetNamespace(testCtx.DefaultNamespace)
	oteld = testapps.CreateK8sResource(&testCtx, oteld).(*v1alpha1.OTeld)

	datasource := v1alpha1.CollectorDataSource{
		Spec: v1alpha1.CollectorDataSourceSpec{
			ClusterDefRef: clusterDefName,
			CollectorSpecs: []v1alpha1.CollectorSpec{{
				ComponentName: mysqlCompDefName,
				ScrapeConfigs: []v1alpha1.ScrapeConfig{{
					ContainerName: "mysql",
					ExternalLabels: map[string]string{
						"label1": "label1",
						"label2": "label2",
					},
					Metrics: &v1alpha1.MetricsCollector{
						CollectionInterval: "15s",
						MetricsSelector:    []string{"mysql_global_status_threads_running"},
						ExporterRef: v1alpha1.ExporterRef{
							ExporterNames: []string{metricsSinkName},
						},
					},
					Logs: &v1alpha1.LogsCollector{
						ExporterRef: v1alpha1.ExporterRef{
							ExporterNames: []string{logsSinkName},
						},
						LogTypes: []string{"error", "slow", "general"},
					},
				}},
			}},
		},
	}
	datasource.SetName("mysql-metric")
	datasource.SetNamespace(testCtx.DefaultNamespace)
	testapps.CreateK8sResource(&testCtx, &datasource)

	prometheus := v1alpha1.MetricsExporterSink{
		Spec: v1alpha1.MetricsExporterSinkSpec{
			Type: v1alpha1.PrometheusSinkType,
			MetricsSinkSource: v1alpha1.MetricsSinkSource{
				PrometheusConfig: &v1alpha1.PrometheusConfig{
					Namespace: testCtx.DefaultNamespace,
					ServiceRef: v1alpha1.ServiceRef{
						Endpoint: "${env:HOST_IP}:1234",
					},
				},
			},
		},
	}
	prometheus.SetName(metricsSinkName)
	prometheus.SetNamespace(testCtx.DefaultNamespace)
	testapps.CreateK8sResource(&testCtx, &prometheus)

	loki := v1alpha1.LogsExporterSink{
		Spec: v1alpha1.LogsExporterSinkSpec{
			Type: v1alpha1.LokiSinkType,
			SinkSource: v1alpha1.SinkSource{
				LokiConfig: &v1alpha1.LokiConfig{
					ServiceRef: v1alpha1.ServiceRef{
						Endpoint: "http://loki-gateway.kb-system/loki/api/v1/push",
					},
				},
			},
		},
	}
	loki.SetName("loki")
	loki.SetNamespace(testCtx.DefaultNamespace)
	testapps.CreateK8sResource(&testCtx, &loki)
	return oteld
}
