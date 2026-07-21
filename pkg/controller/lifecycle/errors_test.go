/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package lifecycle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsActionPending(t *testing.T) {
	testCases := []struct {
		name      string
		err       error
		retryable bool
	}{
		{name: "in progress", err: ErrActionInProgress, retryable: true},
		{name: "busy", err: ErrActionBusy, retryable: true},
		{name: "precondition", err: ErrPreconditionFailed, retryable: true},
		{name: "wrapped retry", err: fmt.Errorf("pod: %w", ErrActionBusy), retryable: true},
		{
			name: "aggregate retry states",
			err: newActionAggregateError([]error{
				fmt.Errorf("pod-0: %w", ErrActionBusy),
				fmt.Errorf("pod-1: %w", ErrPreconditionFailed),
			}),
			retryable: true,
		},
		{
			name: "aggregate retry and failure",
			err: newActionAggregateError([]error{
				fmt.Errorf("pod-0: %w", ErrActionInProgress),
				fmt.Errorf("pod-1: %w", ErrActionFailed),
			}),
			retryable: false,
		},
		{name: "failed", err: ErrActionFailed, retryable: false},
		{name: "nil", err: nil, retryable: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.retryable, IsActionPending(testCase.err))
		})
	}
}
