/*
Copyright © 2022 The dbctl Authors

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

	"jihulab.com/infracreate/dbaas-system/dbctl/pkg/utils/helm"
)

type BitnamiMysql struct {
	serverVersion string
}

func (o *BitnamiMysql) GetRepos() []repo.Entry {
	return []repo.Entry{
		{
			Name: "prometheus-community",
			URL:  "https://prometheus-community.github.io/helm-charts",
		},
	}
}

func (o *BitnamiMysql) GetBaseCharts(ns string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      "prometheus",
			Chart:     "prometheus-community/kube-prometheus-stack",
			Wait:      true,
			Version:   "38.0.2",
			Namespace: ns,
			Sets: []string{
				"prometheusOperator.admissionWebhooks.patch.image.repository=weidixian/ingress-nginx-kube-webhook-certgen",
				"kube-state-metrics.image.repository=jiamiao442/kube-state-metrics",
				"kubeStateMetrics.enabled=false",
				"grafana.sidecar.dashboards.searchNamespace=ALL",
				"prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false",
				"prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false",
			},
		},
	}
}

func (o *BitnamiMysql) GetDBCharts(ns string, dbname string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      dbname,
			Chart:     "bitnami/mysql",
			Wait:      true,
			Namespace: ns,
			Version:   o.serverVersion,
			Sets: []string{
				"metrics.enabled=true",
				"metrics.serviceMonitor.enabled=true",
			},
		},
	}
}
