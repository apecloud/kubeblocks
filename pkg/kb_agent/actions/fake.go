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
	"context"
	"time"
)

type FakeFuncType string

const (
	FakeInit       FakeFuncType = "fake-init"
	FakeIsReadOnly FakeFuncType = "fake-is-read-only"
	FakePreCheck   FakeFuncType = "fake-pre-check"
	FakeDo         FakeFuncType = "fake-do"
	FakeDefault    FakeFuncType = "fake-default"
)

type FakeOperations struct {
	InitFunc       func(ctx context.Context) error
	IsReadOnlyFunc func(ctx context.Context) bool
	PreCheckFunc   func(ctx context.Context, request *OpsRequest) error
	DoFunc         func(ctx context.Context, request *OpsRequest) (*OpsResponse, error)
}

func NewFakeOperations(funcType FakeFuncType, fakeFunc interface{}) *FakeOperations {
	op := &FakeOperations{
		InitFunc: func(ctx context.Context) error {
			return nil
		},
		IsReadOnlyFunc: func(ctx context.Context) bool {
			return false
		},
		PreCheckFunc: func(ctx context.Context, request *OpsRequest) error {
			return nil
		},
		DoFunc: func(ctx context.Context, request *OpsRequest) (*OpsResponse, error) {
			return nil, nil
		},
	}

	switch funcType {
	case FakeInit:
		op.InitFunc = fakeFunc.(func(ctx context.Context) error)
	case FakeIsReadOnly:
		op.IsReadOnlyFunc = fakeFunc.(func(ctx context.Context) bool)
	case FakePreCheck:
		op.PreCheckFunc = fakeFunc.(func(ctx context.Context, request *OpsRequest) error)
	case FakeDo:
		op.DoFunc = fakeFunc.(func(ctx context.Context, request *OpsRequest) (*OpsResponse, error))
	}
	return op
}

func (f *FakeOperations) Init(ctx context.Context) error {
	return f.InitFunc(ctx)
}

func (f *FakeOperations) SetTimeout(timeout time.Duration) {
}

func (f *FakeOperations) IsReadonly(ctx context.Context) bool {
	return f.IsReadOnlyFunc(ctx)
}

func (f *FakeOperations) PreCheck(ctx context.Context, request *OpsRequest) error {
	return f.PreCheckFunc(ctx, request)
}

func (f *FakeOperations) Do(ctx context.Context, request *OpsRequest) (*OpsResponse, error) {
	return f.DoFunc(ctx, request)
}
