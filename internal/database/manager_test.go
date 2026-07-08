package database

import (
	"sync"
	"testing"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestInitAll_Empty(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)
	assert.NotNil(t, DB)
	assert.NotNil(t, DB.MySQL)
	assert.NotNil(t, DB.Postgres)
	assert.NotNil(t, DB.Redis)
	assert.NotNil(t, DB.S3)
	assert.NotNil(t, DB.RustFS)
	assert.NotNil(t, DB.breakers)
	CloseAll()
}

func TestInitAll_InvalidMySQL(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			MySQL: map[string]config.MySQLConfig{
				"main": {
					Host:   "127.0.0.1",
					Port:   9999,
					User:   "root",
					DBName: "test",
				},
			},
		},
	}
	err := InitAll(cfg)
	assert.NoError(t, err)
	assert.Empty(t, DB.MySQL)
	CloseAll()
}

func TestInitAll_InvalidPostgres(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Postgres: map[string]config.PGConfig{
				"nas": {
					Host:   "127.0.0.1",
					Port:   9999,
					User:   "postgres",
					DBName: "test",
				},
			},
		},
	}
	err := InitAll(cfg)
	assert.NoError(t, err)
	assert.Empty(t, DB.Postgres)
	CloseAll()
}

func TestInitAll_InvalidS3(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			S3: map[string]config.S3Config{
				"minio": {
					Endpoint:  "invalid:invalid:9000",
					AccessKey: "test",
					SecretKey: "test",
					Bucket:    "test",
				},
			},
		},
	}
	err := InitAll(cfg)
	assert.NoError(t, err)
	assert.Empty(t, DB.S3)
	CloseAll()
}

func TestHealth_NilDB(t *testing.T) {
	DB = nil
	h := Health()
	assert.Contains(t, h, "database")
	assert.Equal(t, "error", h["database"].Status)
}

func TestHealth_EmptyInit(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)
	defer CloseAll()

	h := Health()
	assert.Empty(t, h)
}

func TestInitAll_UnknownS3Type(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			S3: map[string]config.S3Config{
				"unknown": {},
			},
		},
	}
	err := InitAll(cfg)
	assert.NoError(t, err)
	CloseAll()
}

func TestCloseAll_Multiple(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)

	CloseAll()
	CloseAll()
}

func TestReload_RecreatesPools(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)

	err = DB.Reload(&config.Config{})
	assert.NoError(t, err)
	CloseAll()
}

func TestRunWithBreaker(t *testing.T) {
	DB = &DBManager{breakers: map[string]*CircuitBreaker{}}
	DB.breakers["mysql:main"] = NewCircuitBreaker(1, time.Second)

	var called int
	err := RunWithBreaker("mysql:main", func() error {
		called++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestBreaker_OpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Minute)
	assert.True(t, cb.Allow())

	for i := 0; i < 3; i++ {
		cb.Failure()
	}
	assert.False(t, cb.Allow())

	cb.Success()
	assert.True(t, cb.Allow())
}

func TestGetRedis_Nil(t *testing.T) {
	DB = nil
	client := GetRedis("main")
	assert.Nil(t, client)
}

func TestConcurrentHealth(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)
	defer CloseAll()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Health()
		}()
	}
	wg.Wait()
}
