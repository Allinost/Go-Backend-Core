package template

import (
	"bytes"
	"fmt"
	htemplate "html/template"
	"io/fs"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Engine 模板引擎
type Engine struct {
	mu        sync.RWMutex
	fsys      fs.FS                          // 文件系统
	textCache map[string]*template.Template  // 文本模板缓存
	htmlCache map[string]*htemplate.Template // HTML 模板缓存
	funcs     template.FuncMap               // 模板函数映射
}

// NewEngineFS 基于文件系统创建模板引擎
func NewEngineFS(fsys fs.FS) *Engine {
	return &Engine{
		fsys:      fsys,
		textCache: make(map[string]*template.Template),
		htmlCache: make(map[string]*htemplate.Template),
		funcs:     make(template.FuncMap),
	}
}

// AddFunc 向引擎注册全局模板函数
func (e *Engine) AddFunc(name string, fn any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.funcs[name] = fn
}

// RenderText 渲染文本模板（懒加载并缓存）
func (e *Engine) RenderText(name string, data any) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.textCache[name]
	if !ok {
		var err error
		t, err = e.loadText(name)
		if err != nil {
			return "", err
		}
		e.textCache[name] = t
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: render text %s failed: %w", name, err)
	}
	return buf.String(), nil
}

// RenderHTML 渲染 HTML 模板（自动转义，懒加载并缓存）
// RenderHTML 渲染 HTML 模板（自动转义，懒加载并缓存）
func (e *Engine) RenderHTML(name string, data any) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.htmlCache[name]
	if !ok {
		var err error
		t, err = e.loadHTML(name)
		if err != nil {
			return "", err
		}
		e.htmlCache[name] = t
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: render html %s failed: %w", name, err)
	}
	return buf.String(), nil
}

// loadText 从文件系统加载并解析文本模板
func (e *Engine) loadText(name string) (*template.Template, error) {
	content, err := e.readFile(name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Funcs(e.funcs).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("template: parse text %s failed: %w", name, err)
	}
	return t, nil
}

// loadHTML 从文件系统加载并解析 HTML 模板
func (e *Engine) loadHTML(name string) (*htemplate.Template, error) {
	content, err := e.readFile(name)
	if err != nil {
		return nil, err
	}
	t, err := htemplate.New(name).Funcs(e.funcs).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("template: parse html %s failed: %w", name, err)
	}
	return t, nil
}

// readFile 从文件系统读取文件内容
func (e *Engine) readFile(name string) (string, error) {
	if e.fsys == nil {
		return "", fmt.Errorf("template: no filesystem set, use NewEngineFS")
	}
	data, err := fs.ReadFile(e.fsys, name)
	if err != nil {
		return "", fmt.Errorf("template: read %s failed: %w", name, err)
	}
	return string(data), nil
}

// ClearCache 清除模板缓存
func (e *Engine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.textCache = make(map[string]*template.Template)
	e.htmlCache = make(map[string]*htemplate.Template)
}

// RenderTextString 渲染内联文本模板字符串（不依赖文件系统）
func (e *Engine) RenderTextString(tpl string, data any) (string, error) {
	t, err := template.New("inline").Funcs(e.funcs).Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("template: parse inline text failed: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: render inline text failed: %w", err)
	}
	return buf.String(), nil
}

// RenderHTMLString 渲染内联 HTML 模板字符串（不依赖文件系统）
func (e *Engine) RenderHTMLString(tpl string, data any) (string, error) {
	t, err := htemplate.New("inline").Funcs(e.funcs).Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("template: parse inline html failed: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template: render inline html failed: %w", err)
	}
	return buf.String(), nil
}

// Reset 重置模板引擎的文件系统和缓存
func (e *Engine) Reset(fsys fs.FS) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fsys = fsys
	e.textCache = make(map[string]*template.Template)
	e.htmlCache = make(map[string]*htemplate.Template)
}

// AddFunc 注册全局模板函数（包级函数）
func AddFunc(name string, fn any) {
	global.AddFunc(name, fn)
}

// RenderText 使用全局引擎渲染文本模板
func RenderText(name string, data any) (string, error) {
	return global.RenderText(name, data)
}

// RenderHTML 使用全局引擎渲染 HTML 模板
func RenderHTML(name string, data any) (string, error) {
	return global.RenderHTML(name, data)
}

// RenderTextString 使用全局引擎渲染内联文本模板
func RenderTextString(tpl string, data any) (string, error) {
	return global.RenderTextString(tpl, data)
}

// RenderHTMLString 使用全局引擎渲染内联 HTML 模板
func RenderHTMLString(tpl string, data any) (string, error) {
	return global.RenderHTMLString(tpl, data)
}

// global 全局默认模板引擎
var global *Engine

// InitGlobal 初始化全局模板引擎
func InitGlobal(fsys fs.FS) {
	global = NewEngineFS(fsys)
	global.AddFunc("lower", strings.ToLower)
	global.AddFunc("upper", strings.ToUpper)
	global.AddFunc("title", strings.Title)
	global.AddFunc("default", func(def, val any) any {
		if val == nil || val == "" {
			return def
		}
		return val
	})
	global.AddFunc("now", time.Now)
	global.AddFunc("formatTime", func(t time.Time, layout string) string {
		return t.Format(layout)
	})
}

// ListTemplates 列出缓存中的所有模板名称
func (e *Engine) ListTemplates() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.textCache)+len(e.htmlCache))
	for name := range e.textCache {
		names = append(names, "text:"+name)
	}
	for name := range e.htmlCache {
		names = append(names, "html:"+name)
	}
	return names
}

// TemplateExists检查模板是否在缓存中存在
func (e *Engine) TemplateExists(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if _, ok := e.textCache[name]; ok {
		return true
	}
	if _, ok := e.htmlCache[name]; ok {
		return true
	}
	return false
}

func init() {
	InitGlobal(nil)
}
