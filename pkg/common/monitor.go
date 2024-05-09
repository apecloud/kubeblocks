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

package common

import (
	"k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

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

func FromScrapePath(config appsv1alpha1.Exporter) string {
	if config.ScrapePath != "" {
		return config.ScrapePath
	}
	return defaultScrapePath
}

func FromContainerPort(config appsv1alpha1.Exporter, container *v1.Container) string {
	if config.ScrapePort != "" {
		return config.ScrapePort
	}
	if container != nil && len(container.Ports) > 0 {
		return container.Ports[0].Name
	}
	if config.TargetPort != nil {
		return config.TargetPort.String()
	}
	return ""
}

func FromScheme(config appsv1alpha1.Exporter) string {
	if config.ScrapeScheme != "" {
		return string(config.ScrapeScheme)
	}
	return defaultScrapeScheme
}

func GetScrapeAnnotations(scrapeConfig appsv1alpha1.Exporter, container *v1.Container) map[string]string {
	return map[string]string{
		PrometheusScrapeAnnotationPath:   FromScrapePath(scrapeConfig),
		PrometheusScrapeAnnotationPort:   FromContainerPort(scrapeConfig, container),
		PrometheusScrapeAnnotationScheme: FromScheme(scrapeConfig),
		// Compatible with previous versions of kubeblocks.
		PrometheusScrapeAnnotationEnabled: "true",
	}
}
