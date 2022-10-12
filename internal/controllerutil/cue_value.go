/*
Copyright 2022 The KubeBlocks Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllerutil

import (
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
	return NewCUETpl(string(b)), nil
}

func NewCUETpl(templateContents string) *CUETpl {
	cueValue := CUETpl{
		Ctx: cuecontext.New(),
	}
	temValue := cueValue.Ctx.CompileString(templateContents)
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

func (v *CUEBuilder) Fill(path string, jsonByte []byte) error {
	expr, err := cuejson.Extract("", jsonByte)
	if err != nil {
		return err
	}
	cueValue := v.cueTplValue.Ctx.BuildExpr(expr)
	v.Value = v.Value.FillPath(cue.ParsePath(path), cueValue)
	return nil
}

func (v *CUEBuilder) FillRaw(path string, value interface{}) error {
	v.Value = v.Value.FillPath(cue.ParsePath(path), value)
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
