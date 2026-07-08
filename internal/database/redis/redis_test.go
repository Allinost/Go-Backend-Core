package redis

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	s := miniredis.RunT(t)

	client := &Client{
		Client: redis.NewClient(&redis.Options{Addr: s.Addr()}),
	}
	return client, s
}

func TestNewClient_EmptyConfig(t *testing.T) {
	client, err := NewClient(config.RedisInstance{})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_InvalidAddr(t *testing.T) {
	client, err := NewClient(config.RedisInstance{
		Addr: "127.0.0.1:1",
	})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_WithPoolSize(t *testing.T) {
	s := miniredis.RunT(t)
	defer s.Close()

	client, err := NewClient(config.RedisInstance{
		Addr:     s.Addr(),
		PoolSize: 5,
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 5, client.Options().PoolSize)
	client.Close()
}

func TestClient_Close(t *testing.T) {
	c := &Client{}
	err := c.Close()
	assert.NoError(t, err)
}

func TestClient_Close_WithConn(t *testing.T) {
	client, s := newTestClient(t)
	defer s.Close()

	err := client.Close()
	assert.NoError(t, err)
}

func TestClient_Health_NilClient(t *testing.T) {
	c := &Client{}
	err := c.Health()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestClient_Health_OK(t *testing.T) {
	client, s := newTestClient(t)
	defer s.Close()

	err := client.Health()
	assert.NoError(t, err)
}
