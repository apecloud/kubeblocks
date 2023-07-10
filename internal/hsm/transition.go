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

type ActionInterface[E Event, C any] interface {
	Execute(ctx *C) error
}

type Guard[C any] interface {
	Condition(ctx *C) bool
}

type transitionGuard[C any] struct {
	Guards      []func(ctx *C) bool
	Description string
}

type Transition interface {
}

type basicTransition[E Event, C any] struct {
	Event  E
	Guards transitionGuard[C]
}

type internalTransition[E Event, C any] struct {
	// ActionInterface[E, C]
	basicTransition[E, C]
	actions []func(ctx *C) error
}

type NormalTransition[S StateInterface[C], E Event, C any] struct {
	destination S
	basicTransition[E, C]
}

func (t internalTransition[E, C]) Execute(ctx *C) error {
	for _, action := range t.actions {
		if err := action(ctx); err != nil {
			return err
		}
	}
	return nil
}

func newTransitionGuard[C any](guards ...func(ctx *C) bool) transitionGuard[C] {
	tGuard := transitionGuard[C]{
		Guards: guards,
	}
	return tGuard
}

func (tg transitionGuard[C]) Condition(ctx *C) bool {
	for _, guard := range tg.Guards {
		if !guard(ctx) {
			return false
		}
	}
	return true
}
