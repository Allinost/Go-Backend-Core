package login3rd

import "testing"

func TestPublicBaseURLDefaultEmpty(t *testing.T) {
	original := publicBaseURL
	defer func() { publicBaseURL = original }()

	publicBaseURL = ""
	if got := getHost(); got != "" {
		t.Fatalf("仓库内不应保留默认部署地址，实际 %q", got)
	}
}

func TestSetPublicBaseURL(t *testing.T) {
	original := publicBaseURL
	defer func() { publicBaseURL = original }()

	SetPublicBaseURL("  https://example.com:8443/  ")
	if got := getHost(); got != "https://example.com:8443" {
		t.Fatalf("getHost() = %q", got)
	}

	SetPublicBaseURL("   ")
	if got := getHost(); got != "https://example.com:8443" {
		t.Fatalf("空配置不应覆盖现有地址: %q", got)
	}
}
