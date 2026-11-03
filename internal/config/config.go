package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv     string
	Port       int
	CORSOrigin string
	Database   Database
	JWTSecret  []byte
	OTPTTL     time.Duration
	OCRBinary  string
	OCRLang    string
}

type Database struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
	Schema   string
}

func Load() (Config, error) {
	values := map[string]string{}
	if err := loadDotEnv(".env", values); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	get := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return values[key]
	}
	return fromValues(get)
}

func fromValues(get func(string) string) (Config, error) {
	var cfg Config
	var err error

	cfg.AppEnv = strings.ToLower(strings.TrimSpace(get("APP_ENV")))
	if cfg.AppEnv == "" {
		return cfg, errors.New("APP_ENV wajib diatur (development, test, atau production)")
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "test" && cfg.AppEnv != "production" {
		return cfg, fmt.Errorf("APP_ENV tidak dikenal: %q", cfg.AppEnv)
	}

	cfg.Port, err = intValue(get, "SERVER_PORT", 8080)
	if err != nil || cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, errors.New("SERVER_PORT harus berupa port 1-65535")
	}
	cfg.CORSOrigin = strings.TrimSpace(get("CORS_ORIGIN"))
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "http://localhost:3000"
	}

	cfg.Database = Database{
		Host:     strings.TrimSpace(get("DB_HOST")),
		User:     strings.TrimSpace(get("DB_USER")),
		Password: get("DB_PASSWORD"),
		Name:     strings.TrimSpace(get("DB_NAME")),
		SSLMode:  strings.TrimSpace(get("DB_SSLMODE")),
		Schema:   strings.TrimSpace(get("DB_SCHEMA")),
	}
	cfg.Database.Port, err = intValue(get, "DB_PORT", 5432)
	if err != nil || cfg.Database.Port < 1 || cfg.Database.Port > 65535 {
		return cfg, errors.New("DB_PORT harus berupa port 1-65535")
	}
	if cfg.Database.Host == "" || cfg.Database.User == "" || cfg.Database.Name == "" {
		return cfg, errors.New("DB_HOST, DB_USER, dan DB_NAME wajib diatur")
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.Schema == "" {
		cfg.Database.Schema = "warehouse"
	}
	if !validSchema(cfg.Database.Schema) {
		return cfg, errors.New("DB_SCHEMA hanya boleh berisi huruf, angka, dan underscore; karakter pertama harus huruf atau underscore")
	}
	if cfg.Database.Schema != "warehouse" {
		return cfg, errors.New("DB_SCHEMA harus bernilai warehouse agar sesuai dengan migration")
	}

	secret := get("JWT_SECRET")
	if len(secret) < 32 {
		return cfg, errors.New("JWT_SECRET wajib memiliki minimal 32 karakter")
	}
	cfg.JWTSecret = []byte(secret)

	cfg.OTPTTL, err = time.ParseDuration(strings.TrimSpace(get("OTP_TTL")))
	if get("OTP_TTL") == "" {
		cfg.OTPTTL = 5 * time.Minute
	} else if err != nil || cfg.OTPTTL < time.Minute || cfg.OTPTTL > 15*time.Minute {
		return cfg, errors.New("OTP_TTL harus berdurasi 1m sampai 15m")
	}

	cfg.OCRBinary = strings.TrimSpace(get("OCR_BINARY"))
	if cfg.OCRBinary == "" {
		cfg.OCRBinary = "tesseract"
	}
	cfg.OCRLang = strings.TrimSpace(get("OCR_LANG"))
	if cfg.OCRLang == "" {
		cfg.OCRLang = "ind+eng"
	}
	return cfg, nil
}

func intValue(get func(string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(get(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func validSchema(schema string) bool {
	if schema == "" {
		return false
	}
	for i, r := range schema {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func loadDotEnv(path string, values map[string]string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		} else if comment := strings.Index(value, " #"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}
