/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package builder

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
)

type MonitorServiceBuilder struct {
	BaseBuilder[monitoringv1.ServiceMonitor, *monitoringv1.ServiceMonitor, MonitorServiceBuilder]
}

func NewMonitorServiceBuilder(namespace, name string) *MonitorServiceBuilder {
	builder := &MonitorServiceBuilder{}
	builder.init(namespace, name, &monitoringv1.ServiceMonitor{}, builder)
	return builder
}

func (builder *MonitorServiceBuilder) SetMonitorServiceSpec(spec monitoringv1.ServiceMonitorSpec) *MonitorServiceBuilder {
	builder.get().Spec = spec
	return builder
}

func (builder *MonitorServiceBuilder) SetDefaultEndpoint(exporter *appsv1alpha1.Exporter) *MonitorServiceBuilder {
	if exporter == nil {
		return builder
	}

	if len(builder.get().Spec.Endpoints) != 0 {
		return builder
	}

	endpoint := monitoringv1.Endpoint{
		Port: exporter.ScrapePort,
		// TODO: deprecated: use `port` instead.
		// Compatible with previous versions of kb, the old addon supports int type port.
		TargetPort: exporter.TargetPort,
		Path:       common.FromScrapePath(*exporter),
		Scheme:     common.FromScheme(*exporter),
	}

	builder.get().Spec.Endpoints = []monitoringv1.Endpoint{endpoint}
	return builder
}
