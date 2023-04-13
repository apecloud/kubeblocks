/*
Copyright ApeCloud, Inc.

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

package component

import (
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func buildMonitorConfig(
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	component *SynthesizedComponent) {
	monitorEnable := false
	if clusterCompSpec != nil {
		monitorEnable = clusterCompSpec.Monitor
	}

	monitorConfig := clusterCompDef.Monitor
	if !monitorEnable || monitorConfig == nil {
		disableMonitor(component)
		return
	}

	if !monitorConfig.BuiltIn {
		if monitorConfig.Exporter == nil {
			disableMonitor(component)
			return
		}
		component.Monitor = &MonitorConfig{
			Enable:     true,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort.IntVal,
		}

		if monitorConfig.Exporter.ScrapePort.Type == intstr.String {
			portName := monitorConfig.Exporter.ScrapePort.StrVal
			for _, c := range clusterCompDef.PodSpec.Containers {
				for _, p := range c.Ports {
					if p.Name == portName {
						component.Monitor.ScrapePort = p.ContainerPort
						break
					}
				}
			}
		}
		return
	}

	// TODO: builtin will support by an independent agent soon
	disableMonitor(component)
}

func disableMonitor(component *SynthesizedComponent) {
	component.Monitor = &MonitorConfig{
		Enable: false,
	}
}
