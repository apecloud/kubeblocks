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

type StateInterface[C any] interface {
	comparable

	StateTransitionAction[C]
}

type StateTransitionAction[C any] interface {
	OnExit(ctx *C) error
	OnEnter(ctx *C) error
}

type BaseState[C any] struct {
	StateTransitionAction[C]
}

type StateDefinition[S StateInterface[C], E, C any] struct {
	State        S
	StateMachine *StateMachineDefinition[S, E, C]
	Superstate   *StateDefinition[S, E, C]

	// substates
	Substates []*StateDefinition[S, E, C]

	// transitions
	Transitions  []Transition
	EntryActions []func(ctx *C) error
	ExitActions  []func(ctx *C) error
}

func (b *BaseState[C]) OnExit(ctx *C) error {
	return nil
}

func (b *BaseState[C]) OnEnter(ctx *C) error {
	return nil
}

func (sd *StateDefinition[S, E, C]) OnExit(ctx *C) error {
	for _, action := range sd.ExitActions {
		if err := action(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (sd *StateDefinition[S, E, C]) OnEnter(ctx *C) error {
	for _, action := range sd.EntryActions {
		if err := action(ctx); err != nil {
			return err
		}
	}
	return nil
}
