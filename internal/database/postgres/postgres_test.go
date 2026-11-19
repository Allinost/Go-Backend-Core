package postgres

import (
	"net/url"
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPool_InvalidDSN(t *testing.T) {
	cfg := config.PGConfig{
		Host:   "127.0.0.1",
		Port:   9999,
		User:   "postgres",
		DBName: "test",
	}
	pool, err := NewPool(cfg, false)
	assert.Error(t, err)
	assert.Nil(t, pool)
}

func TestNewPool_EmptyConfig(t *testing.T) {
	pool, err := NewPool(config.PGConfig{}, false)
	assert.Error(t, err)
	assert.Nil(t, pool)
}

func TestPool_Close_NilDB(t *testing.T) {
	p := &Pool{}
	err := p.Close()
	assert.NoError(t, err)
}

func TestPool_Health_NilDB(t *testing.T) {
	p := &Pool{}
	err := p.Health()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}

func TestBuildDSN_DefaultsAndEscaping(t *testing.T) {
	dsn, err := buildDSN(config.PGConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "p@ss word",
		DBName:   "go_backend_core",
	})
	require.NoError(t, err)

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	assert.Equal(t, "postgres", u.Scheme)
	assert.Equal(t, "127.0.0.1:5432", u.Host)
	assert.Equal(t, "postgres", u.User.Username())
	pass, ok := u.User.Password()
	assert.True(t, ok)
	assert.Equal(t, "p@ss word", pass)
	assert.Equal(t, "/go_backend_core", u.Path)
	assert.Equal(t, "disable", u.Query().Get("sslmode"))
	assert.Equal(t, "public", u.Query().Get("search_path"))
	assert.Equal(t, "UTC", u.Query().Get("TimeZone"))
}

func TestBuildDSN_Overrides(t *testing.T) {
	dsn, err := buildDSN(config.PGConfig{
		Host:       "db.internal",
		Port:       6432,
		User:       "app",
		Password:   "secret",
		DBName:     "app",
		SSLMode:    "require",
		SearchPath: "app,public",
		TimeZone:   "Asia/Shanghai",
	})
	require.NoError(t, err)

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	assert.Equal(t, "require", u.Query().Get("sslmode"))
	assert.Equal(t, "app,public", u.Query().Get("search_path"))
	assert.Equal(t, "Asia/Shanghai", u.Query().Get("TimeZone"))
}

func TestBuildDSN_Invalid(t *testing.T) {
	_, err := buildDSN(config.PGConfig{})
	assert.Error(t, err)

	_, err = buildDSN(config.PGConfig{Host: "localhost", DBName: "db"})
	assert.Error(t, err)

	_, err = buildDSN(config.PGConfig{Host: "localhost", Port: 5432})
	assert.Error(t, err)
}
