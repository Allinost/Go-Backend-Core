//go:build integration

package config

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_HotReload(t *testing.T) {
	resetForTest()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	initial := `server:
  name: hot-reload-test
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
  watch: true
`
	err := os.WriteFile(cfgPath, []byte(initial), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.True(t, cfg.Config.Watch)

	var called atomic.Bool
	RegisterReloader(&testHotReloader{fn: func(cfg *Config) error {
		called.Store(true)
		return nil
	}})

	updated := `server:
  name: hot-reload-test
  version: v2.0
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
  watch: true
`
	err = os.WriteFile(cfgPath, []byte(updated), 0644)
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if called.Load() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.True(t, called.Load(), "Reloader 应在文件变更后被调用")
	assert.Equal(t, "v2.0", Get().Server.Version)
}

type testHotReloader struct {
	fn func(cfg *Config) error
}

func (r *testHotReloader) Reload(cfg *Config) error {
	return r.fn(cfg)
}
