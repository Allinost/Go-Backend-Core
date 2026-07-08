package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
server:
  name: test-server
  version: v0.0.0
  port: 9999
  mode: test

log:
  level: debug
  format: text
  output: stdout

database:
  mysql:
    main:
      host: localhost
      port: 3306
      user: root
      password: ""
      dbname: test_db

redis:
  main:
    addr: localhost:6379
    password: ""
    db: 0

auth:
  jwt_secret: test-secret
  jwt_expire: 1h

config:
  watch: false
`
	path := "test_config.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	cfg, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "test-server", cfg.Server.Name)
	assert.Equal(t, "v0.0.0", cfg.Server.Version)
	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "test", cfg.Server.Mode)

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)

	assert.Equal(t, "localhost", cfg.Database.MySQL["main"].Host)
	assert.Equal(t, 3306, cfg.Database.MySQL["main"].Port)
	assert.Equal(t, "test_db", cfg.Database.MySQL["main"].DBName)

	assert.Equal(t, "localhost:6379", cfg.Redis.Main.Addr)
	assert.Equal(t, 0, cfg.Redis.Main.DB)

	assert.Equal(t, "test-secret", cfg.Auth.JWTSecret)
	assert.Equal(t, "1h", cfg.Auth.JWTExpire)

	assert.False(t, cfg.Config.Watch)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("non_existent.yaml")
	assert.Error(t, err)
}

func TestGet_AfterLoad(t *testing.T) {
	content := "server:\n  name: get-test\n  version: v1.0\n  port: 29090\n  mode: test\nlog:\n  level: info\n  format: json\n  output: stdout\ndatabase:\n  mysql:\n    main:\n      host: localhost\n      port: 3306\n      user: root\n      password: \"\"\n      dbname: test\nredis:\n  main:\n    addr: localhost:6379\n    password: \"\"\n    db: 0\nauth:\n  jwt_secret: \"\"\n  jwt_expire: 24h\nconfig:\n  watch: false\n"
	path := "test_get_config.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	cfg, err := Load(path)
	require.NoError(t, err)

	got := Get()
	assert.Equal(t, cfg.Server.Name, got.Server.Name)
}

func TestLoad_EnvOverride(t *testing.T) {
	content := "server:\n  port: 29090\n  mode: test\nlog:\n  level: info\n  format: json\n  output: stdout\ndatabase:\n  mysql:\n    main:\n      host: localhost\n      port: 3306\n      user: root\n      password: \"\"\n      dbname: test\nredis:\n  main:\n    addr: localhost:6379\n    password: \"\"\n    db: 0\nauth:\n  jwt_secret: \"\"\n  jwt_expire: 24h\nconfig:\n  watch: false\n"
	path := "test_env_config.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	t.Setenv("APP_SERVER_PORT", "12345")
	t.Setenv("APP_AUTH_JWT_SECRET", "env-secret")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 12345, cfg.Server.Port)
	assert.Equal(t, "env-secret", cfg.Auth.JWTSecret)
}

func TestRegisterReloader(t *testing.T) {
	RegisterReloader(&testReloader{fn: func(cfg *Config) error {
		return nil
	}})
	assert.Len(t, rls, 1)
}

type testReloader struct {
	fn func(cfg *Config) error
}

func (r *testReloader) Reload(cfg *Config) error {
	return r.fn(cfg)
}
