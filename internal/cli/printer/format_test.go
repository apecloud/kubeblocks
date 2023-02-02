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
	"testing"

	"github.com/spf13/cobra"
)

func TestFormat(t *testing.T) {
	var format Format
	cmd := &cobra.Command{}
	AddOutputFlag(cmd, &format)
	flag := cmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("expect output flag")
	}

	v := flag.Value
	if v.String() != Table.String() {
		t.Errorf("expect table format")
	}

	err := v.Set("json")
	if err != nil {
		t.Errorf("failed to set format")
	}

	testParse := func(formatStr string) bool {
		f, _ := ParseFormat(formatStr)
		return f.String() == formatStr
	}

	for _, f := range Formats() {
		if !testParse(f) {
			t.Errorf("expect %s format", f)
		}
	}
}
