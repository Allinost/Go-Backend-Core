package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBLAKE3Hash(t *testing.T) {
	h := BLAKE3Hash([]byte("test-data"))
	assert.Equal(t, 64, len(h))
}

func TestBLAKE3Hash_Deterministic(t *testing.T) {
	h1 := BLAKE3Hash([]byte("hello"))
	h2 := BLAKE3Hash([]byte("hello"))
	assert.Equal(t, h1, h2)
}

func TestBLAKE3Hash_Different(t *testing.T) {
	h1 := BLAKE3Hash([]byte("data1"))
	h2 := BLAKE3Hash([]byte("data2"))
	assert.NotEqual(t, h1, h2)
}

func TestHash_BLAKE3(t *testing.T) {
	h, err := Hash([]byte("test"), HashBLAKE3)
	assert.NoError(t, err)
	assert.Equal(t, 64, len(h))
}

func TestHash_InvalidAlgo(t *testing.T) {
	_, err := Hash([]byte("test"), "md5")
	assert.Error(t, err)
}
