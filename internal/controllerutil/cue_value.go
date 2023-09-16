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

package controllerutil

import (
	"encoding/json"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
)

func NewCUETplFromPath(filePathString string) (*CUETpl, error) {
	return NewCUETplFromBytes(os.ReadFile(filePathString))
}

func NewCUETplFromBytes(b []byte, err error) (*CUETpl, error) {
	if err != nil {
		return nil, err
	}
	return NewCUETpl(b), nil
}

func NewCUETpl(templateContents []byte) *CUETpl {
	cueValue := CUETpl{
		Ctx: cuecontext.New(),
	}
	temValue := cueValue.Ctx.CompileBytes(templateContents)
	cueValue.Value = temValue
	return &cueValue
}

type CUETpl struct {
	Ctx   *cue.Context
	Value cue.Value
}

type CUEBuilder struct {
	cueTplValue CUETpl
	Value       cue.Value
}

func NewCUEBuilder(cueTpl CUETpl) CUEBuilder {
	return CUEBuilder{
		cueTplValue: cueTpl,
		Value:       cueTpl.Value,
	}
}

func (v *CUEBuilder) FillObj(path string, obj any) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return v.Fill(path, b)
}

func (v *CUEBuilder) Fill(path string, jsonByte []byte) error {
	expr, err := cuejson.Extract("", jsonByte)
	if err != nil {
		return err
	}
	cueValue := v.cueTplValue.Ctx.BuildExpr(expr)
	v.Value = v.Value.FillPath(cue.ParsePath(path), cueValue)
	return nil
}

func (v *CUEBuilder) Lookup(path string) ([]byte, error) {
	cueValue := v.Value.LookupPath(cue.ParsePath(path))
	return cueValue.MarshalJSON()
}

// func (v *CueValue) Render() (string, error) {
// 	b, err := v.Value.MarshalJSON()
// 	str := string(b)
// 	return str, err
// }
