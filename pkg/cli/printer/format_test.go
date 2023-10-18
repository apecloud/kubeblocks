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
