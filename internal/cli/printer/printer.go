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

func (t *TablePrinter) SortBy(sortBy []table.SortBy) {
	t.Tbl.SortBy(sortBy)
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
