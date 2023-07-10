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

package configuration

import (
	"fmt"

	"github.com/apecloud/kubeblocks/internal/hsm"
)

type ConfigStateType string

var ConfigFSMSignature = func(_ ConfigStateType, _ string, _ ConfigFSMContext) {}

func (c ConfigStateType) OnEnter(_ *ConfigFSMContext) error {
	return nil
}

func (c ConfigStateType) OnExit(_ *ConfigFSMContext) error {
	return nil
}

type ConfigFSMContext struct {
	hsm.BaseContext[ConfigStateType, ConfigFSMContext]
}

func NewConfigContext() *ConfigFSMContext {
	context := &ConfigFSMContext{}
	context.InitState("init")
	return context
}

func init() {
	sm := hsm.NewStateMachine("config-fsm", "init", ConfigFSMSignature)
	sm.StateBuilder("init").
		Transition("abcd", "abcde").
		Transition("abcd2", "abcde").
		Transition("abcd3", "abcde").
		Transition("abcd4", "abcde").
		InternalTransition("sync", func(_ *ConfigFSMContext) error {
			return nil
		}, func(ctx *ConfigFSMContext) bool {
			return false
		}).
		OnEnter(func(ctx *ConfigFSMContext) error {
			fmt.Printf("enter state: %s", ctx.GetState())
			return nil
		}).
		OnExit(func(ctx *ConfigFSMContext) error {
			fmt.Printf("exit state: %s", ctx.GetState())
			return nil
		}).
		Build()

	hsm.RegisterStateMachine(sm)
}
