package config

import (
	"errors"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

type mockValidator struct {
	name string
	fn   func(cfg *config.Config) error
}

func (m *mockValidator) Validate(cfg *config.Config) error {
	return m.fn(cfg)
}

func TestRegisterValidator(t *testing.T) {
	s := New()
	v1 := &mockValidator{name: "v1", fn: func(cfg *config.Config) error { return nil }}
	v2 := &mockValidator{name: "v2", fn: func(cfg *config.Config) error { return nil }}

	s.RegisterValidator(v1)
	s.RegisterValidator(v2)

	errs := s.ValidateAll(&config.Config{})
	assert.Empty(t, errs)
}

func TestValidate_AllPass(t *testing.T) {
	s := New()
	s.RegisterValidator(&mockValidator{name: "db", fn: func(cfg *config.Config) error { return nil }})
	s.RegisterValidator(&mockValidator{name: "auth", fn: func(cfg *config.Config) error { return nil }})
	s.RegisterValidator(&mockValidator{name: "redis", fn: func(cfg *config.Config) error { return nil }})

	errs := s.ValidateAll(&config.Config{})
	assert.Empty(t, errs)
}

func TestValidate_PartialFailure(t *testing.T) {
	s := New()
	s.RegisterValidator(&mockValidator{name: "db", fn: func(cfg *config.Config) error { return nil }})
	s.RegisterValidator(&mockValidator{name: "auth", fn: func(cfg *config.Config) error { return errors.New("auth: jwt_secret 为空") }})
	s.RegisterValidator(&mockValidator{name: "redis", fn: func(cfg *config.Config) error { return errors.New("redis: addr 为空") }})

	errs := s.ValidateAll(&config.Config{})
	assert.Len(t, errs, 2)
}

func TestValidate_ErrorAggregation(t *testing.T) {
	s := New()
	s.RegisterValidator(&mockValidator{name: "auth", fn: func(cfg *config.Config) error { return errors.New("jwt_secret 为空") }})

	err := s.ValidateAllOrError(&config.Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "配置校验失败")
	assert.Contains(t, err.Error(), "jwt_secret 为空")
}

func TestValidate_NoValidators(t *testing.T) {
	s := New()
	errs := s.ValidateAll(&config.Config{})
	assert.Empty(t, errs)
}

func TestValidateAllOrError_Success(t *testing.T) {
	s := New()
	s.RegisterValidator(&mockValidator{name: "db", fn: func(cfg *config.Config) error { return nil }})
	err := s.ValidateAllOrError(&config.Config{})
	assert.NoError(t, err)
}
