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

//type StateMachine[T any, S StateInterface, E Event, C] interface {

//states map[StateInterface]*StateDefinition
//}

type BuilderInterface[S StateInterface[C], E, C any] interface {
	OnEvent(event Event, destinationState S) BuilderInterface[S, E, C]
	Build() error
}

type StateBuilder[S StateInterface[C], E, C any] struct {
	BuilderInterface[S, E, C]

	State        S
	StateMachine *StateMachineDefinition[S, E, C]
	Superstate   *StateDefinition[S, E, C]

	// substates
	Substates []*StateDefinition[S, E, C]
}

type StateMachineDefinition[S StateInterface[C], E Event, C any] struct {
	StateMachineInterface

	name         string
	InitialState S
	states       map[S]*StateDefinition[S, E, C]
}

func NewStateMachine[S StateInterface[C], E, C any](id string, initialState S, _ func(_ S, _ E, _ C)) *StateMachineDefinition[S, E, C] {
	return &StateMachineDefinition[S, E, C]{
		name:         id,
		InitialState: initialState,
	}
}

func (smDef *StateMachineDefinition[S, E, C]) StateBuilder() BuilderInterface[S, E, C] {
	return &StateBuilder[S, E, C]{
		StateMachine: smDef,
	}
}

func (smDef *StateMachineDefinition[S, E, C]) stateDefinition(state S) (stateDef *StateDefinition[S, E, C]) {
	var ok bool
	if stateDef, ok = smDef.states[state]; !ok {
		stateDef = &StateDefinition[S, E, C]{
			StateMachine: smDef,
			State:        state,
		}
		smDef.states[state] = stateDef
	}
	return
}

func (smDef StateMachineDefinition[S, E, C]) ID() string {
	return smDef.name
}

func (builder *StateBuilder[S, E, C]) OnEvent(event Event, destinationState S) BuilderInterface[S, E, C] {
	return builder
}

func (builder *StateBuilder[S, E, C]) Build() error {
	sd := builder.StateMachine.stateDefinition(builder.State)
	sd.Superstate = builder.Superstate
	sd.Substates = builder.Substates
	return nil
}

func FromContext[S StateInterface[C], E, C any](ctx *C, id string, _ func(_ S, _ E, _ C)) *StateMachine[S, E, C] {
	stateReference := func(context *C) *BaseContext[S, C] {
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

	smDef := GetStateMachine[S, E, C](id)
	if smDef == nil {
		return nil
	}
	baseState := stateReference(ctx)
	if baseState == nil {
		baseState = &BaseContext[S, C]{}
		baseState.InitState(smDef.InitialState)
	}
	return &StateMachine[S, E, C]{
		context:                ctx,
		state:                  baseState,
		StateMachineDefinition: smDef,
	}
}
