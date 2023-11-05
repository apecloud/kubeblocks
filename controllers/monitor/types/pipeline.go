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

import (
	"context"
	"fmt"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Receiver struct {
	Parameter          string
	CollectionInterval string
}

type Exporters struct {
	MetricsExporter []v1alpha1.MetricsExporterSink
	LogsExporter    []v1alpha1.LogsExporterSink
}

type Pipeline struct {
	Name         string
	ReceiverType string
	ReceiverMap  map[string]Receiver
	ProcessorMap map[string]bool
	ExporterMap  map[string]bool
}

type OteldInstance struct {
	MetricsPipeline    []Pipeline
	LogPipeline        []Pipeline
	AppLogsPipeline    *Pipeline
	AppMetricsPipelien *Pipeline
	OTeld              *v1alpha1.OTeld
	AppDataSources     []v1alpha1.CollectorDataSource

	Cli client.Client
	Ctx context.Context
}

func NewPipeline(name string) Pipeline {
	return Pipeline{
		Name:         name,
		ReceiverType: ReceiverCreatorType,
		ReceiverMap:  make(map[string]Receiver),
		ProcessorMap: make(map[string]bool),
		ExporterMap:  make(map[string]bool),
	}
}

func (p *Pipeline) GetReceiverName() string {
	return fmt.Sprintf("%s/%s", p.ReceiverType, p.Name)
}

func NewOteldInstance(oteld *v1alpha1.OTeld, cli client.Client, ctx context.Context) *OteldInstance {
	return &OteldInstance{
		Cli:                cli,
		Ctx:                ctx,
		OTeld:              oteld,
		MetricsPipeline:    []Pipeline{},
		LogPipeline:        []Pipeline{},
		AppLogsPipeline:    &Pipeline{},
		AppMetricsPipelien: &Pipeline{},
		AppDataSources:     []v1alpha1.CollectorDataSource{},
	}
}
