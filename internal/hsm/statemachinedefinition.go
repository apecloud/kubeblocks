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

//type StateMachine[T any, S StateInterface, E Event, C] interface {

//states map[StateInterface]*StateDefinition
//}

type BuilderInterface[S StateInterface[C], E, C any] interface {
	OnEnter(action func(ctx *C) error) BuilderInterface[S, E, C]
	OnExit(action func(ctx *C) error) BuilderInterface[S, E, C]

	Transition(event E, destinationState S, guards ...func(ctx *C) bool) BuilderInterface[S, E, C]
	InternalTransition(event E, action func(ctx *C) error, guards ...func(ctx *C) bool) BuilderInterface[S, E, C]

	Build() error
}

type StateBuilder[S StateInterface[C], E, C any] struct {
	BuilderInterface[S, E, C]

	Error           error
	State           S
	StateMachineRef *StateMachineDefinition[S, E, C]
	StateDefinition[S, E, C]
}

type StateMachineDefinition[S StateInterface[C], E Event, C any] struct {
	StateMachineInterface
	StatelessStateMachine[S, E, C]

	name      string
	recoverFn func(ctx *C) (S, error)

	InitialState S
	states       map[S]*StateDefinition[S, E, C]
}

func NewStateMachine[S StateInterface[C], E, C any](id string, initialState S, _ func(_ S, _ E, _ C)) *StateMachineDefinition[S, E, C] {
	return &StateMachineDefinition[S, E, C]{
		name:         id,
		InitialState: initialState,
	}
}

func (smDef *StateMachineDefinition[S, E, C]) StateBuilder(state S) BuilderInterface[S, E, C] {
	return &StateBuilder[S, E, C]{
		State:           state,
		StateMachineRef: smDef,
	}
}

func (smDef *StateMachineDefinition[S, E, C]) stateDefinition(state S) (stateDef *StateDefinition[S, E, C]) {
	var ok bool
	if stateDef, ok = smDef.states[state]; !ok {
		stateDef = &StateDefinition[S, E, C]{
			State:        state,
			Transitions:  make([]Transition, 0),
			StateMachine: smDef,
		}
		smDef.states[state] = stateDef
	}
	return
}

func (builder *StateBuilder[S, E, C]) OnEnter(action func(ctx *C) error) BuilderInterface[S, E, C] {
	builder.EntryActions = append(builder.EntryActions, action)
	return builder
}

func (builder *StateBuilder[S, E, C]) OnExit(action func(ctx *C) error) BuilderInterface[S, E, C] {
	builder.EntryActions = append(builder.EntryActions, action)
	return builder
}

func (builder *StateBuilder[S, E, C]) Transition(event E, destinationState S, guards ...func(ctx *C) bool) BuilderInterface[S, E, C] {
	buildFn := func() Transition {
		return &NormalTransition[S, E, C]{
			destination: destinationState,
			basicTransition: basicTransition[E, C]{
				Event:  event,
				Guards: newTransitionGuard(guards...),
			}}
	}
	return builder.buildWrapper(buildFn)
}

func (builder StateBuilder[S, E, C]) InternalTransition(event E, action func(ctx *C) error, guards ...func(ctx *C) bool) BuilderInterface[S, E, C] {
	buildFn := func() Transition {
		return &internalTransition[E, C]{
			actions: []func(ctx *C) error{action},
			basicTransition: basicTransition[E, C]{
				Event:  event,
				Guards: newTransitionGuard(guards...),
			}}
	}
	return builder.buildWrapper(buildFn)
}

func (builder *StateBuilder[S, E, C]) Build() error {
	if builder.Error != nil {
		return builder.Error
	}
	sd := builder.StateMachineRef.stateDefinition(builder.State)
	sd.EntryActions = builder.EntryActions
	sd.ExitActions = builder.ExitActions
	sd.Substates = builder.Substates
	sd.Superstate = builder.Superstate
	sd.Transitions = builder.Transitions
	return nil
}

func (builder *StateBuilder[S, E, C]) buildWrapper(fn func() Transition) BuilderInterface[S, E, C] {
	if builder.Error != nil || fn == nil {
		return builder
	}

	builder.Transitions = append(builder.Transitions, fn())
	return builder
}

func (smDef *StateMachineDefinition[S, E, C]) ID() string {
	return smDef.name
}

func (smDef *StateMachineDefinition[S, E, C]) OnRecover(recoverFn func(ctx *C) (S, error)) {
	smDef.recoverFn = recoverFn
}
