/*
Copyright © 2022 The OpenCli Authors

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

package provider

import (
	"helm.sh/helm/v3/pkg/repo"

	"github.com/infracreate/opencli/pkg/utils/helm"
)

type MysqlOperator struct{}

func (o *MysqlOperator) GetRepos() []repo.Entry {
	return []repo.Entry{
		{
			Name: "prometheus-community",
			URL:  "https://prometheus-community.github.io/helm-charts",
		},
		{
			Name: "mysql-operator",
			URL:  "https://mysql.github.io/mysql-operator/",
		},
	}
}

func (o *MysqlOperator) GetBaseCharts(ns string) []helm.InstallOpts {
	return []helm.InstallOpts{
		//{
		//	Name:      "prometheus",
		//	Chart:     "prometheus-community/kube-prometheus-stack",
		//	Wait:      true,
		//	Version:   "38.0.2",
		//	Namespace: ns,
		//	Sets: []string{
		//		"prometheusOperator.admissionWebhooks.patch.image.repository=weidixian/ingress-nginx-kube-webhook-certgen",
		//		"kube-state-metrics.image.repository=jiamiao442/kube-state-metrics",
		//		"kubeStateMetrics.enabled=false",
		//		"grafana.sidecar.dashboards.searchNamespace=ALL",
		//		"prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false",
		//		"prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false",
		//	},
		//},
	}
}

func (o *MysqlOperator) GetDBCharts(ns string, dbname string) []helm.InstallOpts {
	return []helm.InstallOpts{
		//{
		//	Name:      "mysql-operator",
		//	Chart:     "mysql-operator/mysql-operator",
		//	Wait:      true,
		//	Version:   "2.0.5",
		//	Namespace: "mysql-operator",
		//	Sets:      []string{},
		//},
		{
			Name:      dbname,
			Chart:     "mysql-operator/mysql-innodbcluster",
			Wait:      false,
			Namespace: ns,
			Version:   "2.0.5",
			Sets: []string{
				"credentials.root.user='root'",
				"credentials.root.password=sakila",
				"credentials.root.host='%'",
				"serverInstances=1",
				"routerInstances=1",
				"tls.useSelfSigned=true",
				"serverVersion=8.0.29",
			},
		},
	}
}
