/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package controllerutil

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const FeatureGateEnableRuntimeMetrics = "ENABLED_RUNTIME_METRICS"

const (
	PrometheusScrapeAnnotationPath    = "monitor.kubeblocks.io/path"
	PrometheusScrapeAnnotationPort    = "monitor.kubeblocks.io/port"
	PrometheusScrapeAnnotationScheme  = "monitor.kubeblocks.io/scheme"
	PrometheusScrapeAnnotationEnabled = "monitor.kubeblocks.io/scrape"
)

const (
	defaultScrapePath   = "/metrics"
	defaultScrapeScheme = string(appsv1alpha1.HTTPProtocol)
)

func EnabledRuntimeMetrics() bool {
	return viper.GetBool(FeatureGateEnableRuntimeMetrics)
}

func GetScrapeAnnotations(scrapeConfig appsv1alpha1.PrometheusExporter, container *corev1.Container) map[string]string {
	return map[string]string{
		PrometheusScrapeAnnotationPath:   fromScrapePath(scrapeConfig),
		PrometheusScrapeAnnotationPort:   fromContainerPort(scrapeConfig, container),
		PrometheusScrapeAnnotationScheme: fromScheme(scrapeConfig),
		// Compatible with previous versions of kubeblocks.
		PrometheusScrapeAnnotationEnabled: "true",
	}
}

func fromScrapePath(config appsv1alpha1.PrometheusExporter) string {
	if config.ScrapePath != "" {
		return config.ScrapePath
	}
	return defaultScrapePath
}

func fromContainerPort(config appsv1alpha1.PrometheusExporter, container *corev1.Container) string {
	if config.ScrapePort != "" {
		return config.ScrapePort
	}
	if config.ScrapePort == "" && len(container.Ports) > 0 {
		return container.Ports[0].Name
	}
	return ""
}

func fromScheme(config appsv1alpha1.PrometheusExporter) string {
	if config.ScrapeScheme != "" {
		return string(config.ScrapeScheme)
	}
	return defaultScrapeScheme
}
