package configcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Service 配置中心：注册表 + 持久化 + 运行时应用钩子。
type Service struct {
	registry *Registry
	store    Store
	apply    func(values map[string]any) error

	mu     sync.RWMutex
	values map[string]any
	cipher SecretCipher
}

func New(registry *Registry, store Store) *Service {
	return &Service{
		registry: registry,
		store:    store,
		values:   registry.Defaults(),
	}
}

// SetApply 设置运行时应用钩子；配置更新成功后调用。
func (s *Service) SetApply(fn func(values map[string]any) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apply = fn
}

// Load 从存储加载并覆盖默认值。
func (s *Service) Load(ctx context.Context) error {
	stored, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	values := s.registry.Defaults()
	for key, raw := range stored {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("configcenter: 解析 %s 失败: %w", key, err)
		}
		if spec, ok := s.registry.Get(key); ok {
			opened, err := s.openSecret(spec, value)
			if err != nil {
				return err
			}
			value = opened
		}
		values[key] = value
	}
	if err := s.registry.Validate(values); err != nil {
		return err
	}
	s.mu.Lock()
	s.values = values
	s.mu.Unlock()
	return nil
}

// ApplyCurrent 将当前值应用到运行时钩子（启动加载后调用）。
func (s *Service) ApplyCurrent() error {
	s.mu.RLock()
	values := make(map[string]any, len(s.values))
	for k, v := range s.values {
		values[k] = v
	}
	apply := s.apply
	s.mu.RUnlock()
	if apply == nil {
		return nil
	}
	return apply(values)
}

// Specs 返回全部配置项定义。
func (s *Service) Specs() []Spec {
	return s.registry.Specs()
}

// Values 返回当前配置值；mask=true 时敏感项脱敏。
func (s *Service) Values(mask bool) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]any, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	if mask {
		return s.registry.Mask(out)
	}
	return out
}

// ValidatePatch 校验并归一化补丁，不落库。
func (s *Service) ValidatePatch(patch map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(patch))
	for key, value := range patch {
		spec, ok := s.registry.Get(key)
		if !ok {
			return nil, fmt.Errorf("configcenter: 未知配置项 %s", key)
		}
		v, err := normalizeValue(spec, value)
		if err != nil {
			return nil, fmt.Errorf("configcenter: %s %w", key, err)
		}
		normalized[key] = v
	}

	s.mu.RLock()
	merged := make(map[string]any, len(s.values)+len(normalized))
	for k, v := range s.values {
		merged[k] = v
	}
	s.mu.RUnlock()
	for k, v := range normalized {
		merged[k] = v
	}
	if err := s.registry.Validate(merged); err != nil {
		return nil, err
	}
	return normalized, nil
}

// Update 校验、持久化并应用配置补丁。
func (s *Service) Update(ctx context.Context, patch map[string]any, operator, note string) error {
	normalized, err := s.ValidatePatch(patch)
	if err != nil {
		return err
	}
	// 前端可能回传脱敏占位符：视为“不修改”，避免把 ****** 写进配置。
	for key, value := range normalized {
		if isMasked(value) {
			delete(normalized, key)
		}
	}
	if len(normalized) == 0 {
		return nil
	}

	raw := make(map[string]json.RawMessage, len(normalized))
	for key, value := range normalized {
		spec, _ := s.registry.Get(key)
		stored, err := s.sealSecret(spec, value)
		if err != nil {
			return err
		}
		b, err := json.Marshal(stored)
		if err != nil {
			return fmt.Errorf("configcenter: 序列化 %s 失败: %w", key, err)
		}
		raw[key] = b
	}
	if err := s.store.Save(ctx, raw, operator, note); err != nil {
		return err
	}

	s.mu.Lock()
	for k, v := range normalized {
		s.values[k] = v
	}
	values := make(map[string]any, len(s.values))
	for k, v := range s.values {
		values[k] = v
	}
	apply := s.apply
	s.mu.Unlock()

	if apply != nil {
		if err := apply(values); err != nil {
			return fmt.Errorf("configcenter: 应用配置失败: %w", err)
		}
	}
	return nil
}

// Reset 将指定 key 重置为默认值；keys 为空时重置全部。
func (s *Service) Reset(ctx context.Context, keys []string, operator, note string) error {
	if len(keys) == 0 {
		for _, spec := range s.registry.Specs() {
			keys = append(keys, spec.Key)
		}
	}
	if err := s.store.Reset(ctx, keys, operator, note); err != nil {
		return err
	}

	defaults := s.registry.Defaults()
	s.mu.Lock()
	for _, key := range keys {
		if v, ok := defaults[key]; ok {
			s.values[key] = v
		}
	}
	values := make(map[string]any, len(s.values))
	for k, v := range s.values {
		values[k] = v
	}
	apply := s.apply
	s.mu.Unlock()

	if apply != nil {
		if err := apply(values); err != nil {
			return fmt.Errorf("configcenter: 应用配置失败: %w", err)
		}
	}
	return nil
}

func (s *Service) History(ctx context.Context, key string, limit int) ([]HistoryEntry, error) {
	entries, err := s.store.History(ctx, key, limit)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		spec, ok := s.registry.Get(entries[i].Key)
		if !ok || !spec.Sensitive {
			continue
		}
		entries[i].OldValue = maskHistoryValue(entries[i].OldValue)
		entries[i].NewValue = maskHistoryValue(entries[i].NewValue)
	}
	return entries, nil
}

// maskHistoryValue 对敏感项历史值脱敏；保留 null 语义。
func maskHistoryValue(value json.RawMessage) json.RawMessage {
	if len(value) == 0 || string(value) == "null" {
		return value
	}
	return json.RawMessage(`"******"`)
}
