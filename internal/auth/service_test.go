package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestEmailValidation(t *testing.T) {
	if !validEmail(normalizeEmail(" User@Example.com ")) {
		t.Fatal("expected valid normalized email")
	}
	for _, email := range []string{"", "not-an-email", " user@example.com ", "user@example.com,other@example.com"} {
		if validEmail(email) {
			t.Errorf("validEmail(%q) = true, want false", email)
		}
	}
}

func TestParseTokenRequiresExpiryAndHS256(t *testing.T) {
	secret := []byte(strings.Repeat("x", 32))
	service := NewService(nil, secret)
	valid := signedAuthToken(t, jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "42",
		"email": "user@example.com",
		"name":  "User",
		"roles": []string{"staff"},
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}, secret)
	if user, err := service.ParseToken(valid); err != nil || len(user.Roles) != 1 || user.Roles[0] != "staff" {
		t.Fatalf("ParseToken() = (%+v, %v), want staff user", user, err)
	}

	missingExpiry := signedAuthToken(t, jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "42",
		"email": "user@example.com",
		"name":  "User",
		"roles": []string{"staff"},
	}, secret)
	if _, err := service.ParseToken(missingExpiry); err == nil {
		t.Fatal("expected token without expiry to be rejected")
	}

	weakAlgorithm := signedAuthToken(t, jwt.SigningMethodHS384, jwt.MapClaims{
		"sub":   "42",
		"email": "user@example.com",
		"name":  "User",
		"roles": []string{"staff"},
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}, secret)
	if _, err := service.ParseToken(weakAlgorithm); err == nil {
		t.Fatal("expected non-HS256 token to be rejected")
	}

	invalidRole := signedAuthToken(t, jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "42",
		"email": "user@example.com",
		"name":  "User",
		"roles": []string{"admin"},
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}, secret)
	if _, err := service.ParseToken(invalidRole); err == nil {
		t.Fatal("expected unknown role to be rejected")
	}
}

func signedAuthToken(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, secret []byte) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
