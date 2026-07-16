package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	unsetEnvForTest(t, "TELDOG_BASE_URL")
	unsetEnvForTest(t, "TELDOG_API_KEY")
	unsetEnvForTest(t, "LISTEN_ADDR")
	unsetEnvForTest(t, "HTTP_TIMEOUT")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := []byte(`
# comment
TELDOG_BASE_URL=https://api.example.com
TELDOG_API_KEY="agt_xxx"
LISTEN_ADDR=:8081
HTTP_TIMEOUT=15s
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}

	if got := os.Getenv("TELDOG_BASE_URL"); got != "https://api.example.com" {
		t.Fatalf("TELDOG_BASE_URL=%q", got)
	}
	if got := os.Getenv("TELDOG_API_KEY"); got != "agt_xxx" {
		t.Fatalf("TELDOG_API_KEY=%q", got)
	}
	if got := os.Getenv("LISTEN_ADDR"); got != ":8081" {
		t.Fatalf("LISTEN_ADDR=%q", got)
	}
	if got := os.Getenv("HTTP_TIMEOUT"); got != "15s" {
		t.Fatalf("HTTP_TIMEOUT=%q", got)
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()

	old, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}

	t.Cleanup(func() {
		var err error
		if had {
			err = os.Setenv(key, old)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore %s: %v", key, err)
		}
	})
}
