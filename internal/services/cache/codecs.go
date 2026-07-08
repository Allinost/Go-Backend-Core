package cache

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"sync"
)

type Codec interface {
	Encode(value interface{}) ([]byte, error)
	Decode(data []byte, dest interface{}) error
	Version() byte
	Name() string
}

type gobCodec struct{}

func (gobCodec) Encode(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (gobCodec) Decode(data []byte, dest interface{}) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(dest)
}

func (gobCodec) Version() byte { return 1 }
func (gobCodec) Name() string  { return "gob" }

type jsonCodec struct{}

func (jsonCodec) Encode(value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

func (jsonCodec) Decode(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

func (jsonCodec) Version() byte { return 2 }
func (jsonCodec) Name() string  { return "json" }

var (
	codecsMu    sync.RWMutex
	codecsByVer = map[byte]Codec{
		1: gobCodec{},
		2: jsonCodec{},
	}
	codecsByName = map[string]Codec{
		"gob":  gobCodec{},
		"json": jsonCodec{},
	}
)

func RegisterCodec(c Codec) {
	codecsMu.Lock()
	defer codecsMu.Unlock()
	codecsByVer[c.Version()] = c
	codecsByName[c.Name()] = c
}

func getCodec(name string) Codec {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	return codecsByName[name]
}

func getCodecByVer(ver byte) Codec {
	codecsMu.RLock()
	defer codecsMu.RUnlock()
	return codecsByVer[ver]
}

func encodeWith(codec Codec, value interface{}) ([]byte, error) {
	payload, err := codec.Encode(value)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(payload)+1)
	out = append(out, codec.Version())
	out = append(out, payload...)
	return out, nil
}

var legacyGob = gobCodec{}

func decodeWith(data []byte, dest interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("cache: empty data")
	}

	ver := data[0]
	codec := getCodecByVer(ver)

	if codec != nil && len(data) > 1 {
		if err := codec.Decode(data[1:], dest); err != nil {
			return fmt.Errorf("cache: decode version %d: %w", ver, err)
		}
		return nil
	}

	if err := legacyGob.Decode(data, dest); err != nil {
		return fmt.Errorf("cache: decode (legacy gob): %w", err)
	}
	return nil
}
