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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

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

func FromScrapePath(exporter appsv1alpha1.Exporter) string {
	if exporter.ScrapePath != "" {
		return exporter.ScrapePath
	}
	return defaultScrapePath
}

func getPortNumberFromContainer(portName string, container *corev1.Container) string {
	if container == nil || portName == "" {
		return portName
	}
	for _, port := range container.Ports {
		if port.Name == portName {
			return strconv.Itoa(int(port.ContainerPort))
		}
	}
	// Compatible with number ports.
	return portName
}

func FromContainerPort(exporter Exporter, container *corev1.Container) string {
	convertPort := func(port intstr.IntOrString) string {
		switch {
		case port.StrVal == "":
			return getPortNumberFromContainer(exporter.TargetPort.StrVal, container)
		case port.IntVal != 0:
			return strconv.Itoa(int(exporter.TargetPort.IntVal))
		default:
			return ""
		}
	}

	if exporter.ScrapePort != "" {
		return getPortNumberFromContainer(exporter.ScrapePort, container)
	}
	if exporter.TargetPort != nil {
		return convertPort(*exporter.TargetPort)
	}
	if container != nil && len(container.Ports) > 0 {
		return strconv.Itoa(int(container.Ports[0].ContainerPort))
	}
	return ""
}

func FromScheme(exporter appsv1alpha1.Exporter) string {
	if exporter.ScrapeScheme != "" {
		return string(exporter.ScrapeScheme)
	}
	return defaultScrapeScheme
}

func GetScrapeAnnotations(exporter Exporter, container *corev1.Container) map[string]string {
	return map[string]string{
		PrometheusScrapeAnnotationPath:   FromScrapePath(exporter.Exporter),
		PrometheusScrapeAnnotationPort:   FromContainerPort(exporter, container),
		PrometheusScrapeAnnotationScheme: FromScheme(exporter.Exporter),
		// Compatible with previous versions of kubeblocks.
		PrometheusScrapeAnnotationEnabled: "true",
	}
}
