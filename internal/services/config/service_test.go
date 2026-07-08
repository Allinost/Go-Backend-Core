package config

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

type mockReloader struct {
	fn     func(cfg *config.Config) error
	called atomic.Int32
}

func (m *mockReloader) Reload(cfg *config.Config) error {
	m.called.Add(1)
	return m.fn(cfg)
}

func TestRegisterReloader(t *testing.T) {
	s := New()
	r1 := &mockReloader{fn: func(cfg *config.Config) error { return nil }}
	r2 := &mockReloader{fn: func(cfg *config.Config) error { return nil }}

	s.RegisterReloader(r1)
	s.RegisterReloader(r2)

	cfg := &config.Config{}
	errs := s.ReloadAll(cfg)
	assert.Empty(t, errs)
	assert.Equal(t, int32(1), r1.called.Load())
	assert.Equal(t, int32(1), r2.called.Load())
}

func TestReload_PropagatesToAll(t *testing.T) {
	s := New()
	var order []string

	r1 := &mockReloader{fn: func(cfg *config.Config) error { order = append(order, "r1"); return nil }}
	r2 := &mockReloader{fn: func(cfg *config.Config) error { order = append(order, "r2"); return nil }}

	s.RegisterReloader(r1)
	s.RegisterReloader(r2)
	s.ReloadAll(&config.Config{})

	assert.Equal(t, []string{"r1", "r2"}, order)
}

func TestReload_SingleFailureDoesNotBlockOthers(t *testing.T) {
	s := New()
	var called []string

	r1 := &mockReloader{fn: func(cfg *config.Config) error { return errors.New("r1 failed") }}
	r2 := &mockReloader{fn: func(cfg *config.Config) error { called = append(called, "r2"); return nil }}
	r3 := &mockReloader{fn: func(cfg *config.Config) error { called = append(called, "r3"); return nil }}

	s.RegisterReloader(r1)
	s.RegisterReloader(r2)
	s.RegisterReloader(r3)

	errs := s.ReloadAll(&config.Config{})
	assert.Len(t, errs, 1)
	assert.Equal(t, "r1 failed", errs[0].Error())
	assert.Equal(t, []string{"r2", "r3"}, called)
}

func TestReload_NoReloaders(t *testing.T) {
	s := New()
	errs := s.ReloadAll(&config.Config{})
	assert.Empty(t, errs)
}

func TestReload_AllFail(t *testing.T) {
	s := New()
	r1 := &mockReloader{fn: func(cfg *config.Config) error { return errors.New("fail1") }}
	r2 := &mockReloader{fn: func(cfg *config.Config) error { return errors.New("fail2") }}

	s.RegisterReloader(r1)
	s.RegisterReloader(r2)

	errs := s.ReloadAll(&config.Config{})
	assert.Len(t, errs, 2)
}
