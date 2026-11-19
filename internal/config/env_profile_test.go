package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles_ProfileOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_PROFILE=dev\nBASE=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.dev"), []byte("BASE=2\nPROFILE_ONLY=yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldProfile, hadProfile := os.LookupEnv("APP_PROFILE")
	oldBase, hadBase := os.LookupEnv("BASE")
	oldOnly, hadOnly := os.LookupEnv("PROFILE_ONLY")
	t.Cleanup(func() {
		restoreEnv("APP_PROFILE", oldProfile, hadProfile)
		restoreEnv("BASE", oldBase, hadBase)
		restoreEnv("PROFILE_ONLY", oldOnly, hadOnly)
	})
	_ = os.Unsetenv("APP_PROFILE")
	_ = os.Unsetenv("BASE")
	_ = os.Unsetenv("PROFILE_ONLY")

	if err := loadEnvFiles(dir); err != nil {
		t.Fatalf("loadEnvFiles 失败: %v", err)
	}
	if got := os.Getenv("BASE"); got != "2" {
		t.Errorf("profile 未覆盖基础 .env: %q", got)
	}
	if got := os.Getenv("PROFILE_ONLY"); got != "yes" {
		t.Errorf("profile 变量未加载: %q", got)
	}
}

func TestLoadEnvFiles_ProfilePrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(
		"APP_PROFILE=host\nAPP_HOST_SERVER_PUBLIC_BASE_URL=https://host.example.com\nAPP_HOST_DATABASE_POSTGRES_MAIN_HOST=10.0.0.2\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	oldProfile, hadProfile := os.LookupEnv("APP_PROFILE")
	oldURL, hadURL := os.LookupEnv("APP_SERVER_PUBLIC_BASE_URL")
	oldHost, hadHost := os.LookupEnv("APP_DATABASE_POSTGRES_MAIN_HOST")
	t.Cleanup(func() {
		restoreEnv("APP_PROFILE", oldProfile, hadProfile)
		restoreEnv("APP_SERVER_PUBLIC_BASE_URL", oldURL, hadURL)
		restoreEnv("APP_DATABASE_POSTGRES_MAIN_HOST", oldHost, hadHost)
	})
	_ = os.Unsetenv("APP_PROFILE")
	_ = os.Unsetenv("APP_SERVER_PUBLIC_BASE_URL")
	_ = os.Unsetenv("APP_DATABASE_POSTGRES_MAIN_HOST")

	if err := loadEnvFiles(dir); err != nil {
		t.Fatalf("loadEnvFiles 失败: %v", err)
	}
	if got := os.Getenv("APP_SERVER_PUBLIC_BASE_URL"); got != "https://host.example.com" {
		t.Errorf("base url 前缀映射失败: %q", got)
	}
	if got := os.Getenv("APP_DATABASE_POSTGRES_MAIN_HOST"); got != "10.0.0.2" {
		t.Errorf("数据库前缀映射失败: %q", got)
	}
}

func TestLoadEnvFiles_InvalidProfile(t *testing.T) {
	dir := t.TempDir()
	old, had := os.LookupEnv("APP_PROFILE")
	t.Cleanup(func() { restoreEnv("APP_PROFILE", old, had) })
	if err := os.Setenv("APP_PROFILE", "../evil"); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvFiles(dir); err == nil {
		t.Fatal("非法 profile 应报错")
	}
}

func restoreEnv(key, value string, existed bool) {
	if existed {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}
