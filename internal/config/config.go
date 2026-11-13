package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv     string
	Port       int
	CORSOrigin string
	Database   Database
	JWTSecret  []byte
	Cloudinary Cloudinary
}

type Database struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type Cloudinary struct {
	CloudName    string
	APIKey       string
	APISecret    string
	UploadFolder string
	Enabled      bool
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

	portValue := get("SERVER_PORT")
	if strings.TrimSpace(portValue) == "" {
		portValue = get("PORT")
	}
	cfg.Port, err = intValue(func(string) string { return portValue }, "SERVER_PORT", 8080)
	if err != nil || cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, errors.New("SERVER_PORT atau PORT harus berupa port 1-65535")
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

	secret := get("JWT_SECRET")
	if len(secret) < 32 {
		return cfg, errors.New("JWT_SECRET wajib memiliki minimal 32 karakter")
	}
	cfg.JWTSecret = []byte(secret)

	cfg.Cloudinary = Cloudinary{
		CloudName:    strings.TrimSpace(get("CLOUDINARY_CLOUD_NAME")),
		APIKey:       strings.TrimSpace(get("CLOUDINARY_API_KEY")),
		APISecret:    strings.TrimSpace(get("CLOUDINARY_API_SECRET")),
		UploadFolder: strings.TrimSpace(get("CLOUDINARY_UPLOAD_FOLDER")),
	}
	if cfg.Cloudinary.UploadFolder == "" {
		cfg.Cloudinary.UploadFolder = "crud_image"
	}
	hasCloudinaryCredentials := cfg.Cloudinary.CloudName != "" || cfg.Cloudinary.APIKey != "" || cfg.Cloudinary.APISecret != ""
	if hasCloudinaryCredentials {
		if cfg.Cloudinary.CloudName == "" || cfg.Cloudinary.APIKey == "" || cfg.Cloudinary.APISecret == "" {
			return cfg, errors.New("CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, dan CLOUDINARY_API_SECRET harus diatur bersama")
		}
		cfg.Cloudinary.Enabled = true
	} else if cfg.AppEnv == "production" {
		return cfg, errors.New("CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, dan CLOUDINARY_API_SECRET wajib diatur di production")
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
