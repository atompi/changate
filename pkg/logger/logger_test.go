package logger

import (
	"testing"
)

func TestInit(t *testing.T) {
	logger := Init()
	if logger == nil {
		t.Error("logger should not be nil after Init()")
	}
}
