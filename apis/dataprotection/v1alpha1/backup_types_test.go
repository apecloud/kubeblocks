/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"testing"
	"time"
)

func expectToDuration(t *testing.T, ttl string, baseNum, targetNum int) {
	d := ToDuration(&ttl)
	if d != time.Hour*time.Duration(baseNum)*time.Duration(targetNum) {
		t.Errorf(`Expected duration is "%d*%d*time.Hour"", got %v`, targetNum, baseNum, d)
	}
}

func TestToDuration(t *testing.T) {
	d := ToDuration(nil)
	if d != time.Duration(0) {
		t.Errorf("Expected duration is 0, got %v", d)
	}
	expectToDuration(t, "7d", 24, 7)
	expectToDuration(t, "7D", 24, 7)
	expectToDuration(t, "12h", 1, 12)
	expectToDuration(t, "12H", 1, 12)
}
