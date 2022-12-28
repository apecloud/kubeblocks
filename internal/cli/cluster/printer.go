/*
Copyright ApeCloud Inc.

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

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

type Printer interface {
	AddRow(objs *ClusterObjects)
	Print()
}

// ClusterPrinter prints cluster info
type ClusterPrinter struct {
	tbl *printer.TablePrinter
}

var _ Printer = &ClusterPrinter{}

func NewClusterPrinter(out io.Writer) *ClusterPrinter {
	p := &ClusterPrinter{tbl: printer.NewTablePrinter(out)}
	p.tbl.SetHeader("NAME", "NAMESPACE", "VERSION", "CLUSTER-DEFINITION", "TERMINATION-POLICY", "STATUS", "INTERNAL-ENDPOINTS", "EXTERNAL-ENDPOINTS", "AGE")
	return p
}

func (p *ClusterPrinter) AddRow(objs *ClusterObjects) {
	c := objs.GetClusterInfo()
	p.tbl.AddRow(c.Name, c.Namespace, c.ClusterVersion, c.ClusterDefinition, c.TerminationPolicy, c.Status, c.InternalEP, c.ExternalEP, c.Age)
}

func (p *ClusterPrinter) Print() {
	p.tbl.Print()
}

// ComponentPrinter prints component info
type ComponentPrinter struct {
	tbl *printer.TablePrinter
}

var _ Printer = &ComponentPrinter{}

func NewComponentPrinter(out io.Writer) *ComponentPrinter {
	p := &ComponentPrinter{tbl: printer.NewTablePrinter(out)}
	p.tbl.SetHeader("NAME", "CLUSTER", "TYPE", "REPLICAS(DESIRED/TOTAL)", "IMAGE")
	return p
}

func (p *ComponentPrinter) AddRow(objs *ClusterObjects) {
	components := objs.GetComponentInfo()
	for _, c := range components {
		p.tbl.AddRow(c.Name, c.Cluster, c.Type, c.Replicas, c.Image)
	}
}

func (p *ComponentPrinter) Print() {
	p.tbl.Print()
}

// InstancePrinter prints instance info
type InstancePrinter struct {
	tbl *printer.TablePrinter
}

var _ Printer = &InstancePrinter{}

func NewInstancePrinter(out io.Writer) *InstancePrinter {
	p := &InstancePrinter{tbl: printer.NewTablePrinter(out)}
	p.tbl.SetHeader("NAME", "CLUSTER", "COMPONENT", "STATUS", "ROLE", "ACCESSMODE", "AZ", "REGION", "CPU(REQUEST/LIMIT)", "MEMORY(REQUEST/LIMIT)", "STORAGE", "NODE", "AGE")
	return p
}

func (p *InstancePrinter) AddRow(objs *ClusterObjects) {
	instances := objs.GetInstanceInfo()
	for _, instance := range instances {
		p.tbl.AddRow(instance.Name, instance.Cluster, instance.Component,
			instance.Status, instance.Role, instance.AccessMode,
			instance.AZ, instance.Region, instance.CPU, instance.Memory,
			instance.Storage, instance.Node, instance.Age)
	}
}

func (p *InstancePrinter) Print() {
	p.tbl.Print()
}
