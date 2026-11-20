package ops

import (
	"path/filepath"
	"testing"
)

func TestHostKeyCallbackSwitch(t *testing.T) {
	if _, err := hostKeyCallback(false, ""); err != nil {
		t.Fatalf("disabled callback should not fail: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "known_hosts")
	if _, err := hostKeyCallback(true, missing); err == nil {
		t.Fatal("enabled callback with missing known_hosts should fail")
	}
}
