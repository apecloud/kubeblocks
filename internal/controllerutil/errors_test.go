package controllerutil

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
)

func TestNerError(t *testing.T) {
	err1 := NewError(ErrorTypeBackupNotCompleted, "test c2")
	if err1.Error() != "test c2" {
		t.Error("NewErrorf failed")
	}
}

func TestNerErrorf(t *testing.T) {
	err1 := NewErrorf(ErrorTypeBackupNotCompleted, "test %s %s", "c1", "c2")
	if err1.Error() != "test c1 c2" {
		t.Error("NewErrorf failed")
	}
	testError := fmt.Errorf("test: %w", err1)
	if !errors.Is(testError, err1) {
		t.Error("errors.Is failed")
	}

	var target *Error
	if !errors.As(testError, &target) {
		t.Error("errors.As failed")
	}
}
