package configcenter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ValueType 配置值类型
type ValueType string

const (
	TypeString   ValueType = "string"
	TypeInt      ValueType = "int"
	TypeBool     ValueType = "bool"
	TypeDuration ValueType = "duration"
	TypeJSON     ValueType = "json"
	TypeSecret   ValueType = "secret"
)

// Spec 配置项定义。模块通过注册 Spec 把自己的可配置能力暴露给配置中心。
type Spec struct {
	Key         string    `json:"key"`
	Module      string    `json:"module"`
	Type        ValueType `json:"type"`
	Default     any       `json:"default"`
	Enum        []string  `json:"enum,omitempty"`
	Min         *float64  `json:"min,omitempty"`
	Max         *float64  `json:"max,omitempty"`
	Sensitive   bool      `json:"sensitive"`
	Reloadable  bool      `json:"reloadable"`
	EnvLocked   bool      `json:"env_locked"`
	Group       string    `json:"group"`
	Description string    `json:"description"`
}

// Registry 配置项注册表
type Registry struct {
	mu    sync.RWMutex
	specs map[string]Spec
}

func NewRegistry() *Registry {
	return &Registry{specs: make(map[string]Spec)}
}

// Register 注册配置项；重复 key 或非法定义会返回错误。
func (r *Registry) Register(specs ...Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, spec := range specs {
		if err := normalizeSpec(&spec); err != nil {
			return err
		}
		if _, exists := r.specs[spec.Key]; exists {
			return fmt.Errorf("configcenter: 重复配置项 %s", spec.Key)
		}
		r.specs[spec.Key] = spec
	}
	return nil
}

func (r *Registry) Get(key string) (Spec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.specs[key]
	return s, ok
}

func (r *Registry) Specs() []Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Spec, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (r *Registry) Defaults() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]any, len(r.specs))
	for k, s := range r.specs {
		out[k] = s.Default
	}
	return out
}

// Validate 校验完整配置值集合；未知 key 报错。
func (r *Registry) Validate(values map[string]any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for key, value := range values {
		spec, ok := r.specs[key]
		if !ok {
			return fmt.Errorf("configcenter: 未知配置项 %s", key)
		}
		if err := validateValue(spec, value); err != nil {
			return fmt.Errorf("configcenter: %s %w", key, err)
		}
	}
	return nil
}

// Mask 返回脱敏后的配置值，敏感项只保留末 4 位。
func (r *Registry) Mask(values map[string]any) map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]any, len(values))
	for key, value := range values {
		spec, ok := r.specs[key]
		if ok && spec.Sensitive {
			out[key] = maskSecret(value)
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeSpec(spec *Spec) error {
	spec.Key = strings.TrimSpace(spec.Key)
	if spec.Key == "" {
		return fmt.Errorf("configcenter: 配置项 key 不能为空")
	}
	if spec.Type == "" {
		spec.Type = TypeString
	}
	switch spec.Type {
	case TypeString, TypeInt, TypeBool, TypeDuration, TypeJSON, TypeSecret:
	default:
		return fmt.Errorf("configcenter: %s 类型不支持: %s", spec.Key, spec.Type)
	}
	if spec.Default == nil {
		switch spec.Type {
		case TypeString, TypeDuration, TypeSecret:
			spec.Default = ""
		case TypeInt:
			spec.Default = 0
		case TypeBool:
			spec.Default = false
		case TypeJSON:
			spec.Default = json.RawMessage(`{}`)
		}
	}
	normalized, err := normalizeValue(*spec, spec.Default)
	if err != nil {
		return fmt.Errorf("configcenter: %s 默认值非法: %w", spec.Key, err)
	}
	spec.Default = normalized
	return nil
}

func validateValue(spec Spec, value any) error {
	if value == nil {
		return nil
	}
	switch spec.Type {
	case TypeString, TypeSecret:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("必须是字符串")
		}
	case TypeInt:
		n, err := toInt(value)
		if err != nil {
			return err
		}
		if spec.Min != nil && float64(n) < *spec.Min {
			return fmt.Errorf("不能小于 %v", *spec.Min)
		}
		if spec.Max != nil && float64(n) > *spec.Max {
			return fmt.Errorf("不能大于 %v", *spec.Max)
		}
	case TypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("必须是布尔值")
		}
	case TypeDuration:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("必须是时长字符串")
		}
		if _, err := time.ParseDuration(s); err != nil {
			return fmt.Errorf("时长格式错误: %w", err)
		}
	case TypeJSON:
		switch value.(type) {
		case json.RawMessage, map[string]any, []any:
		default:
			return fmt.Errorf("必须是 JSON")
		}
	}

	if len(spec.Enum) > 0 {
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("枚举项必须是字符串")
		}
		for _, allowed := range spec.Enum {
			if s == allowed {
				return nil
			}
		}
		return fmt.Errorf("必须是 %s 之一", strings.Join(spec.Enum, "/"))
	}
	return nil
}

func normalizeValue(spec Spec, value any) (any, error) {
	switch spec.Type {
	case TypeString, TypeSecret:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("必须是字符串")
		}
		return s, nil
	case TypeInt:
		n, err := toInt(value)
		if err != nil {
			return nil, err
		}
		return n, nil
	case TypeBool:
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("必须是布尔值")
		}
		return b, nil
	case TypeDuration:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("必须是时长字符串")
		}
		if _, err := time.ParseDuration(s); err != nil {
			return nil, err
		}
		return s, nil
	case TypeJSON:
		if raw, ok := value.(json.RawMessage); ok {
			return raw, nil
		}
		if s, ok := value.(string); ok {
			return json.RawMessage(s), nil
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	default:
		return value, nil
	}
}

func toInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("必须是整数")
		}
		return int(v), nil
	case json.Number:
		n, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, fmt.Errorf("必须是整数")
		}
		return n, nil
	default:
		return 0, fmt.Errorf("必须是整数")
	}
}

func maskSecret(value any) any {
	s, ok := value.(string)
	if !ok || s == "" {
		return "******"
	}
	if len([]rune(s)) <= 4 {
		return "******"
	}
	r := []rune(s)
	return "******" + string(r[len(r)-4:])
}
