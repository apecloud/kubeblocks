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

package multicluster

import (
	"errors"
	"fmt"
)

type unavailableError struct {
	call string
}

func (e *unavailableError) Error() string {
	return fmt.Sprintf("unavailable for %s call", e.call)
}

var _ error = &unavailableError{}

func NewUnavailableError(call string) error {
	return &unavailableError{call}
}

func IsUnavailableError(err error) bool {
	return errors.Is(err, &unavailableError{})
}
