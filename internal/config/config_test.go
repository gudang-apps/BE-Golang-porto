package config

import (
	"strings"
	"testing"
)

func TestFromValues(t *testing.T) {
	values := map[string]string{
		"APP_ENV":     "development",
		"DB_HOST":     "localhost",
		"DB_USER":     "warehouse",
		"DB_NAME":     "warehouse",
		"DB_SCHEMA":   "warehouse",
		"JWT_SECRET":  strings.Repeat("x", 32),
		"OTP_TTL":     "5m",
		"OCR_LANG":    "ind+eng",
		"SERVER_PORT": "8081",
	}
	cfg, err := fromValues(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8081 || cfg.Database.Schema != "warehouse" || cfg.OCRLang != "ind+eng" {
		t.Fatalf("unexpected config values: %+v", cfg)
	}
}

func TestFromValuesRejectsWeakSecretAndSchemaInjection(t *testing.T) {
	base := map[string]string{
		"APP_ENV":    "development",
		"DB_HOST":    "localhost",
		"DB_USER":    "warehouse",
		"DB_NAME":    "warehouse",
		"JWT_SECRET": strings.Repeat("x", 32),
	}
	for _, test := range []struct {
		name string
		key  string
		val  string
	}{
		{"weak secret", "JWT_SECRET", "short"},
		{"schema injection", "DB_SCHEMA", "warehouse;drop schema"},
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
