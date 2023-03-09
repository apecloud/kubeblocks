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

package controllerutil

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
)

func TestNerError(t *testing.T) {
	err1 := NewError(ErrorTypeBackupNotCompleted, "test c2")
	if err1.Error() != "test c2" {
		t.Error("NewErrorf failed")
	}
}

func TestNerErrorf(t *testing.T) {
	err1 := NewErrorf(ErrorTypeBackupNotCompleted, "test %s %s", "c1", "c2")
	if err1.Error() != "test c1 c2" {
		t.Error("NewErrorf failed")
	}
	testError := fmt.Errorf("test: %w", err1)
	if !errors.Is(testError, err1) {
		t.Error("errors.Is failed")
	}

	var target *Error
	if !errors.As(testError, &target) {
		t.Error("errors.As failed")
	}
}
