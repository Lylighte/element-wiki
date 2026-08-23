package store

import (
	"errors"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("ErrNotFound 应命中")
	}
	if IsNotFound(errors.New("other")) || IsNotFound(nil) {
		t.Error("其他错误不应命中")
	}
}
