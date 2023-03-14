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
