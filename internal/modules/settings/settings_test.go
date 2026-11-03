package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadDefaultsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "settings.json"))
	defaults := Data{PublicBaseURL: "https://example.com", FrontendTitle: "Core", FrontendAPIBase: "/api/v1"}

	got, err := store.Load(defaults)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got != defaults {
		t.Errorf("文件不存在时应返回默认值：%+v", got)
	}
	if store.Exists() {
		t.Error("文件不应存在")
	}

	want := Data{
		PublicBaseURL: "https://example.com:8443",
		FrontendTitle: "控制台",
		N8nURL:        "http://n8n:5678",
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if !store.Exists() {
		t.Error("保存后文件应存在")
	}
	got, err = store.Load(defaults)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got.PublicBaseURL != want.PublicBaseURL || got.N8nURL != want.N8nURL {
		t.Errorf("读取结果不一致：%+v", got)
	}
}

func TestNormalizeData(t *testing.T) {
	got, err := normalizeData(Data{
		PublicBaseURL:   " https://example.com/ ",
		FrontendAPIBase: " /api/v1/ ",
		N8nURL:          "http://n8n:5678/",
	})
	if err != nil {
		t.Fatalf("normalizeData 失败: %v", err)
	}
	if got.PublicBaseURL != "https://example.com" {
		t.Errorf("公开地址未归一化：%q", got.PublicBaseURL)
	}
	if got.FrontendAPIBase != "/api/v1" {
		t.Errorf("API 前缀未归一化：%q", got.FrontendAPIBase)
	}
	if got.N8nURL != "http://n8n:5678" {
		t.Errorf("n8n 地址未去尾斜杠：%q", got.N8nURL)
	}
	if got.FrontendTitle != "" {
		t.Errorf("空标题应保持空，实际 %q", got.FrontendTitle)
	}
}

func TestNormalizeDataRejectsInvalid(t *testing.T) {
	cases := []Data{
		{PublicBaseURL: "ftp://example.com"},
		{PublicBaseURL: "not-a-url"},
		{FrontendAPIBase: "ftp://example.com"},
		{N8nURL: "javascript:alert(1)"},
	}
	for i, c := range cases {
		if _, err := normalizeData(c); err == nil {
			t.Errorf("用例 %d 应校验失败：%+v", i, c)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	if got, err := normalizeURL("", true, "http"); err != nil || got != "" {
		t.Errorf("空值应允许：%q / %v", got, err)
	}
	if _, err := normalizeURL("", false, "http"); err == nil {
		t.Error("不允许空值时应收敛")
	}
	if got, err := normalizeURL(" wss://example.com/ ", true, "ws", "wss"); err != nil || got != "wss://example.com" {
		t.Errorf("wss 地址异常：%q / %v", got, err)
	}
}

func TestSettingsFilePathEnv(t *testing.T) {
	old, had := os.LookupEnv("APP_SETTINGS_FILE")
	defer func() {
		if had {
			os.Setenv("APP_SETTINGS_FILE", old)
		} else {
			os.Unsetenv("APP_SETTINGS_FILE")
		}
	}()
	os.Setenv("APP_SETTINGS_FILE", "custom/settings.json")
	if got := settingsFilePath(); got != "custom/settings.json" {
		t.Errorf("自定义路径未生效：%s", got)
	}
}
