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
