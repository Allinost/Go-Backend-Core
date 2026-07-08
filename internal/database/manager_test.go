package database

import (
	"testing"

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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MySQL[main]")
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PostgreSQL[nas]")
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MinIO")
}

func TestInitAll_InvalidRustFS(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			S3: map[string]config.S3Config{
				"rustfs": {
					Endpoint:  "invalid:invalid:9000",
					AccessKey: "test",
					SecretKey: "test",
					Bucket:    "test",
				},
			},
		},
	}
	err := InitAll(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RustFS")
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的 S3 存储类型")
}

func TestCloseAll_Multiple(t *testing.T) {
	err := InitAll(&config.Config{})
	assert.NoError(t, err)

	CloseAll()
	CloseAll()
}
