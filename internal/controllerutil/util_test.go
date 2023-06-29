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
	"context"
	"testing"

	"k8s.io/client-go/tools/record"
)

func TestGetUncachedObjects(t *testing.T) {
	GetUncachedObjects()
}

func TestRequestCtxMisc(t *testing.T) {
	itFuncs := func(reqCtx *RequestCtx) {
		reqCtx.Event(nil, "type", "reason", "msg")
		reqCtx.Eventf(nil, "type", "reason", "%s", "arg")
		if reqCtx != nil {
			reqCtx.UpdateCtxValue("key", "value")
			reqCtx.WithValue("key", "value")
		}
	}
	itFuncs(nil)
	itFuncs(&RequestCtx{
		Ctx:      context.Background(),
		Recorder: record.NewFakeRecorder(100),
	})
}
