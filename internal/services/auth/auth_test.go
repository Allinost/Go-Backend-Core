package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testService(t *testing.T) *Service {
	return NewService(Config{
		Secret:        "test-secret-key",
		AccessExpire:  time.Hour,
		RefreshExpire: 24 * time.Hour,
	})
}

func TestGenerateAndValidateToken(t *testing.T) {
	svc := testService(t)
	pair, err := svc.GenerateTokenPair(1, "alice")
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, TokenTypeAccess, claims.TokenType)
}

func TestValidateRefreshToken(t *testing.T) {
	svc := testService(t)
	pair, _ := svc.GenerateTokenPair(1, "alice")

	claims, err := svc.ValidateRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)
	assert.Equal(t, TokenTypeRefresh, claims.TokenType)
}

func TestTokenTypeMismatch(t *testing.T) {
	svc := testService(t)
	pair, _ := svc.GenerateTokenPair(1, "alice")

	_, err := svc.ValidateAccessToken(pair.RefreshToken)
	assert.Error(t, err)

	_, err = svc.ValidateRefreshToken(pair.AccessToken)
	assert.Error(t, err)
}

func TestInvalidToken(t *testing.T) {
	svc := testService(t)
	_, err := svc.ValidateToken("invalid-token")
	assert.Error(t, err)
}

func TestWrongSecret(t *testing.T) {
	svc1 := testService(t)
	svc2 := NewService(Config{Secret: "different-secret", AccessExpire: time.Hour, RefreshExpire: 24 * time.Hour})

	pair, err := svc1.GenerateTokenPair(1, "alice")
	require.NoError(t, err)

	_, err = svc2.ValidateAccessToken(pair.AccessToken)
	assert.Error(t, err)
}

func TestExpiredToken(t *testing.T) {
	svc := NewService(Config{
		Secret:        "test-secret",
		AccessExpire:  1 * time.Millisecond,
		RefreshExpire: 1 * time.Millisecond,
	})

	pair, err := svc.GenerateTokenPair(1, "alice")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateAccessToken(pair.AccessToken)
	assert.Error(t, err)
}

func TestRefreshAccessToken(t *testing.T) {
	svc := testService(t)
	pair, _ := svc.GenerateTokenPair(1, "alice")

	time.Sleep(2 * time.Second)

	newPair, err := svc.RefreshAccessToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, pair.AccessToken, newPair.AccessToken)
	assert.NotEqual(t, pair.RefreshToken, newPair.RefreshToken)

	claims, err := svc.ValidateAccessToken(newPair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(1), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
}

func TestRefreshInvalidToken(t *testing.T) {
	svc := testService(t)
	_, err := svc.RefreshAccessToken("bad-token")
	assert.Error(t, err)
}

func TestInMemoryBlacklist(t *testing.T) {
	b := NewInMemoryBlacklist()
	ctx := context.Background()

	revoked, err := b.IsRevoked(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, revoked)

	err = b.Revoke(ctx, "token-1", time.Now().Add(time.Hour))
	require.NoError(t, err)

	revoked, err = b.IsRevoked(ctx, "token-1")
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestInMemoryBlacklist_Expired(t *testing.T) {
	b := NewInMemoryBlacklist()
	ctx := context.Background()

	_ = b.Revoke(ctx, "expired", time.Now().Add(-time.Hour))
	revoked, err := b.IsRevoked(ctx, "expired")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestTokenPairResponse(t *testing.T) {
	svc := testService(t)
	pair, err := svc.GenerateTokenPair(1, "alice")
	require.NoError(t, err)

	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, uint(1), pair.UserID)
	assert.Equal(t, "alice", pair.Username)
	assert.Greater(t, pair.ExpiresIn, int64(0))
}

func TestInMemoryUserStore_CreateAndFind(t *testing.T) {
	store := NewInMemoryUserStore()
	user, err := store.CreateUser(RegisterRequest{
		Username: "alice",
		Password: "secret123",
	})
	require.NoError(t, err)
	assert.Greater(t, user.ID, uint(0))
	assert.Equal(t, "alice", user.Username)

	found, err := store.FindByUsername("alice")
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}

func TestInMemoryUserStore_Duplicate(t *testing.T) {
	store := NewInMemoryUserStore()
	_, err := store.CreateUser(RegisterRequest{Username: "dup", Password: "pass123"})
	require.NoError(t, err)

	_, err = store.CreateUser(RegisterRequest{Username: "dup", Password: "pass456"})
	assert.Error(t, err)
}

func TestInMemoryUserStore_VerifyPassword(t *testing.T) {
	store := NewInMemoryUserStore()
	_, err := store.CreateUser(RegisterRequest{Username: "bob", Password: "correct-pass"})
	require.NoError(t, err)

	user, err := store.VerifyPassword("bob", "correct-pass")
	require.NoError(t, err)
	assert.Equal(t, "bob", user.Username)

	_, err = store.VerifyPassword("bob", "wrong-pass")
	assert.Error(t, err)

	_, err = store.VerifyPassword("nobody", "pass")
	assert.Error(t, err)
}

func TestInMemoryUserStore_FindByID(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "id-test", Password: "pass123"})

	found, err := store.FindByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, "id-test", found.Username)

	_, err = store.FindByID(9999)
	assert.Error(t, err)
}

