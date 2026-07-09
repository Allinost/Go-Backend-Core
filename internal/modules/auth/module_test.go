package auth

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestUpdateProfile(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "profi1e", "pass123")

	body := `{"nickname":"NewNick","email":"new@test.com"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/auth/profile", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			Nickname string `json:"nickname"`
			Email    string `json:"email"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "NewNick", resp.Data.Nickname)
	assert.Equal(t, "new@test.com", resp.Data.Email)
}

func TestChangePassword(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "changep1", "oldpass")

	body := `{"old_password":"oldpass","new_password":"newpass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/auth/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestChangePassword_WrongOld(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "changep2", "realpass")

	body := `{"old_password":"wrongpass","new_password":"newpass123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/auth/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListUsers_Forbidden(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "normaluser", "pass123")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminListUsers(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "adminuser", "pass123")
	m.rbac.AssignRole(1, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			Data  []any  `json:"data"`
			Total int    `json:"total"`
			Page  int    `json:"page"`
			Size  int    `json:"size"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.GreaterOrEqual(t, resp.Data.Total, 0)
	assert.Equal(t, 1, resp.Data.Page)
}

func TestAdminGetUser(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "admin2", "pass123")
	m.rbac.AssignRole(1, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminUpdateUser(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "admin3", "pass123")
	m.rbac.AssignRole(1, "admin")

	body := `{"nickname":"AdminUpdated"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/auth/users/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminDeleteUser(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "admin4", "pass123")
	m.rbac.AssignRole(1, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

func TestCreateAPIKey(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "keyuser1", "pass123")

	body := `{"name":"my-key","scopes":"task:read"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			ID        uint   `json:"id"`
			RawKey    string `json:"raw_key"`
			Name      string `json:"name"`
			KeyPrefix string `json:"key_prefix"`
			Scopes    string `json:"scopes"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp.Data.RawKey)
	assert.Equal(t, "my-key", resp.Data.Name)
	assert.Equal(t, "task:read", resp.Data.Scopes)
}

func TestListAPIKeys_Own(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "keyuser2", "pass123")

	_, _ = m.apiKeys.GenerateKey("k1", 1, "*:*", nil)
	_, _ = m.apiKeys.GenerateKey("k2", 1, "*:*", nil)
	_, _ = m.apiKeys.GenerateKey("k3", 2, "*:*", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListAPIKeys_Admin(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "keyadmin", "pass123")
	m.rbac.AssignRole(1, "admin")

	_, _ = m.apiKeys.GenerateKey("k1", 1, "*:*", nil)
	_, _ = m.apiKeys.GenerateKey("k2", 2, "*:*", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data struct {
			Data  []any `json:"data"`
			Total int   `json:"total"`
			Page  int   `json:"page"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 2, resp.Data.Total)
}

func TestDeleteAPIKey(t *testing.T) {
	m := testModule(t)
	r := setupRouter(m)
	token := loginAndGetToken(t, m, r, "keydel", "pass123")

	key, _ := m.apiKeys.GenerateKey("del-me", 1, "*:*", nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/auth/api-keys/"+fmt.Sprint(key.ID), nil)
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
