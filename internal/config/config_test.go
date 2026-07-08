package config

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	resetForTest()
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
	resetForTest()
	_, err := Load("non_existent.yaml")
	assert.Error(t, err)
}

func TestGet_AfterLoad(t *testing.T) {
	resetForTest()
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
	resetForTest()
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
	resetForTest()
	RegisterReloader(&testReloader{fn: func(cfg *Config) error {
		return nil
	}})
	assert.Len(t, rls, 1)
}

func TestRegisterValidator(t *testing.T) {
	resetForTest()
	RegisterValidator(&testValidator{fn: func(cfg *Config) error {
		return nil
	}})
	RegisterValidator(&testValidator{fn: func(cfg *Config) error {
		return nil
	}})
	assert.Len(t, validators, 2)
}

func TestValidate_Pass(t *testing.T) {
	resetForTest()
	RegisterValidator(&testValidator{fn: func(cfg *Config) error { return nil }})
	err := Validate(&Config{})
	assert.NoError(t, err)
}

func TestValidate_Fail(t *testing.T) {
	resetForTest()
	RegisterValidator(&testValidator{fn: func(cfg *Config) error {
		return fmt.Errorf("server.port 不能为 0")
	}})
	err := Validate(&Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server.port 不能为 0")
}

func TestValidate_NoValidators(t *testing.T) {
	resetForTest()
	err := Validate(&Config{})
	assert.NoError(t, err)
}

func TestLoad_ValidationBlocksOnFailure(t *testing.T) {
	resetForTest()
	content := `
server:
  name: test
  version: v1.0
  port: 29090
  mode: test
log:
  level: info
  format: json
  output: stdout
database:
  mysql:
    main:
      host: localhost
      port: 3306
      user: root
      password: ""
      dbname: test
redis:
  main:
    addr: localhost:6379
    password: ""
    db: 0
auth:
  jwt_secret: ""
  jwt_expire: 24h
config:
  watch: false
`
	path := "test_validation_config.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	RegisterValidator(&testValidator{fn: func(cfg *Config) error {
		return fmt.Errorf("mock validation failure")
	}})

	_, err = Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock validation failure")
}

func TestOnChange_CalledAfterSuccessfulReload(t *testing.T) {
	resetForTest()
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
	path := "test_onchange_config.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	_, err = Load(path)
	require.NoError(t, err)

	var hookCalled int32
	OnChange(func(cfg *Config) {
		atomic.StoreInt32(&hookCalled, 1)
	})

	err = reloadConfig()
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&hookCalled))
}

func TestOnChange_MultipleHooks(t *testing.T) {
	resetForTest()
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
	path := "test_onchange_multi.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	_, err = Load(path)
	require.NoError(t, err)

	var count int32
	OnChange(func(cfg *Config) { atomic.AddInt32(&count, 1) })
	OnChange(func(cfg *Config) { atomic.AddInt32(&count, 1) })

	err = reloadConfig()
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&count))
}

func TestOnChange_ReceivesConfig(t *testing.T) {
	resetForTest()
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
	path := "test_onchange_cfg.yaml"
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	defer os.Remove(path)

	_, err = Load(path)
	require.NoError(t, err)

	var gotPort int
	OnChange(func(cfg *Config) {
		gotPort = cfg.Server.Port
	})

	err = reloadConfig()
	require.NoError(t, err)
	assert.Equal(t, 9999, gotPort)
}

type testReloader struct {
	fn func(cfg *Config) error
}

func (r *testReloader) Reload(cfg *Config) error {
	return r.fn(cfg)
}

type testValidator struct {
	fn func(cfg *Config) error
}

func (v *testValidator) Validate(cfg *Config) error {
	return v.fn(cfg)
}
