package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"warehouse-service/domain/entity"
	"warehouse-service/domain/repository"
	"warehouse-service/infrastructure/security"
	"warehouse-service/internal/config"
	"warehouse-service/presentation/controller"
	"warehouse-service/usecase"
)

type testAuthRepository struct{}

func (testAuthRepository) FindByEmail(_ context.Context, _ string) (entity.User, string, error) {
	return entity.User{}, "", repository.ErrNotFound
}

func TestProductsRequireAuthentication(t *testing.T) {
	cfg := config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: []byte(strings.Repeat("x", 32))}
	tokens := security.NewJWT(cfg.JWTSecret)
	api := controller.New(usecase.NewAuth(testAuthRepository{}, tokens), nil, nil, nil, tokens)
	server := New(cfg, nil, api)
	response, err := server.App.Test(httptest.NewRequest("GET", "/api/v1/products", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401 for a protected product route", response.StatusCode)
	}
}

func TestLoginRejectsMissingCredentials(t *testing.T) {
	secret := []byte(strings.Repeat("x", 32))
	tokens := security.NewJWT(secret)
	api := controller.New(usecase.NewAuth(testAuthRepository{}, tokens), nil, nil, nil, tokens)
	server := New(config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: secret}, nil, api)
	request := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := server.App.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}
