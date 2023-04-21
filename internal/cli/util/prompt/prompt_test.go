/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package prompt

import (
	"bytes"
	"io"
	"testing"
)

func Test(t *testing.T) {
	c := NewPrompt("Please input something", nil, &bytes.Buffer{})
	res, _ := c.Run()
	if res != "" {
		t.Errorf("expected an empty result")
	}

	in := &bytes.Buffer{}
	in.Write([]byte("t\n"))
	c.Stdin = io.NopCloser(in)
	res, err := c.Run()
	if err != nil {
		t.Errorf("prompt error %v", err)
	}
	if res != "t" {
		t.Errorf("prompt result is not expected")
	}
}
