package net

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultResolveHost(t *testing.T) {
	result, err := DefaultResolveHost(context.Background(), "localhost")
	require.NoError(t, err)
	assert.Equal(t, "localhost", result.Hostname)
	assert.NotEmpty(t, result.Addresses)
}

func TestResolveHost_Invalid(t *testing.T) {
	r := NewResolver(time.Second)
	_, err := r.LookupHost(context.Background(), "invalid-host-xxxxx.local")
	assert.Error(t, err)
}

func TestNewResolver(t *testing.T) {
	r := NewResolver(0)
	assert.NotNil(t, r)
	assert.Equal(t, 5*time.Second, r.timeout)
}

func TestNewResolverWithDNS(t *testing.T) {
	r := NewResolverWithDNS("8.8.8.8", 3*time.Second)
	assert.NotNil(t, r)
}
