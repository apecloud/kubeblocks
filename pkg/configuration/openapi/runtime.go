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

package openapi

import (
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type Runtime struct {
	ctx *cue.Context
	rt  cue.Value
}

func NewRuntime(cueString string) (*Runtime, error) {
	var ctx = cuecontext.New()

	cueOption := &load.Config{Stdin: strings.NewReader(cueString)}
	insts := load.Instances([]string{"-"}, cueOption)
	for _, ins := range insts {
		if err := ins.Err; err != nil {
			return nil, core.MakeError(errors.Details(err, nil))
		}
	}

	inst := insts[0]
	rt := cuecontext.New().BuildInstance(inst)
	if err := rt.Validate(cue.All()); err != nil {
		return nil, core.MakeError(errors.Details(err, nil))
	}

	return &Runtime{
		ctx: ctx,
		rt:  ctx.BuildInstance(inst),
	}, nil
}

func (r *Runtime) Underlying() cue.Value {
	return r.rt
}

func (r *Runtime) Context() *cue.Context {
	return r.ctx
}

func (r *Runtime) BuildFile(file *ast.File, options ...cue.BuildOption) cue.Value {
	return r.ctx.BuildFile(file, options...)
}
