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

package hsm

import "reflect"

type StateReference[S any] struct {
	State S
}

type BaseContext[S StateInterface[C], C any] struct {
	reference *StateReference[S]
}

func (c *BaseContext[S, C]) GetState() S {
	return c.reference.State
}

func (c *BaseContext[S, C]) SetState(newState S) {
	c.reference.State = newState
}

func (c *BaseContext[S, C]) InitState(initialState S) {
	c.reference = &StateReference[S]{State: initialState}
}

func wrapStateReference[S StateInterface[C], E, C any](ctx *C, _ func(_ S, _ E, _ C)) *BaseContext[S, C] {
	v := reflect.ValueOf(ctx)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	v = v.FieldByName("BaseContext")
	if !v.IsValid() {
		return nil
	}
	switch i := v.Interface().(type) {
	default:
		return nil
	case BaseContext[S, C]:
		return &i
	case *BaseContext[S, C]:
		return i
	}
}
