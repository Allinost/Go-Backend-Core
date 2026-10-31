package login3rd

import (
	"testing"
	"time"
)

func TestSessionPlatformCandidates(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"jd", []string{"jd", "qixian", "jdm"}},
		{"qixian", []string{"qixian", "jd", "jdm"}},
		{"jdm", []string{"jdm", "jd", "qixian"}},
		{"taobao", []string{"taobao", "tmall"}},
		{"tmall", []string{"tmall", "taobao"}},
		{"pdd", []string{"pdd"}},
		{"", nil},
	}
	for _, c := range cases {
		got := SessionPlatformCandidates(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%q: candidates=%v want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: candidates=%v want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestAccountSessionUsable(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	valid := &Account{
		Status:           "active",
		VerifiedStatus:   "valid",
		SessionExpiresAt: &future,
		SessionData:      &SessionData{Cookies: []SessionCookie{{Name: "pt_key", Value: "x"}}},
	}
	if !accountSessionUsable(valid) {
		t.Error("有效会话应可用")
	}

	expired := *valid
	expired.SessionExpiresAt = &past
	if accountSessionUsable(&expired) {
		t.Error("过期时间已过不应可用")
	}

	disabled := *valid
	disabled.Status = "disabled"
	if accountSessionUsable(&disabled) {
		t.Error("disabled 账号不应可用")
	}

	empty := &Account{Status: "active", VerifiedStatus: "valid"}
	if accountSessionUsable(empty) {
		t.Error("无 Cookie 账号不应可用")
	}
}
