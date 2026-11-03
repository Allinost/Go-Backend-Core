package login3rd

import "testing"

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
