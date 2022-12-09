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

package printer

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	// StyleKubeCtl renders a Table like kubectl
	StyleKubeCtl = table.Style{
		Name:    "StyleKubeCtl",
		Box:     table.StyleBoxDefault,
		Color:   table.ColorOptionsDefault,
		Format:  table.FormatOptionsDefault,
		HTML:    table.DefaultHTMLOptions,
		Options: table.OptionsNoBordersAndSeparators,
		Title:   table.TitleOptionsDefault,
	}
)

type TablePrinter struct {
	tbl table.Writer
}

func NewTablePrinter(out io.Writer) *TablePrinter {
	t := table.NewWriter()
	t.SetStyle(StyleKubeCtl)
	t.SetOutputMirror(out)
	return &TablePrinter{tbl: t}
}

func (t *TablePrinter) SetHeader(header ...interface{}) {
	t.tbl.AppendHeader(header)
}

func (t *TablePrinter) AddRow(row ...interface{}) {
	rowObj := table.Row{}
	for _, col := range row {
		rowObj = append(rowObj, col)
	}
	t.tbl.AppendRow(rowObj)
}

func (t *TablePrinter) Print() {
	t.tbl.Render()
}
