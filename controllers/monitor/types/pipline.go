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

package types

import "github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"

type Receiver struct {
	Parameter          string
	CollectionInterval string
}

type Exporters struct {
	Metricsexporter []v1alpha1.MetricsExporterSink
	Logsexporter    []v1alpha1.LogsExporterSink
}

type Pipline struct {
	Name         string
	ReceiverMap  map[string]Receiver
	ProcessorMap map[string]bool
	ExporterMap  map[string]bool
}

type OteldInstance struct {
	MetricsPipline []Pipline
	LogsPipline    []Pipline
	OteldTemplate  *v1alpha1.OTeldCollectorTemplate
}

func NewPipline() Pipline {
	return Pipline{
		ReceiverMap:  make(map[string]Receiver),
		ProcessorMap: make(map[string]bool),
		ExporterMap:  make(map[string]bool),
	}
}

func NewOteldInstance() *OteldInstance {
	return &OteldInstance{
		MetricsPipline: []Pipline{},
		LogsPipline:    []Pipline{},
	}
}
