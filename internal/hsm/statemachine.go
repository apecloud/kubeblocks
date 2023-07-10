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

import (
	"container/list"
	"sync"
)

//type Context[S StateInterface[C], C any] interface {
//	context.Context
//}

type StateMachineInterface interface {
	ID() string
}

type StatelessStateMachine[S StateInterface[C], E Event, C any] interface {
	OnRecover(recoverFn func(ctx *C) (S, error))
}

type StateMachine[S StateInterface[C], E Event, C any] struct {
	*StateMachineDefinition[S, E, C]

	context    *C
	state      *BaseContext[S, C]
	eventQueue list.List
	mutex      sync.Mutex
}

func NewStateMachineInstance[S StateInterface[C], E Event, C any](ctx *C, baseState *BaseContext[S, C], smDef *StateMachineDefinition[S, E, C], _ func(_ S, _ E, _ C)) (*StateMachine[S, E, C], error) {
	sm := &StateMachine[S, E, C]{
		context:                ctx,
		state:                  baseState,
		StateMachineDefinition: smDef,
	}

	if sm.recoverFn == nil {
		return sm, nil
	}
	newState, err := sm.recoverFn(ctx)
	if err != nil {
		return nil, err
	}
	sm.state.SetState(newState)
	return sm, err
}
