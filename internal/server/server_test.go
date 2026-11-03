package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

func TestInventoryRequiresAuthentication(t *testing.T) {
	cfg := config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: []byte(strings.Repeat("x", 32))}
	api := New(cfg, nil, auth.NewService(nil, cfg.JWTSecret, time.Minute, nil))
	response, err := api.App.Test(httptest.NewRequest("GET", "/api/v1/products", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestOnlyAdminCanInviteUsers(t *testing.T) {
	secret := []byte(strings.Repeat("x", 32))
	cfg := config.Config{Port: 8080, CORSOrigin: "http://localhost:3000", JWTSecret: secret}
	api := New(cfg, nil, auth.NewService(nil, secret, time.Minute, nil))
	token := signedToken(t, secret, "staff")
	request := httptest.NewRequest("POST", "/api/v1/auth/invitations", strings.NewReader(`{"phone":"+628123456789"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := api.App.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 403 {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
}

func signedToken(t *testing.T, secret []byte, role string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "a8c17038-50e1-4efb-849a-dda82cbde8cb",
		"phone": "+628123456789",
		"role":  role,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestStockQuantityPrecision(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  bool
	}{
		{0, true},
		{12.345, true},
		{12.3456, false},
	} {
		if got := threeDecimalPlaces(test.value); got != test.want {
			t.Errorf("threeDecimalPlaces(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}
