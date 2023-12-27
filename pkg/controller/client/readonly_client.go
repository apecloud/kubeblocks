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

package multicluster

import (
	"context"
)

func IntoContext(ctx context.Context, placement string) context.Context {
	return context.WithValue(ctx, placementKey{}, placement)
}

func FromContext(ctx context.Context) (string, error) {
	if v, ok := ctx.Value(placementKey{}).(string); ok {
		return v, nil
	}
	return "", placementNotFoundError{}
}

type placementKey struct{}

type placementNotFoundError struct{}

func (placementNotFoundError) Error() string {
	return "no placement was present"
}
