package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/config"
)

func TestWarehouseRouteIsNotRegistered(t *testing.T) {
	cfg := config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: []byte(strings.Repeat("x", 32))}
	api := New(cfg, nil, auth.NewService(nil, cfg.JWTSecret))
	response, err := api.App.Test(httptest.NewRequest("GET", "/api/v1/products", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 404 {
		t.Fatalf("status = %d, want 404 because warehouse API is outside the ERD", response.StatusCode)
	}
}

func TestLoginRejectsMissingCredentials(t *testing.T) {
	secret := []byte(strings.Repeat("x", 32))
	api := New(config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: secret}, nil, auth.NewService(nil, secret))
	request := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := api.App.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}
