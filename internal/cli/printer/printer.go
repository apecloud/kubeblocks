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

package printer

import (
	"fmt"
	"io"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	// KubeCtlStyle renders a Table like kubectl
	KubeCtlStyle table.Style

	// TerminalStyle renders a Table like below:
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   # | FIRST NAME | LAST NAME | SALARY |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |   1 | Arya       | Stark     |   3000 |                             |
	//  |  20 | Jon        | Snow      |   2000 | You know nothing, Jon Snow! |
	//  | 300 | Tyrion     | Lannister |   5000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	//  |     |            | TOTAL     |  10000 |                             |
	//  +-----+------------+-----------+--------+-----------------------------+
	TerminalStyle = table.Style{
		Name:    "TerminalStyle",
		Box:     table.StyleBoxDefault,
		Color:   table.ColorOptionsDefault,
		Format:  table.FormatOptionsDefault,
		HTML:    table.DefaultHTMLOptions,
		Options: table.OptionsDefault,
		Title:   table.TitleOptionsDefault,
	}
)

type TablePrinter struct {
	Tbl table.Writer
}

func init() {
	boxStyle := table.StyleBoxDefault
	boxStyle.PaddingLeft = ""
	boxStyle.PaddingRight = "   "
	KubeCtlStyle = table.Style{
		Name:    "StyleKubeCtl",
		Box:     boxStyle,
		Color:   table.ColorOptionsDefault,
		Format:  table.FormatOptionsDefault,
		HTML:    table.DefaultHTMLOptions,
		Options: table.OptionsNoBordersAndSeparators,
		Title:   table.TitleOptionsDefault,
	}
}

// PrintTable high level wrapper function.
func PrintTable(out io.Writer, customSettings func(*TablePrinter), rowFeeder func(*TablePrinter) error, header ...interface{}) error {
	t := NewTablePrinter(out)
	t.SetHeader(header...)
	if customSettings != nil {
		customSettings(t)
	}
	if rowFeeder != nil {
		if err := rowFeeder(t); err != nil {
			return err
		}
	}
	t.Print()
	return nil
}

func NewTablePrinter(out io.Writer) *TablePrinter {
	t := table.NewWriter()
	t.SetStyle(KubeCtlStyle)
	t.SetOutputMirror(out)
	return &TablePrinter{Tbl: t}
}

func (t *TablePrinter) SetStyle(style table.Style) {
	t.Tbl.SetStyle(style)
}

func (t *TablePrinter) SetHeader(header ...interface{}) {
	t.Tbl.AppendHeader(header)
}

func (t *TablePrinter) AddRow(row ...interface{}) {
	rowObj := table.Row{}
	for _, col := range row {
		rowObj = append(rowObj, col)
	}
	t.Tbl.AppendRow(rowObj)
}

func (t *TablePrinter) Print() {
	t.Tbl.Render()
}

// PrintPairStringToLine print pair string for a line , the format is as follows "<space>*<key>:\t<value>".
// spaceCount is the space character count which is placed in the offset of field string.
// the default values of tabCount is 2.
func PrintPairStringToLine(name, value string, spaceCount ...int) {
	scn := 2
	// only the first variable of tabCount is effective.
	if len(spaceCount) > 0 {
		scn = spaceCount[0]
	}
	var (
		spaceString string
		i           int
	)
	for i = 0; i < scn; i++ {
		spaceString += " "
	}
	line := fmt.Sprintf("%s%-20s%s", spaceString, name+":", value)
	fmt.Println(line)
}

type Pair string

func NewPair(key, value string) Pair {
	return Pair(fmt.Sprintf("%s: %s", key, value))
}

func PrintLineWithTabSeparator(ps ...Pair) {
	var line string
	for _, v := range ps {
		line += string(v) + "\t"
	}
	fmt.Println(line)
}

func PrintTitle(title string) {
	titleTpl := fmt.Sprintf("\n%s:", title)
	fmt.Println(titleTpl)
}

func PrintLine(line string) {
	fmt.Println(line)
}

// PrintBlankLine print a blank line
func PrintBlankLine(out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintln(out)
}
