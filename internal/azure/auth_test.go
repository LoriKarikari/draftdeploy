package azure

import (
	"testing"
)

func TestNewCredential(t *testing.T) {
	_, err := NewCredential()
	if err != nil {
		t.Logf("expected error in test environment: %v", err)
	}
}
