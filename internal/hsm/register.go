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
	"sync"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var (
	locker          sync.Mutex
	stateMachineMap map[string]StateMachineInterface
)

func init() {
	stateMachineMap = make(map[string]StateMachineInterface)
}

func RegisterStateMachine(fsm StateMachineInterface) {
	locker.Lock()
	defer locker.Unlock()
	stateMachineMap[fsm.ID()] = fsm
}

func GetStateMachine[S StateInterface[C], E, C any](id string, _ func(_ S, _ E, _ C)) *StateMachineDefinition[S, E, C] {
	locker.Lock()
	defer locker.Unlock()
	if sm, ok := stateMachineMap[id]; ok {
		return sm.(*StateMachineDefinition[S, E, C])
	}
	return nil
}

func FromContext[S StateInterface[C], E, C any](ctx *C, id string, signature func(_ S, _ E, _ C)) (*StateMachine[S, E, C], error) {
	smDef := GetStateMachine(id, signature)
	if smDef == nil {
		return nil, cfgcore.MakeError("state machine not found: %s", id)
	}
	baseState := wrapStateReference(ctx, signature)
	if baseState == nil || baseState.reference == nil {
		baseState = &BaseContext[S, C]{}
		baseState.InitState(smDef.InitialState)
	}
	return NewStateMachineInstance(ctx, baseState, smDef, signature)
}
