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

package operations

import (
	"context"

	"github.com/pkg/errors"
)

type Operation interface {
	Init(context.Context) error
	IsReadonly(context.Context) bool
	PreCheck(context.Context, *OpsRequest) error
	Do(context.Context, *OpsRequest) (*OpsResponse, error)
}

type Base struct {
}

func (b *Base) Init(ctx context.Context) error {
	return nil
}

func (b *Base) IsReadonly(ctx context.Context) bool {
	return false
}

func (b *Base) PreCheck(ctx context.Context, request *OpsRequest) error {
	return nil
}

func (b *Base) Do(ctx context.Context, request *OpsRequest) (*OpsResponse, error) {
	return nil, errors.New("not implemented")
}
