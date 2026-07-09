package net

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckPort_Unreachable(t *testing.T) {
	ok := CheckPort(context.Background(), "127.0.0.1", 1, time.Second)
	assert.False(t, ok)
}

func TestScanPorts(t *testing.T) {
	result := ScanPorts(context.Background(), "127.0.0.1", []int{1, 2, 3}, time.Second)
	assert.Len(t, result, 3)
	for port, ok := range result {
		assert.False(t, ok, "port %d should be unreachable", port)
	}
}

func TestDefaultTCPConfig(t *testing.T) {
	cfg := DefaultTCPConfig()
	assert.Equal(t, 5*time.Second, cfg.ConnectTimeout)
	assert.Equal(t, 10*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.WriteTimeout)
}
