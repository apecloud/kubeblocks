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
	"fmt"
)

type unavailableError struct {
	context string
}

func (e *unavailableError) Error() string {
	return fmt.Sprintf("cluster %s is unavailable", e.context)
}

func genericUnavailableError(context string) error {
	return &unavailableError{context}
}

func getUnavailableError(context string) error {
	return &unavailableError{context}
}

func listUnavailableError(context string) error {
	return &unavailableError{context}
}

func IsUnavailableError(err error) bool {
	_, ok := err.(*unavailableError)
	return ok
}
