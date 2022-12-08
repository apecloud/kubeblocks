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
	"os"
	"testing"
)

var (
	colTitleIndex     = "#"
	colTitleFirstName = "First Name"
	colTitleLastName  = "Last Name"
	colTitleSalary    = "Salary"
)

func TestPrintTable(t *testing.T) {
	printer := NewTablePrinter(os.Stdout)
	printer.SetHeader(colTitleIndex, colTitleFirstName, colTitleLastName, colTitleSalary)
	for _, r := range [][]string{
		{"1", "Arya", "Stark", "3000"},
		{"20", "Jon", "Snow", "2000"},
	} {
		row := make([]interface{}, len(r))
		for i, rr := range r {
			row[i] = rr
		}
		printer.AddRow(row...)
	}
	printer.Print()
}
