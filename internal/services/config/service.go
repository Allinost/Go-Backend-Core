package config

import (
	"fmt"
	"strings"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Service struct {
	reloaders  []config.Reloader
	validators []config.Validator
}

func New() *Service {
	return &Service{}
}

func (s *Service) RegisterReloader(r config.Reloader) {
	s.reloaders = append(s.reloaders, r)
}

func (s *Service) RegisterValidator(v config.Validator) {
	s.validators = append(s.validators, v)
}

func (s *Service) ReloadAll(cfg *config.Config) []error {
	var errs []error
	for _, r := range s.reloaders {
		if err := r.Reload(cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (s *Service) ValidateAll(cfg *config.Config) []error {
	var errs []error
	for _, v := range s.validators {
		if err := v.Validate(cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (s *Service) ValidateAllOrError(cfg *config.Config) error {
	errs := s.ValidateAll(cfg)
	if len(errs) == 0 {
		return nil
	}
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("配置校验失败:\n%s", strings.Join(msgs, "\n"))
}
