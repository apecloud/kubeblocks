/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package lifecycle

import (
	"errors"
)

var (
	ErrActionNotDefined     = errors.New("action is not defined")
	ErrActionNotImplemented = errors.New("action is not implemented")
	ErrPreconditionFailed   = errors.New("action precondition is not matched")
	ErrActionInProgress     = errors.New("action is in progress")
	ErrActionBusy           = errors.New("action is busy")
	ErrActionTimedOut       = errors.New("action timed-out")
	ErrActionFailed         = errors.New("action failed")
	ErrActionInternalError  = errors.New("action internal error")
)

func IgnoreNotDefined(err error) error {
	if errors.Is(err, ErrActionNotDefined) {
		return nil
	}
	return err
}
