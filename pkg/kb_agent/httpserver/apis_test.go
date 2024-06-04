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

package httpserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
)

func TestRegisterOperations(t *testing.T) {
	fakeAPI := &api{}
	fakeOps := map[string]actions.Action{
		"fake-1": actions.NewFakeAction(actions.FakeInit, func(ctx context.Context) error {
			return fmt.Errorf("some error")
		}),
		"fake-2": actions.NewFakeAction(actions.FakeIsReadOnly, func(ctx context.Context) bool {
			return true
		}),
		"fake-3": actions.NewFakeAction(actions.FakeDefault, nil),
	}

	fakeAPI.RegisterOperations(fakeOps)
	assert.True(t, fakeAPI.ready)
	assert.Equal(t, 2, len(fakeAPI.endpoints))
	assert.Equal(t, "v1.0", fakeAPI.endpoints[0].Version)
}
