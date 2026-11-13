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
	if cfg.Cloudinary.UploadFolder != "crud_image" {
		t.Fatalf("Cloudinary upload folder = %q, want crud_image", cfg.Cloudinary.UploadFolder)
	}
}

func TestFromValuesUsesRenderPort(t *testing.T) {
	values := map[string]string{
		"APP_ENV":    "development",
		"PORT":       "10000",
		"DB_HOST":    "localhost",
		"DB_USER":    "small_company",
		"DB_NAME":    "small_company",
		"JWT_SECRET": strings.Repeat("x", 32),
	}
	cfg, err := fromValues(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 10000 {
		t.Fatalf("port = %d, want Render PORT 10000", cfg.Port)
	}
}

func TestProductionRequiresCloudinaryCredentials(t *testing.T) {
	values := map[string]string{
		"APP_ENV":    "production",
		"DB_HOST":    "localhost",
		"DB_USER":    "small_company",
		"DB_NAME":    "small_company",
		"JWT_SECRET": strings.Repeat("x", 32),
	}
	if _, err := fromValues(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected production Cloudinary credentials to be required")
	}
}

func TestCloudinaryCredentialsMustBeComplete(t *testing.T) {
	values := map[string]string{
		"APP_ENV":               "development",
		"DB_HOST":               "localhost",
		"DB_USER":               "small_company",
		"DB_NAME":               "small_company",
		"JWT_SECRET":            strings.Repeat("x", 32),
		"CLOUDINARY_API_KEY":    "api-key",
		"CLOUDINARY_API_SECRET": "api-secret",
	}
	if _, err := fromValues(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected partial Cloudinary credentials to be rejected")
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
