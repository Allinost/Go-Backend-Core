package collector

import (
	"errors"
	"testing"
)

func TestParseJDUnionResponse(t *testing.T) {
	body := []byte(`{"jd_union_open_goods_promotiongoodsinfo_query_responce":{"code":"0","queryResult":"{\"code\":200,\"data\":[{\"skuId\":\"100086179820\",\"skuName\":\"华硕 B550M 重炮手 + 5700X\",\"priceInfo\":{\"lowestPrice\":1805.26,\"price\":2127,\"marketPrice\":2127}}]}"}}`)
	r, err := parseJDUnionResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.CurrentPrice != 1805.26 || r.OriginalPrice != 2127 || r.Title == "" {
		t.Errorf("unexpected result: %+v", r)
	}
}

func TestJDUnionSignStable(t *testing.T) {
	params := map[string]string{
		"method":            "jd.union.open.goods.promotiongoodsinfo.query",
		"app_key":           "key",
		"timestamp":         "2026-10-31 12:00:00",
		"format":            "json",
		"v":                 "1.0",
		"sign_method":       "md5",
		"360buy_param_json": `{"skuIds":"1"}`,
	}
	a := jdUnionSign(params, "secret")
	b := jdUnionSign(params, "secret")
	if a == "" || a != b {
		t.Errorf("sign not stable: %q %q", a, b)
	}
	if c := jdUnionSign(params, "other"); c == a {
		t.Error("different secret should produce different sign")
	}
}

func TestIsRateLimitError(t *testing.T) {
	for _, msg := range []string{"京东访问频率限制（PC频控页）", "pc-frequent reason=403", "HTTP 403"} {
		if !IsRateLimitError(errors.New(msg)) {
			t.Errorf("%q should be rate limit", msg)
		}
	}
	if IsRateLimitError(errors.New("需要登录")) {
		t.Error("login error should not be rate limit")
	}
}
