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

package operations

import (
	"sync"

	"github.com/pkg/errors"
)

type Ops struct {
	ops  map[string]Operation
	lock sync.Mutex
}

var ops Ops

func (ops *Ops) Register(name string, op Operation) error {
	if _, ok := ops.ops[name]; ok {
		return errors.New("Operation already registered: " + name)
	}

	ops.lock.Lock()
	defer ops.lock.Unlock()
	if ops.ops == nil {
		ops.ops = make(map[string]Operation)
	}

	ops.ops[name] = op
	return nil
}

func (ops *Ops) Operations() map[string]Operation {
	return ops.ops
}

func Register(name string, op Operation) error {
	return ops.Register(name, op)
}

func Operations() map[string]Operation {
	return ops.Operations()
}
