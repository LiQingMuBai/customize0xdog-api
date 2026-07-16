package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	ListenAddr    string
	TeldogBaseURL string
	TeldogAPIKey  string
	HTTPTimeout   time.Duration
}

func Load() (Config, error) {
	_ = loadDotEnv(".env")

	cfg := Config{
		ListenAddr:  strings.TrimSpace(getenvDefault("LISTEN_ADDR", ":8080")),
		HTTPTimeout: 12 * time.Second,
	}

	cfg.TeldogBaseURL = strings.TrimSpace(os.Getenv("TELDOG_BASE_URL"))
	cfg.TeldogAPIKey = strings.TrimSpace(os.Getenv("TELDOG_API_KEY"))

	if cfg.TeldogBaseURL == "" {
		return Config{}, errors.New("TELDOG_BASE_URL is required")
	}
	if cfg.TeldogAPIKey == "" {
		return Config{}, errors.New("TELDOG_API_KEY is required")
	}

	if v := strings.TrimSpace(os.Getenv("HTTP_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, err
		}
		cfg.HTTPTimeout = d
	}

	return cfg, nil
}

func getenvDefault(key, def string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func loadDotEnv(path string) error {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}
