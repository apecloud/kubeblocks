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

package cluster

import (
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type PrintType string

const (
	PrintClusters   PrintType = "clusters"
	PrintWide       PrintType = "wide"
	PrintInstances  PrintType = "instances"
	PrintComponents PrintType = "components"
	PrintEvents     PrintType = "events"
	PrintLabels     PrintType = "label"
)

type PrinterOptions struct {
	ShowLabels bool
}

type tblInfo struct {
	header     []interface{}
	addRow     func(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions)
	getOptions GetOptions
}

var mapTblInfo = map[PrintType]tblInfo{
	PrintClusters: {
		header: []interface{}{"NAME", "NAMESPACE", "CLUSTER-DEFINITION", "VERSION", "TERMINATION-POLICY", "STATUS", "CREATED-TIME"},
		addRow: func(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
			c := objs.GetClusterInfo()
			info := []interface{}{c.Name, c.Namespace, c.ClusterDefinition, c.ClusterVersion, c.TerminationPolicy, c.Status, c.CreatedTime}
			if opt.ShowLabels {
				info = append(info, c.Labels)
			}

			tbl.AddRow(info...)
		},
		getOptions: GetOptions{},
	},
	PrintWide: {
		header: []interface{}{"NAME", "NAMESPACE", "CLUSTER-DEFINITION", "VERSION", "TERMINATION-POLICY", "STATUS", "INTERNAL-ENDPOINTS", "EXTERNAL-ENDPOINTS", "CREATED-TIME"},
		addRow: func(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
			c := objs.GetClusterInfo()
			info := []interface{}{c.Name, c.Namespace, c.ClusterDefinition, c.ClusterVersion, c.TerminationPolicy, c.Status, c.InternalEP, c.ExternalEP, c.CreatedTime}
			if opt.ShowLabels {
				info = append(info, c.Labels)
			}
			tbl.AddRow(info...)
		},
		getOptions: GetOptions{WithClusterDef: true, WithService: true, WithPod: true},
	},
	PrintInstances: {
		header:     []interface{}{"NAME", "NAMESPACE", "CLUSTER", "COMPONENT", "STATUS", "ROLE", "ACCESSMODE", "AZ", "CPU(REQUEST/LIMIT)", "MEMORY(REQUEST/LIMIT)", "STORAGE", "NODE", "CREATED-TIME"},
		addRow:     AddInstanceRow,
		getOptions: GetOptions{WithClusterDef: true, WithPod: true},
	},
	PrintComponents: {
		header:     []interface{}{"NAME", "NAMESPACE", "CLUSTER", "TYPE", "IMAGE"},
		addRow:     AddComponentRow,
		getOptions: GetOptions{WithClusterDef: true, WithPod: true},
	},
	PrintEvents: {
		header:     []interface{}{"NAMESPACE", "TIME", "TYPE", "REASON", "OBJECT", "MESSAGE"},
		addRow:     AddEventRow,
		getOptions: GetOptions{WithClusterDef: true, WithPod: true, WithEvent: true},
	},
	PrintLabels: {
		header:     []interface{}{"NAME", "NAMESPACE"},
		addRow:     AddLabelRow,
		getOptions: GetOptions{},
	},
}

// Printer prints cluster info
type Printer struct {
	tbl *printer.TablePrinter
	opt *PrinterOptions
	tblInfo
}

func NewPrinter(out io.Writer, printType PrintType, opt *PrinterOptions) *Printer {
	p := &Printer{tbl: printer.NewTablePrinter(out)}
	p.tblInfo = mapTblInfo[printType]

	if opt == nil {
		opt = &PrinterOptions{}
	}
	p.opt = opt

	if opt.ShowLabels {
		p.tblInfo.header = append(p.tblInfo.header, "LABELS")
	}

	p.tbl.SetHeader(p.tblInfo.header...)
	return p
}

func (p *Printer) AddRow(objs *ClusterObjects) {
	p.addRow(p.tbl, objs, p.opt)
}

func (p *Printer) Print() {
	p.tbl.Print()
}

func (p *Printer) GetterOptions() GetOptions {
	return p.getOptions
}

func AddLabelRow(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
	c := objs.GetClusterInfo()
	info := []interface{}{c.Name, c.Namespace}
	if opt.ShowLabels {
		labels := strings.ReplaceAll(c.Labels, ",", "\n")
		info = append(info, labels)
	}
	tbl.AddRow(info...)
}

func AddComponentRow(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
	components := objs.GetComponentInfo()
	for _, c := range components {
		tbl.AddRow(c.Name, c.NameSpace, c.Cluster, c.Type, c.Image)
	}
}

func AddInstanceRow(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
	instances := objs.GetInstanceInfo()
	for _, instance := range instances {
		tbl.AddRow(instance.Name, instance.Namespace, instance.Cluster, instance.Component,
			instance.Status, instance.Role, instance.AccessMode,
			instance.AZ, instance.CPU, instance.Memory,
			BuildStorageSize(instance.Storage), instance.Node, instance.CreatedTime)
	}
}

func AddEventRow(tbl *printer.TablePrinter, objs *ClusterObjects, opt *PrinterOptions) {
	events := util.SortEventsByLastTimestamp(objs.Events, "")
	for _, event := range *events {
		e := event.(*corev1.Event)
		tbl.AddRow(e.Namespace, util.GetEventTimeStr(e), e.Type, e.Reason, util.GetEventObject(e), e.Message)
	}
}
