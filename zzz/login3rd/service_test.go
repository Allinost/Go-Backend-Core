package login3rd

import (
	"bytes"
	"testing"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
)

func testKey() []byte {
	return bytes.Repeat([]byte{0x2a}, 32)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey()
	plain := []byte("p@ssw0rd-中文-🔐")
	enc, err := encrypt(plain, key)
	if err != nil {
		t.Fatalf("加密失败：%v", err)
	}
	if enc == string(plain) || len(enc) == 0 {
		t.Fatal("密文异常")
	}
	dec, err := decrypt(enc, key)
	if err != nil {
		t.Fatalf("解密失败：%v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Errorf("解密结果不一致：%q", dec)
	}
}

func TestDecryptRejectsTamperedOrWrongKey(t *testing.T) {
	key := testKey()
	enc, err := encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatal(err)
	}

	tampered := enc[:len(enc)-1] + "0"
	if tampered == enc {
		tampered = enc[:len(enc)-1] + "1"
	}
	if _, err := decrypt(tampered, key); err == nil {
		t.Error("篡改密文应解密失败")
	}

	wrongKey := bytes.Repeat([]byte{0x11}, 32)
	if _, err := decrypt(enc, wrongKey); err == nil {
		t.Error("错误密钥应解密失败")
	}
	if _, err := decrypt("not-hex", key); err == nil {
		t.Error("非法 hex 应解密失败")
	}
	if _, err := decrypt("00", key); err == nil {
		t.Error("过短密文应解密失败")
	}
}

func TestSessionConversion(t *testing.T) {
	if toModelSession(nil) != nil || toProviderSession(nil) != nil {
		t.Error("nil 会话应返回 nil")
	}

	expires := int64(1760000000)
	src := &provider.SessionData{
		Cookies: []provider.Cookie{{
			Name: "sid", Value: "v", Domain: ".example.com", Path: "/",
			Secure: true, HTTPOnly: true, Expires: expires,
		}},
		Headers:   map[string]string{"UA": "test"},
		Extra:     map[string]any{"k": "v"},
		ExpiresAt: expires + 3600,
	}

	model := toModelSession(src)
	if model == nil || len(model.Cookies) != 1 {
		t.Fatalf("会话转换失败：%+v", model)
	}
	if model.Cookies[0].Expires.Unix() != expires {
		t.Errorf("Cookie 过期时间丢失：%v", model.Cookies[0].Expires)
	}
	if model.ExpiresAt == nil || model.ExpiresAt.Unix() != expires+3600 {
		t.Errorf("会话过期时间丢失：%v", model.ExpiresAt)
	}

	back := toProviderSession(model)
	if back == nil || len(back.Cookies) != 1 {
		t.Fatalf("反向转换失败：%+v", back)
	}
	if back.Cookies[0].Expires != expires {
		t.Errorf("反向 Cookie 过期时间不一致：%d", back.Cookies[0].Expires)
	}
	if back.Headers["UA"] != "test" {
		t.Error("Headers 丢失")
	}
}

func TestGetDecryptedPassword(t *testing.T) {
	key := testKey()
	enc, err := encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{masterKey: key}
	if got := svc.GetDecryptedPassword(&Account{EncryptedPwd: enc}); got != "secret" {
		t.Errorf("密码解密异常：%q", got)
	}
	if got := svc.GetDecryptedPassword(&Account{}); got != "" {
		t.Errorf("空密码应返回空串，实际 %q", got)
	}
	if got := svc.GetDecryptedPassword(&Account{EncryptedPwd: "bad"}); got != "" {
		t.Errorf("非法密文应返回空串，实际 %q", got)
	}
}

func TestListAccountsReqNormalize(t *testing.T) {
	r := &ListAccountsReq{Page: 0, PageSize: 500}
	r.Normalize()
	if r.Page != 1 || r.PageSize != 20 {
		t.Errorf("分页归一化异常：%+v", r)
	}
	if got := r.Offset(); got != 0 {
		t.Errorf("Offset 应为 0，实际 %d", got)
	}
}
