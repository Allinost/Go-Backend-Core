package rustfs

import (
	"testing"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewClient_InvalidEndpoint(t *testing.T) {
	cfg := config.S3Config{
		Endpoint:  "invalid:invalid:9000",
		AccessKey: "test",
		SecretKey: "test",
		Bucket:    "test",
	}
	client, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_EmptyConfig(t *testing.T) {
	client, err := NewClient(config.S3Config{})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestClient_Close(t *testing.T) {
	c := &Client{}
	err := c.Close()
	assert.NoError(t, err)
}

func TestClient_Health_NilClient(t *testing.T) {
	c := &Client{}
	err := c.Health()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}
