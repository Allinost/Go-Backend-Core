package servicenav

import "testing"

func TestSplitNavURL(t *testing.T) {
	cases := []struct {
		raw      string
		protocol string
		address  string
	}{
		{"https://api.example.com", "https", "api.example.com"},
		{"http://api.example.com:8080/health?x=1", "http", "api.example.com:8080/health?x=1"},
		{"api.example.com", "http", "api.example.com"},
		{"", "http", ""},
	}
	for _, c := range cases {
		protocol, address := splitNavURL(c.raw)
		if protocol != c.protocol || address != c.address {
			t.Errorf("splitNavURL(%q)=(%q,%q)，期望 (%q,%q)", c.raw, protocol, address, c.protocol, c.address)
		}
	}
}

func TestBuildImportName(t *testing.T) {
	c := ImportCandidate{
		ProjectName: "本地容器栈",
		Purpose:     "控制台",
		Exposure:    "extranet",
	}
	if got, want := buildImportName(c), "本地容器栈 · 控制台（公网）"; got != want {
		t.Errorf("buildImportName=%q，期望 %q", got, want)
	}
	c.Purpose = ""
	c.ListenPort = 8080
	c.Exposure = "intranet"
	if got, want := buildImportName(c), "本地容器栈 · 端口 8080（内网）"; got != want {
		t.Errorf("buildImportName=%q，期望 %q", got, want)
	}
}

func TestExposureLabel(t *testing.T) {
	cases := map[string]string{
		"extranet":  "公网",
		"intranet":  "内网",
		"proxy_net": "反代网络",
		"none":      "不暴露",
		"":          "不暴露",
	}
	for in, want := range cases {
		if got := exposureLabel(in); got != want {
			t.Errorf("exposureLabel(%q)=%q，期望 %q", in, got, want)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("项目开发管理", 3); got != "项目开" {
		t.Errorf("truncateRunes 中文异常：%q", got)
	}
	if got := truncateRunes("abc", 5); got != "abc" {
		t.Errorf("truncateRunes 短串异常：%q", got)
	}
}

func TestImportableProjectTypes(t *testing.T) {
	want := map[string]bool{"base": true, "frontend_base": true, "container_project": true, "template": true}
	if len(importableProjectTypes) != len(want) {
		t.Fatalf("可导入类型应为 %d 种，实际 %d", len(want), len(importableProjectTypes))
	}
	for _, typ := range importableProjectTypes {
		if !want[typ] {
			t.Errorf("类型 %s 不应在导入范围", typ)
		}
	}
	for _, typ := range []string{"backend_module", "frontend_module"} {
		for _, allowed := range importableProjectTypes {
			if typ == allowed {
				t.Errorf("模块类型 %s 不应允许导入", typ)
			}
		}
	}
}
