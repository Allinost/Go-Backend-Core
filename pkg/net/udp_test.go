package net

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultUDPConfig(t *testing.T) {
	cfg := DefaultUDPConfig()
	assert.Equal(t, 5*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
	assert.Equal(t, []byte{0x00}, cfg.ProbeData)
}

func TestCheckUDP_Unreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := CheckUDP(ctx, "127.0.0.1:1")
	_ = err
}

func TestCheckUDP_InvalidAddr(t *testing.T) {
	err := CheckUDP(context.Background(), "invalid-addr")
	assert.Error(t, err)
}
