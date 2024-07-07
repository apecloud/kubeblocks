package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetClientSet(t *testing.T) {
	set, err := GetClientSet()
	assert.Nil(t, err)
	assert.NotNil(t, set)
}
