package config

import (
	"fmt"
	"strings"

	"github.com/Allinost/go-backend-core/internal/config"
)

// Service 配置服务，管理重载器和校验器的注册与执行
type Service struct {
	reloaders  []config.Reloader  // 配置重载器列表
	validators []config.Validator // 配置校验器列表
}

// New 创建配置服务实例
func New() *Service {
	return &Service{}
}

// RegisterReloader 注册配置重载器
func (s *Service) RegisterReloader(r config.Reloader) {
	s.reloaders = append(s.reloaders, r)
}

// RegisterValidator 注册配置校验器
func (s *Service) RegisterValidator(v config.Validator) {
	s.validators = append(s.validators, v)
}

// ReloadAll 依次调用所有注册的重载器，返回所有错误
func (s *Service) ReloadAll(cfg *config.Config) []error {
	var errs []error
	for _, r := range s.reloaders {
		if err := r.Reload(cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ValidateAll 遍历所有注册的校验器，返回所有校验失败的错误
func (s *Service) ValidateAll(cfg *config.Config) []error {
	var errs []error
	for _, v := range s.validators {
		if err := v.Validate(cfg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ValidateAllOrError 执行全部校验，合并所有错误为单个错误
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
