package class

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func GetMinCPUAndMemory(model appsv1alpha1.ClassFamilyModel) (*resource.Quantity, *resource.Quantity) {
	var (
		minCPU    resource.Quantity
		minMemory resource.Quantity
	)

	if len(model.CPU.Slots) > 0 {
		minCPU = model.CPU.Slots[0]
	}

	if model.CPU.Min != nil && minCPU.Cmp(*model.CPU.Min) < 0 {
		minCPU = *model.CPU.Min
	}
	var memory *inf.Dec
	if model.Memory.MinPerCPU != nil {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), model.Memory.MinPerCPU.AsDec())
	} else {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), model.Memory.SizePerCPU.AsDec())
	}
	minMemory = resource.MustParse(memory.String())
	return &minCPU, &minMemory
}

type ClassModelWithFamilyName struct {
	Family string
	Model  appsv1alpha1.ClassFamilyModel
}

type ByModelList []ClassModelWithFamilyName

func (m ByModelList) Len() int {
	return len(m)
}

func (m ByModelList) Less(i, j int) bool {
	cpu1, mem1 := GetMinCPUAndMemory(m[i].Model)
	cpu2, mem2 := GetMinCPUAndMemory(m[j].Model)
	switch cpu1.Cmp(*cpu2) {
	case 1:
		return false
	case -1:
		return true
	}
	switch mem1.Cmp(*mem2) {
	case 1:
		return false
	case -1:
		return true
	}
	return false
}

func (m ByModelList) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type ComponentClass struct {
	Name    string            `json:"name,omitempty"`
	CPU     resource.Quantity `json:"cpu,omitempty"`
	Memory  resource.Quantity `json:"memory,omitempty"`
	Storage []*Disk           `json:"storage,omitempty"`
	Family  string            `json:"-"`
}

var _ sort.Interface = ByClassCPUAndMemory{}

type ByClassCPUAndMemory []*ComponentClass

func (b ByClassCPUAndMemory) Len() int {
	return len(b)
}

func (b ByClassCPUAndMemory) Less(i, j int) bool {
	if out := b[i].CPU.Cmp(b[j].CPU); out != 0 {
		return out < 0
	}

	if out := b[i].Memory.Cmp(b[j].Memory); out != 0 {
		return out < 0
	}

	return false
}

func (b ByClassCPUAndMemory) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type Filters map[string]resource.Quantity

func (f Filters) String() string {
	var result []string
	for k, v := range f {
		result = append(result, fmt.Sprintf("%s=%v", k, v.Value()))
	}
	return strings.Join(result, ",")
}

type Disk struct {
	Name  string            `json:"name,omitempty"`
	Size  resource.Quantity `json:"size,omitempty"`
	Class string            `json:"class,omitempty"`
}

func (d Disk) String() string {
	return fmt.Sprintf("%s=%s", d.Name, d.Size.String())
}

type ProviderComponentClassDef struct {
	Provider string   `json:"provider,omitempty"`
	Args     []string `json:"args,omitempty"`
}

type DiskDef struct {
	Name  string `json:"name,omitempty"`
	Size  string `json:"size,omitempty"`
	Class string `json:"class,omitempty"`
}

type ComponentClassDef struct {
	Name     string                      `json:"name,omitempty"`
	CPU      string                      `json:"cpu,omitempty"`
	Memory   string                      `json:"memory,omitempty"`
	Storage  []DiskDef                   `json:"storage,omitempty"`
	Args     []string                    `json:"args,omitempty"`
	Variants []ProviderComponentClassDef `json:"variants,omitempty"`
}

type ComponentClassSeriesDef struct {
	Name    string              `json:"name,omitempty"`
	Classes []ComponentClassDef `json:"classes,omitempty"`
}

type ComponentClassFamilyDef struct {
	Family   string                    `json:"family"`
	Template string                    `json:"template,omitempty"`
	Vars     []string                  `json:"vars,omitempty"`
	Series   []ComponentClassSeriesDef `json:"series,omitempty"`
}
