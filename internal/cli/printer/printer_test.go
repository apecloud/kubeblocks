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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
)

var (
	header = []string{"NAME", "NAMESPACE", "CLUSTER-DEFINITION", "VERSION", "TERMINATION-POLICY", "CREATED-TIME"}
)

func TestPrintTable(t *testing.T) {
	printer := NewTablePrinter(os.Stdout)
	headerRow := make([]interface{}, len(header))
	for i, h := range header {
		headerRow[i] = h
	}
	printer.SetHeader(headerRow...)
	for _, r := range [][]string{
		{"brier63", "default", "apecloud-mysql", "ac-mysql-8.0.30", "Delete", "Feb 20,2023 16:39 UTC+0800"},
		{"cedar51", "default", "apecloud-mysql", "ac-mysql-8.0.30", "Delete", "Feb 20,2023 16:39 UTC+0800"},
	} {
		row := make([]interface{}, len(r))
		for i, rr := range r {
			row[i] = rr
		}
		printer.AddRow(row...)
	}
	printer.Print()
}

func TestPrintPairStringToLine(t *testing.T) {
	doPrintPairStringToLineAssert(nil, t)
	spaceCount := 0
	doPrintPairStringToLineAssert(&spaceCount, t)
	spaceCount = 3
	doPrintPairStringToLineAssert(&spaceCount, t)
}

func doPrintPairStringToLineAssert(spaceCount *int, t *testing.T) {
	done := clitesting.Capture()
	key, value := "key", "value"
	var expectSpaceCount int
	if spaceCount == nil {
		PrintPairStringToLine(key, value)
		expectSpaceCount = 2
	} else {
		PrintPairStringToLine(key, value, *spaceCount)
		expectSpaceCount = *spaceCount
	}

	capturedOutput, err := done()
	if err != nil {
		t.Error("capture stdout failed:" + err.Error())
	}
	var spaceStr string
	for i := 0; i < expectSpaceCount; i++ {
		spaceStr += " "
	}
	assert.Equal(t, fmt.Sprintf("%s%-20s%s", spaceStr, key+":", value+"\n"), capturedOutput)
}

func TestPrintLineWithTabSeparator(t *testing.T) {
	done := clitesting.Capture()
	key, value := "key", "value"
	PrintLineWithTabSeparator(NewPair(key, value))
	checkOutPut(t, done, fmt.Sprintf("%s: %s\t\n", key, value))
}

func TestPrintTitle(t *testing.T) {
	done := clitesting.Capture()
	line := "Title"
	PrintTitle(line)
	checkOutPut(t, done, fmt.Sprintf("\n%s:\n", line))
}

func TestPrintLine(t *testing.T) {
	done := clitesting.Capture()
	line := "test line"
	PrintLine(line)
	checkOutPut(t, done, "test line\n")
}

func checkOutPut(t *testing.T, captureFunc func() (string, error), expect string) {
	capturedOutput, err := captureFunc()
	if err != nil {
		t.Error("capture stdout failed:" + err.Error())
	}
	assert.Equal(t, expect, capturedOutput)
}
