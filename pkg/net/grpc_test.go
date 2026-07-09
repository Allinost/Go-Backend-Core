package net

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultGRPCConfig(t *testing.T) {
	cfg := DefaultGRPCConfig("localhost:9090")
	assert.Equal(t, "localhost:9090", cfg.Address)
	assert.Equal(t, 2, cfg.MaxRetries)
	assert.True(t, cfg.Insecure)
}

func TestNewGRPCClient_InvalidAddr(t *testing.T) {
	cfg := GRPCConfig{
		Address:  "invalid-addr-xxxxx:99999",
		Timeout:  1,
		Insecure: true,
	}
	_, err := NewGRPCClient(cfg)
	assert.Error(t, err)
}
