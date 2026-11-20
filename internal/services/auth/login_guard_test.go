package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLoginGuardLockAndReset(t *testing.T) {
	g := NewLoginGuard(2, 30*time.Millisecond)
	key := "alice"
	if locked, _ := g.Check(context.Background(), key); locked {
		t.Fatal("initial should not be locked")
	}
	g.Fail(context.Background(), key)
	if locked, _ := g.Check(context.Background(), key); locked {
		t.Fatal("one failure should not lock")
	}
	g.Fail(context.Background(), key)
	if locked, remaining := g.Check(context.Background(), key); !locked || remaining <= 0 {
		t.Fatalf("two failures should lock, locked=%v remaining=%v", locked, remaining)
	}
	time.Sleep(40 * time.Millisecond)
	if locked, _ := g.Check(context.Background(), key); locked {
		t.Fatal("lock should expire")
	}
	g.Fail(context.Background(), key)
	g.Success(context.Background(), key)
	if locked, _ := g.Check(context.Background(), key); locked {
		t.Fatal("success should reset")
	}
}

func TestLoginGuardRedis(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer rdb.Close()

	g := NewLoginGuard(2, time.Minute)
	g.SetRedis(rdb)
	ctx := context.Background()
	key := "redis-alice"
	if locked, _ := g.Check(ctx, key); locked {
		t.Fatal("initial should not be locked")
	}
	g.Fail(ctx, key)
	if locked, _ := g.Check(ctx, key); locked {
		t.Fatal("one failure should not lock")
	}
	g.Fail(ctx, key)
	if locked, remaining := g.Check(ctx, key); !locked || remaining <= 0 {
		t.Fatalf("two failures should lock, locked=%v remaining=%v", locked, remaining)
	}
	g.Success(ctx, key)
	if locked, _ := g.Check(ctx, key); locked {
		t.Fatal("success should reset redis lock")
	}
}
