package config

import (
	"strings"
	"testing"
)

func TestFromValues(t *testing.T) {
	values := map[string]string{
		"APP_ENV":     "development",
		"DB_HOST":     "localhost",
		"DB_USER":     "small_company",
		"DB_NAME":     "small_company",
		"JWT_SECRET":  strings.Repeat("x", 32),
		"SERVER_PORT": "8081",
	}
	cfg, err := fromValues(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8081 {
		t.Fatalf("unexpected config values: %+v", cfg)
	}
}

func TestFromValuesRejectsWeakSecret(t *testing.T) {
	base := map[string]string{
		"APP_ENV":    "development",
		"DB_HOST":    "localhost",
		"DB_USER":    "small_company",
		"DB_NAME":    "small_company",
		"JWT_SECRET": strings.Repeat("x", 32),
	}
	for _, test := range []struct {
		name string
		key  string
		val  string
	}{
		{"weak secret", "JWT_SECRET", "short"},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := make(map[string]string, len(base)+1)
			for key, value := range base {
				values[key] = value
			}
			values[test.key] = test.val
			if _, err := fromValues(func(key string) string { return values[key] }); err == nil {
				t.Fatal("expected invalid configuration to be rejected")
			}
		})
	}
}
