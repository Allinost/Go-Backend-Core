package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Register(t *testing.T) {
	m := NewManager()
	err := m.Register(Flag{Name: "test-feature", Enabled: true})
	require.NoError(t, err)
	assert.True(t, m.IsEnabled("test-feature"))
}

func TestManager_RegisterDuplicate(t *testing.T) {
	m := NewManager()
	err := m.Register(Flag{Name: "dup"})
	require.NoError(t, err)
	err = m.Register(Flag{Name: "dup"})
	assert.Error(t, err)
}

func TestManager_RegisterOrUpdate(t *testing.T) {
	m := NewManager()
	m.RegisterOrUpdate(Flag{Name: "f1", Enabled: false})
	assert.False(t, m.IsEnabled("f1"))
	m.RegisterOrUpdate(Flag{Name: "f1", Enabled: true})
	assert.True(t, m.IsEnabled("f1"))
}

func TestManager_EnableDisable(t *testing.T) {
	m := NewManager()
	require.NoError(t, m.Register(Flag{Name: "toggle", Enabled: false}))
	assert.False(t, m.IsEnabled("toggle"))

	require.NoError(t, m.Enable("toggle"))
	assert.True(t, m.IsEnabled("toggle"))

	require.NoError(t, m.Disable("toggle"))
	assert.False(t, m.IsEnabled("toggle"))
}

func TestManager_EnableNotFound(t *testing.T) {
	m := NewManager()
	err := m.Enable("not-found")
	assert.Error(t, err)
}

func TestManager_IsEnabledDefault(t *testing.T) {
	m := NewManager()
	assert.False(t, m.IsEnabled("anything"))
}

func TestManager_List(t *testing.T) {
	m := NewManager()
	m.RegisterOrUpdate(Flag{Name: "a", Enabled: true})
	m.RegisterOrUpdate(Flag{Name: "b", Enabled: false})

	flags := m.List()
	assert.Len(t, flags, 2)
}

func TestManager_Get(t *testing.T) {
	m := NewManager()
	m.RegisterOrUpdate(Flag{Name: "get-test", Enabled: true, Description: "test"})

	f, ok := m.Get("get-test")
	assert.True(t, ok)
	assert.True(t, f.Enabled)
	assert.Equal(t, "test", f.Description)

	_, ok = m.Get("missing")
	assert.False(t, ok)
}

// TestManager_Delete 验证删除已存在的开关
func TestManager_Delete(t *testing.T) {
	m := NewManager()
	m.RegisterOrUpdate(Flag{Name: "to-delete", Enabled: true})
	assert.True(t, m.IsEnabled("to-delete"))

	err := m.Delete("to-delete")
	require.NoError(t, err)
	assert.False(t, m.IsEnabled("to-delete"))

	// 再次删除应返回错误
	err = m.Delete("to-delete")
	assert.Error(t, err)
}

// TestManager_DeleteNotFound 验证删除不存在的开关返回错误
func TestManager_DeleteNotFound(t *testing.T) {
	m := NewManager()
	err := m.Delete("not-exists")
	assert.Error(t, err)
}
