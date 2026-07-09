package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testModule(t *testing.T) *Module {
	m := &Module{}
	err := m.Init(&config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "test-secret",
			JWTExpire: "24h",
		},
	})
	require.NoError(t, err)
	return m
}

func setupRouter(m *Module) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m.RegisterRoutes(r.Group("/api/v1/auth"))
	return r
}

func loginAndGetToken(t *testing.T, m *Module, r *gin.Engine, username, password string) string {
	body := `{"username":"` + username + `","password":"` + password + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var regResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &regResp)
	return regResp.Data.AccessToken
}

func TestRegister(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"username":"newuser","password":"pass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
}

func TestRegisterDuplicate(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"username":"dupuser","password":"pass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 1006, resp.Code)
}

func TestLogin(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"username":"loginuser","password":"pass456"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
	assert.NotEmpty(t, resp.Data["access_token"])
	assert.NotEmpty(t, resp.Data["refresh_token"])
}

func TestLoginWrongPassword(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"username":"nobody","password":"wrong"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var resp struct {
		Code int `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 1003, resp.Code)
}

func TestRefresh(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"username":"refuser","password":"pass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	var regResp struct {
		Data struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &regResp)

	refreshBody := `{"refresh_token":"` + regResp.Data.RefreshToken + `"}`
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/refresh", strings.NewReader(refreshBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var refreshResp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &refreshResp)
	assert.Equal(t, 0, refreshResp.Code)
	assert.NotEmpty(t, refreshResp.Data["access_token"])
}

func TestRefreshInvalid(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	body := `{"refresh_token":"bad-token"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMe_Unauthenticated(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMe_Authenticated(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "meuser", "pass123")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var meResp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &meResp)
	assert.Equal(t, "meuser", meResp.Data["username"])
	assert.Contains(t, meResp.Data, "roles")
}

func TestLogout(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "logoutuser", "pass123")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegisterMultipleProviders(t *testing.T) {
	m := testModule(t)
	m.RegisterProvider(&mockProvider{name: "qq"})
	m.RegisterProvider(&mockProvider{name: "apple"})
	m.RegisterProvider(&mockProvider{name: "huawei"})
	m.RegisterProvider(&mockProvider{name: "honor"})
	r := setupRouter(m)

	for _, name := range []string{"qq", "apple", "huawei", "honor"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/auth/"+name+"/url", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "provider: "+name)
	}
}

func TestOAuth2URL(t *testing.T) {
	m := testModule(t)
	m.RegisterProvider(&mockProvider{name: "mock_test"})
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/mock_test/url", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			Provider string `json:"provider"`
			AuthURL  string `json:"auth_url"`
			State    string `json:"state"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "mock_test", resp.Data.Provider)
	assert.NotEmpty(t, resp.Data.AuthURL)
}

func TestOAuth2Callback(t *testing.T) {
	m := testModule(t)
	m.RegisterProvider(&mockProvider{name: "mock_cb"})
	r := setupRouter(m)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/mock_cb/callback?code=testcode", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp.Data.AccessToken)
}

func TestBindUnbind(t *testing.T) {
	m := testModule(t)
	m.RegisterProvider(&mockProvider{name: "mock_bind"})
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "binduser", "pass123")

	body := `{"code":"testcode"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/mock_bind/bind", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/auth/mock_bind/unbind", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAccounts(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "acctuser", "pass123")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAssignRole_Forbidden(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "roleuser", "pass123")

	body := `{"user_id":1,"role":"admin"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/roles/assign", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

type mockProvider struct {
	name string
}

func (p *mockProvider) Name() string                { return p.name }
func (p *mockProvider) AuthURL(state string) string { return "https://mock/" + state }
func (p *mockProvider) Exchange(ctx context.Context, code string) (string, error) {
	return "mock-token", nil
}
func (p *mockProvider) GetUserInfo(ctx context.Context, token string) (*auth.SocialUser, error) {
	return &auth.SocialUser{
		OpenID:   "mock-openid",
		UnionID:  "mock-unionid",
		Nickname: "MockUser",
	}, nil
}
