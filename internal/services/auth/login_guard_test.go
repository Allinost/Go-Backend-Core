package auth

import (
	"testing"
	"time"
)

func TestLoginGuardLockAndReset(t *testing.T) {
	g := NewLoginGuard(2, 30*time.Millisecond)
	key := "alice"
	if locked, _ := g.Check(key); locked {
		t.Fatal("initial should not be locked")
	}
	g.Fail(key)
	if locked, _ := g.Check(key); locked {
		t.Fatal("one failure should not lock")
	}
	g.Fail(key)
	if locked, remaining := g.Check(key); !locked || remaining <= 0 {
		t.Fatalf("two failures should lock, locked=%v remaining=%v", locked, remaining)
	}
	time.Sleep(40 * time.Millisecond)
	if locked, _ := g.Check(key); locked {
		t.Fatal("lock should expire")
	}
	g.Fail(key)
	g.Success(key)
	if locked, _ := g.Check(key); locked {
		t.Fatal("success should reset")
	}
}
