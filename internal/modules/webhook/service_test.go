package webhook

import "testing"

func TestIsValidType(t *testing.T) {
	for _, typ := range []string{"feishu", "dingtalk", "wecom", "github", "generic"} {
		if !isValidType(typ) {
			t.Errorf("%s 应为合法类型", typ)
		}
	}
	for _, typ := range []string{"", "slack", "FEISHU", "http"} {
		if isValidType(typ) {
			t.Errorf("%s 应为非法类型", typ)
		}
	}
}

func TestParseInt64(t *testing.T) {
	if got, err := parseInt64("42"); err != nil || got != 42 {
		t.Errorf("parseInt64(42)=%d / %v", got, err)
	}
	if _, err := parseInt64("not-a-number"); err == nil {
		t.Error("非法数字应返回错误")
	}
}

func TestSha256Hex(t *testing.T) {
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := sha256Hex("abc"); got != want {
		t.Errorf("sha256Hex(abc)=%s", got)
	}
}

func TestPtrHelpers(t *testing.T) {
	if got := intPtr(7); got == nil || *got != 7 {
		t.Error("intPtr 异常")
	}
	if got := strPtr("x"); got == nil || *got != "x" {
		t.Error("strPtr 异常")
	}
}
