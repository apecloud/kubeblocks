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

	"github.com/gosuri/uitable"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Printer interface {
	AddHeader()
	AddRow(objs *ClusterObjects)
	Print(out io.Writer) error
}

// ClusterPrinter prints cluster info
type ClusterPrinter struct {
	Tbl *uitable.Table
}

var _ Printer = &ClusterPrinter{}

func (p *ClusterPrinter) AddHeader() {
	p.Tbl.AddRow("NAME", "NAMESPACE", "APP-VERSION", "CLUSTER-DEFINITION", "TERMINATION-POLICY", "STATUS", "INTERNAL-ENDPOINTS", "EXTERNAL-ENDPOINTS", "AGE")
}

func (p *ClusterPrinter) AddRow(objs *ClusterObjects) {
	c := objs.GetClusterInfo()
	p.Tbl.AddRow(c.Name, c.Namespace, c.AppVersion, c.ClusterDefinition, c.TerminationPolicy, c.Status, c.InternalEP, c.ExternalEP, c.Age)
}

func (p *ClusterPrinter) Print(out io.Writer) error {
	return util.PrintTable(out, p.Tbl)
}

// ComponentPrinter prints component info
type ComponentPrinter struct {
	Tbl *uitable.Table
}

var _ Printer = &ComponentPrinter{}

func (p *ComponentPrinter) AddHeader() {
	p.Tbl.AddRow("NAME", "CLUSTER", "TYPE", "REPLICAS(DESIRED/TOTAL)", "IMAGE")
}

func (p *ComponentPrinter) AddRow(objs *ClusterObjects) {
	components := objs.GetComponentInfo()
	for _, c := range components {
		p.Tbl.AddRow(c.Name, c.Cluster, c.Type, c.Replicas, c.Image)
	}
}

func (p *ComponentPrinter) Print(out io.Writer) error {
	return util.PrintTable(out, p.Tbl)
}

// InstancePrinter prints instance info
type InstancePrinter struct {
	Tbl *uitable.Table
}

var _ Printer = &InstancePrinter{}

func (p *InstancePrinter) AddHeader() {
	p.Tbl.AddRow("NAME", "CLUSTER", "COMPONENT", "STATUS", "ROLE", "ACCESSMODE", "AZ", "REGION", "CPU(REQUEST/LIMIT)", "MEMORY(REQUEST/LIMIT)", "STORAGE", "NODE", "AGE")
}

func (p *InstancePrinter) AddRow(objs *ClusterObjects) {
	instances := objs.GetInstanceInfo()
	for _, instance := range instances {
		p.Tbl.AddRow(instance.Name, instance.Cluster, instance.Component,
			instance.Status, instance.Role, instance.AccessMode,
			instance.AZ, instance.Region, instance.CPU, instance.Memory,
			instance.Storage, instance.Node, instance.Age)
	}
}

func (p *InstancePrinter) Print(out io.Writer) error {
	return util.PrintTable(out, p.Tbl)
}
