package cache

import (
	"context"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitAll_NoRedis(t *testing.T) {
	err := InitAll(&config.Config{
		Cache: config.CacheConfig{
			DefaultTTL:    "5m",
			DefaultNilTTL: "30s",
			Jitter:        "10s",
			L1MaxSize:     1000,
			RedisKey:      "",
		},
	})
	require.NoError(t, err)
	t.Cleanup(CloseAll)

	c := Get("default")
	require.NotNil(t, c)

	ctx := context.Background()
	err = c.Set(ctx, "k", "v", 0)
	require.NoError(t, err)

	var val string
	err = c.Get(ctx, "k", &val)
	require.NoError(t, err)
	assert.Equal(t, "v", val)
}

func TestInitAll_DefaultConfig(t *testing.T) {
	err := InitAll(&config.Config{Cache: config.CacheConfig{}})
	require.NoError(t, err)
	t.Cleanup(CloseAll)

	c := Get("default")
	require.NotNil(t, c)

	stats := c.Stats()
	assert.Equal(t, 0, stats.L1Count)
}

func TestGet_Unknown(t *testing.T) {
	CloseAll()
	c := Get("nonexistent")
	assert.Nil(t, c, "Get should return nil when no caches are initialized")
}
