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

package util

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSentEventForProbe(t *testing.T) {
	t.Run("action is empty", func(t *testing.T) {
		msg := &MockNilActionMessage{}
		err := SentEventForProbe(nil, msg)
		assert.Error(t, errors.New("action is unset"), err)
	})
	t.Run("action is not empty", func(t *testing.T) {
		msg := &MockNotNilActionMessage{}
		err := SentEventForProbe(nil, msg)
		assert.Nil(t, err)
	})
}

func TestCreateEvent(t *testing.T) {
	msg := &MockNotNilActionMessage{}
	event, err := CreateEvent("test", msg)
	assert.Nil(t, err)
	assert.NotEqual(t, nil, event)
}

type MockNotNilActionMessage struct {
}

func (m *MockNotNilActionMessage) GetAction() string {
	return "test"
}

type MockNilActionMessage struct {
}

func (m *MockNilActionMessage) GetAction() string {
	return ""
}
