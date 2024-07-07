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
