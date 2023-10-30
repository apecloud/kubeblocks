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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var OTeldSignature = func(_ v1alpha1.OTeld, _ *v1alpha1.OTeld, _ v1alpha1.OTeldList, _ *v1alpha1.OTeldList) {}

const (
	logsSinkName    = "loki"
	metricsSinkName = "prometheus"

	mysqlCompDefName = "replicasets"
	mysqlCompName    = "mysql"
)

var _ = Describe("Oteld Monitor Controller", func() {

	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster"
	)

	Context("OTeld Controller", func() {

		var cluster *appsv1alpha1.Cluster

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
		})
	})
})

func mockOTeldInstance() *v1alpha1.OTeld {

	oteld := &v1alpha1.OTeld{
		Spec: v1alpha1.OTeldSpec{
			Image: "docker.io/apecloud/oteld:0.1.0-beta.1",
			Batch: v1alpha1.BatchConfig{
				Enabled: true,
			},
			CollectionInterval: "15s",
			LogsLevel:          "debug",
			MetricsPort:        8888,
			UseConfigMap:       true,
			SystemDataSource: &v1alpha1.SystemDataSource{
				EnabledPodLogs:              true,
				EnabledK8sNodeStatesMetrics: true,
				EnabledK8sClusterExporter:   false,
				EnabledNodeExporter:         true,
				MetricsExporterRef:          "prometheus",
				LogsExporterRef:             "loki",
				CollectionInterval:          15 * time.Second,
			},
		},
	}
	oteld.SetName("oteld-test")
	oteld.SetNamespace(testCtx.DefaultNamespace)
	oteld = testapps.CreateK8sResource(&testCtx, oteld).(*v1alpha1.OTeld)

	testapps.CreateCustomizedObj(&testCtx, "monitor/collectordatasource.yaml", &v1alpha1.CollectorDataSource{},
		testCtx.UseDefaultNamespace(),
		testapps.WithName(metricsSinkName))

	prometheus := v1alpha1.MetricsExporterSink{
		Spec: v1alpha1.MetricsExporterSinkSpec{
			Type: v1alpha1.PrometheusSinkType,
			MetricsSinkSource: v1alpha1.MetricsSinkSource{
				PrometheusConfig: &v1alpha1.PrometheusConfig{
					Namespace: testCtx.DefaultNamespace,
					ServiceRef: v1alpha1.ServiceRef{
						Endpoint:                    "${env:HOST_IP}:1234",
						Namespace:                   "default",
						ServiceConnectionCredential: "prometheus-service",
					},
				},
			},
		},
	}
	prometheus.SetName("prometheus")
	prometheus.SetNamespace(testCtx.DefaultNamespace)
	testapps.CreateK8sResource(&testCtx, &prometheus)

	loki := v1alpha1.LogsExporterSink{
		Spec: v1alpha1.LogsExporterSinkSpec{
			Type: v1alpha1.LokiSinkType,
			SinkSource: v1alpha1.SinkSource{
				LokiConfig: &v1alpha1.LokiConfig{
					ServiceRef: v1alpha1.ServiceRef{
						Endpoint:                    "http://loki-gateway.kb-system/loki/api/v1/push",
						Namespace:                   "default",
						ServiceConnectionCredential: "loki-service",
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
