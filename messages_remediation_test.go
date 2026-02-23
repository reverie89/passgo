package main

import (
	"errors"
	"strings"
	"testing"
)

func TestRunBulkVMOperation(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		err := runBulkVMOperation("stop", []string{"vm1", "vm2"}, func(string) (string, error) {
			return "", nil
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("returns aggregated errors", func(t *testing.T) {
		expectedErr := errors.New("boom")
		err := runBulkVMOperation("start", []string{"vm1", "vm2", "vm3"}, func(name string) (string, error) {
			if name == "vm2" || name == "vm3" {
				return "", expectedErr
			}
			return "", nil
		})
		if err == nil {
			t.Fatalf("expected non-nil aggregated error")
		}
		if !strings.Contains(err.Error(), "start vm2") || !strings.Contains(err.Error(), "start vm3") {
			t.Fatalf("expected per-VM context in error, got %q", err.Error())
		}
	})
}
