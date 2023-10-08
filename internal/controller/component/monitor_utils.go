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

package component

import (
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func buildMonitorConfigLegacy(clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec,
	synthesizeComp *SynthesizedComponent) error {
	var (
		compDef *appsv1alpha1.ComponentDefinition
		comp    *appsv1alpha1.Component
		err     error
	)
	if compDef, err = BuildComponentDefinitionFrom(clusterCompDef, clusterCompVer, synthesizeComp.ClusterName); err != nil {
		return err
	}
	if comp, err = BuildComponentFrom(clusterCompDef, clusterCompVer, clusterCompSpec); err != nil {
		return err
	}
	buildMonitorConfig(compDef, comp, synthesizeComp)
	return nil
}

func buildMonitorConfig(compDef *appsv1alpha1.ComponentDefinition,
	comp *appsv1alpha1.Component,
	synthesizeComp *SynthesizedComponent) {
	monitorEnable := false
	if comp != nil {
		monitorEnable = comp.Spec.Monitor
	}

	monitorConfig := compDef.Spec.Monitor
	if !monitorEnable || monitorConfig == nil {
		disableMonitor(synthesizeComp)
		return
	}

	if !monitorConfig.BuiltIn {
		if monitorConfig.Exporter == nil {
			disableMonitor(synthesizeComp)
			return
		}
		synthesizeComp.Monitor = &MonitorConfig{
			Enable:     true,
			BuiltIn:    false,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort.IntVal,
		}

		if monitorConfig.Exporter.ScrapePort.Type == intstr.String {
			portName := monitorConfig.Exporter.ScrapePort.StrVal
			for _, c := range compDef.Spec.Runtime.Containers {
				for _, p := range c.Ports {
					if p.Name == portName {
						synthesizeComp.Monitor.ScrapePort = p.ContainerPort
						break
					}
				}
			}
		}
		return
	}

	synthesizeComp.Monitor = &MonitorConfig{
		Enable:  true,
		BuiltIn: true,
	}
}

func disableMonitor(component *SynthesizedComponent) {
	component.Monitor = &MonitorConfig{
		Enable:  false,
		BuiltIn: false,
	}
}
