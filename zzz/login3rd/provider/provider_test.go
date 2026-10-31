package provider

import "testing"

func TestHasAnyCookie(t *testing.T) {
	cookies := []Cookie{{Name: "pt_key", Value: "k"}, {Name: "pt_pin", Value: "p"}}
	if !hasAnyCookie(cookies, "pt_key") {
		t.Error("应识别 pt_key")
	}
	if hasAnyCookie(cookies, "unb") {
		t.Error("不应识别不存在的 unb")
	}
	if hasAnyCookie([]Cookie{{Name: "pt_key", Value: ""}}, "pt_key") {
		t.Error("空值 Cookie 不应识别")
	}
}

func TestIsLoginURL(t *testing.T) {
	cases := map[string]bool{
		"https://login.taobao.com/member/login.jhtml":         true,
		"https://login.m.taobao.com/login.htm?redirectURL=x":  true,
		"https://mobile.yangkeduo.com/login.html?from=goods":  true,
		"https://item.taobao.com/item.htm?id=1":               false,
		"https://mobile.yangkeduo.com/goods1.html?goods_id=1": false,
	}
	for raw, want := range cases {
		if got := isLoginURL(raw); got != want {
			t.Errorf("isLoginURL(%q)=%v want %v", raw, got, want)
		}
	}
}