func TestQQProvider_NameAndURL(t *testing.T) {
	p := NewQQProvider("appid", "appkey", "https://example.com/callback")
	assert.Equal(t, "qq", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "graph.qq.com")
	assert.Contains(t, p.AuthURL("state123"), "state123")
}

func TestAppleProvider_NameAndURL(t *testing.T) {
	p := NewAppleProvider("client", "team", "key", "pk", "https://example.com/callback")
	assert.Equal(t, "apple", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "appleid.apple.com")
}

func TestHuaweiProvider_NameAndURL(t *testing.T) {
	p := NewHuaweiProvider("client", "secret", "https://example.com/callback")
	assert.Equal(t, "huawei", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "huawei.com")
}

func TestHonorProvider_NameAndURL(t *testing.T) {
	p := NewHonorProvider("client", "secret", "https://example.com/callback")
	assert.Equal(t, "honor", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "honor.com")
}

func TestWechatProvider_NameAndURL(t *testing.T) {
	p := NewWechatProvider("appid", "secret", "https://example.com/callback")
	assert.Equal(t, "wechat", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "weixin.qq.com")
}

func TestFeishuProvider_NameAndURL(t *testing.T) {
	p := NewFeishuProvider("appid", "secret", "https://example.com/callback")
	assert.Equal(t, "feishu", p.Name())
	assert.Contains(t, p.AuthURL("state123"), "feishu.cn")
}

func TestInMemoryRBACStore_Defaults(t *testing.T) {
	s := NewInMemoryRBACStore()
	err := s.EnsureDefaultPermissions()
	require.NoError(t, err)

	roles, err := s.GetRoles(1)
	require.NoError(t, err)
	assert.Equal(t, []string{"user"}, roles)

	perms, err := s.GetPermissions([]string{"admin"})
	require.NoError(t, err)
	assert.Equal(t, 1, len(perms))
	assert.Equal(t, "*:*", perms[0].Name)
}

func TestInMemoryRBACStore_AssignAndCheck(t *testing.T) {
	s := NewInMemoryRBACStore()
	_ = s.EnsureDefaultPermissions()

	err := s.AssignRole(1, "admin")
	require.NoError(t, err)

	roles, _ := s.GetRoles(1)
	assert.Equal(t, []string{"admin"}, roles)

	perms, _ := s.GetPermissions([]string{"admin"})
	assert.Equal(t, "*:*", perms[0].Name)
}

func TestInMemoryRBACStore_InvalidRole(t *testing.T) {
	s := NewInMemoryRBACStore()
	err := s.AssignRole(1, "superadmin")
	assert.Error(t, err)
}

func TestRBACService_HasPermission(t *testing.T) {
	store := NewInMemoryRBACStore()
	_ = store.EnsureDefaultPermissions()
	svc := NewRBACService(store)

	assert.True(t, svc.HasPermission(1, "task", "read"))
	assert.True(t, svc.HasPermission(1, "task", "create"))
	assert.True(t, svc.HasPermission(1, "task", "delete"))
	assert.False(t, svc.HasPermission(1, "user", "admin"))

	_ = svc.AssignRole(1, "admin")
	assert.True(t, svc.HasPermission(1, "*", "*"))
	assert.True(t, svc.HasPermission(1, "anything", "anything"))
}
