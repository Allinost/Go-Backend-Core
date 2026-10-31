package browser

import (
	"testing"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
)

func TestIsLoginPageURL(t *testing.T) {
	cases := map[string]bool{
		"https://login.taobao.com/member/login.jhtml":         true,
		"https://mobile.yangkeduo.com/login.html?from=x":      true,
		"https://passport.jd.com/new/login.aspx":              true,
		"https://www.jd.com":                                  false,
		"https://mobile.yangkeduo.com/goods1.html?goods_id=1": false,
	}
	for raw, want := range cases {
		if got := isLoginPageURL(raw); got != want {
			t.Errorf("isLoginPageURL(%q)=%v want %v", raw, got, want)
		}
	}
}

func TestHasAnyNamedCookie(t *testing.T) {
	cookies := []*provider.Cookie{
		{Name: "foo", Value: "1"},
		{Name: "pt_key", Value: "key"},
	}
	if !hasAnyNamedCookie(cookies, knownAuthCookies) {
		t.Error("应识别 pt_key 为登录 Cookie")
	}
	if hasAnyNamedCookie([]*provider.Cookie{{Name: "foo", Value: "1"}}, knownAuthCookies) {
		t.Error("普通 Cookie 不应判定为登录成功")
	}
	if hasAnyNamedCookie([]*provider.Cookie{{Name: "pt_key", Value: ""}}, knownAuthCookies) {
		t.Error("空值 Cookie 不应判定为登录成功")
	}
}
