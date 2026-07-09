package auth

import (
	"context"
	"fmt"
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

func TestInMemoryUserStore_ListUsers_Empty(t *testing.T) {
	store := NewInMemoryUserStore()
	users, total, err := store.ListUsers(1, 20, "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, users)
}

func TestInMemoryUserStore_ListUsers_Pagination(t *testing.T) {
	store := NewInMemoryUserStore()
	for i := 0; i < 5; i++ {
		_, _ = store.CreateUser(RegisterRequest{
			Username: fmt.Sprintf("user%d", i),
			Password: "pass123",
		})
	}

	users, total, err := store.ListUsers(1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Equal(t, 2, len(users))

	users, total, err = store.ListUsers(3, 2, "")
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Equal(t, 1, len(users))
}

func TestInMemoryUserStore_ListUsers_Search(t *testing.T) {
	store := NewInMemoryUserStore()
	_, _ = store.CreateUser(RegisterRequest{Username: "alice", Password: "pass123", Nickname: "Alice"})
	_, _ = store.CreateUser(RegisterRequest{Username: "bob", Password: "pass123", Nickname: "Bob"})

	users, total, err := store.ListUsers(1, 20, "alice")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "alice", users[0].Username)
}

func TestInMemoryUserStore_UpdateUser(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "update_me", Password: "pass123"})

	email := "new@email.com"
	nick := "NewName"
	updated, err := store.UpdateUser(user.ID, UpdateUserRequest{
		Nickname: &nick,
		Email:    &email,
	})
	require.NoError(t, err)
	assert.Equal(t, "NewName", updated.Nickname)
	assert.Equal(t, "new@email.com", updated.Email)

	_, err = store.UpdateUser(9999, UpdateUserRequest{})
	assert.Error(t, err)
}

func TestInMemoryUserStore_DeleteUser(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "delete_me", Password: "pass123"})

	err := store.DeleteUser(user.ID)
	require.NoError(t, err)

	_, err = store.FindByID(user.ID)
	assert.Error(t, err)

	err = store.DeleteUser(9999)
	assert.Error(t, err)
}

func TestInMemoryUserStore_ChangePassword(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "changepass", Password: "oldpass"})

	err := store.ChangePassword(user.ID, "oldpass", "newpass")
	require.NoError(t, err)

	_, err = store.VerifyPassword("changepass", "oldpass")
	assert.Error(t, err)

	result, err := store.VerifyPassword("changepass", "newpass")
	require.NoError(t, err)
	assert.Equal(t, user.ID, result.ID)
}

func TestInMemoryUserStore_ChangePassword_WrongOld(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "wrongold", Password: "realpass"})

	err := store.ChangePassword(user.ID, "wrongpass", "newpass")
	assert.Error(t, err)
}

func TestInMemoryUserStore_InactiveCannotLogin(t *testing.T) {
	store := NewInMemoryUserStore()
	user, _ := store.CreateUser(RegisterRequest{Username: "inactiveuser", Password: "pass123"})
	_ = store.DeleteUser(user.ID)

	_, err := store.VerifyPassword("inactiveuser", "pass123")
	assert.Error(t, err)
}

func TestApiKeyService_GenerateAndValidate(t *testing.T) {
	store := NewInMemoryApiKeyStore()
	svc := NewApiKeyService(store)

	key, err := svc.GenerateKey("test-key", 1, "*:*", nil)
	require.NoError(t, err)
	assert.Equal(t, "test-key", key.Name)
	assert.NotEmpty(t, key.RawKey)
	assert.Contains(t, key.RawKey, "gbk_")
	assert.NotEmpty(t, key.KeyPrefix)
	assert.Equal(t, "*:*", key.Scopes)
	assert.Equal(t, "active", key.Status)

	validated, err := svc.ValidateKey(key.RawKey)
	require.NoError(t, err)
	assert.Equal(t, key.ID, validated.ID)
	assert.NotNil(t, validated.LastUsedAt)
}

func TestApiKeyService_InvalidKey(t *testing.T) {
	store := NewInMemoryApiKeyStore()
	svc := NewApiKeyService(store)

	_, err := svc.ValidateKey("gbk_invalidkey")
	assert.Error(t, err)
}

func TestApiKeyService_ExpiredKey(t *testing.T) {
	store := NewInMemoryApiKeyStore()
	svc := NewApiKeyService(store)

	past := time.Now().Add(-time.Hour)
	key, err := svc.GenerateKey("expired", 1, "*:*", &past)
	require.NoError(t, err)

	_, err = svc.ValidateKey(key.RawKey)
	assert.Error(t, err)
}

func TestApiKeyService_ListAndDelete(t *testing.T) {
	store := NewInMemoryApiKeyStore()
	svc := NewApiKeyService(store)

	_, _ = svc.GenerateKey("k1", 1, "*:*", nil)
	_, _ = svc.GenerateKey("k2", 1, "*:*", nil)
	_, _ = svc.GenerateKey("k3", 2, "*:*", nil)

	keys, total, err := svc.ListAll(1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, 3, len(keys))

	user1Keys, err := svc.ListByUser(1)
	require.NoError(t, err)
	assert.Equal(t, 2, len(user1Keys))

	_ = svc.Delete(user1Keys[0].ID)
	keys, total, _ = svc.ListAll(1, 20)
	assert.Equal(t, int64(2), total)

	_, err = svc.ListByUser(999)
	require.NoError(t, err)
	assert.Empty(t, err)
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
