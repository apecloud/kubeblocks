/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package actions

import (
	"sync"

	"github.com/pkg/errors"
)

type ActionRegister struct {
	actions map[string]Action
	lock    sync.Mutex
}

var register ActionRegister

func (r *ActionRegister) Register(name string, action Action) error {
	if _, ok := r.actions[name]; ok {
		return errors.New("Action already registered: " + name)
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	if r.actions == nil {
		r.actions = make(map[string]Action)
	}

	r.actions[name] = action
	return nil
}

func (r *ActionRegister) Operations() map[string]Action {
	return r.actions
}

func Register(name string, action Action) error {
	return register.Register(name, action)
}

func Operations() map[string]Action {
	return register.Operations()
}
