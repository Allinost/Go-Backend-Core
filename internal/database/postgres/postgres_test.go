package postgres

import (
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewPool_InvalidDSN(t *testing.T) {
	cfg := config.PGConfig{
		Host:   "127.0.0.1",
		Port:   9999,
		User:   "postgres",
		DBName: "test",
	}
	pool, err := NewPool(cfg)
	assert.Error(t, err)
	assert.Nil(t, pool)
}

func TestNewPool_EmptyConfig(t *testing.T) {
	pool, err := NewPool(config.PGConfig{})
	assert.Error(t, err)
	assert.Nil(t, pool)
}

func TestPool_Close_NilDB(t *testing.T) {
	p := &Pool{}
	err := p.Close()
	assert.NoError(t, err)
}

func TestPool_Health_NilDB(t *testing.T) {
	p := &Pool{}
	err := p.Health()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}
