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

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

// RequestCtx wrapper for reconcile procedure context parameters
type RequestCtx struct {
	Ctx      context.Context
	Req      ctrl.Request
	Log      logr.Logger
	Recorder record.EventRecorder
}

// UpdateCtxValue update Context value, return parent Context.
func (r *RequestCtx) UpdateCtxValue(key, val any) context.Context {
	p := r.Ctx
	r.Ctx = context.WithValue(r.Ctx, key, val)
	return p
}

// WithValue returns a copy of parent in which the value associated with key is
// val.
func (r *RequestCtx) WithValue(key, val any) context.Context {
	return context.WithValue(r.Ctx, key, val)
}
