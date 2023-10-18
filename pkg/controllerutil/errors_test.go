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

package controllerutil

import (
	"testing"

	"github.com/pkg/errors"
)

func TestIsTargetError(t *testing.T) {
	var err1 error
	if IsTargetError(err1, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a false return, but got false")
	}
	err1 = NewError(ErrorWaitCacheRefresh, "test c2")
	if !IsTargetError(err1, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a true return, but got false")
	}
	err2 := errors.Wrap(err1, "test c1")
	if !IsTargetError(err2, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a true return, but got false")
	}
}
