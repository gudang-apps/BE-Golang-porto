package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPhonePatternAndNormalization(t *testing.T) {
	valid := normalizePhone(" +628123456789 ")
	if !phonePattern.MatchString(valid) {
		t.Fatalf("expected valid E.164 phone, got %q", valid)
	}
	if phonePattern.MatchString("08123456789") {
		t.Fatal("local phone format must be rejected; use E.164")
	}
}

func TestOTPHashIsBoundToChallenge(t *testing.T) {
	service := NewService(nil, []byte(strings.Repeat("x", 32)), 0, nil)
	hash := service.hashCode("challenge-a", "123456")
	if !service.matchesCode("challenge-a", "123456", fmtHex(hash)) {
		t.Fatal("expected matching code")
	}
	if service.matchesCode("challenge-b", "123456", fmtHex(hash)) {
		t.Fatal("code hash must not be reusable across challenges")
	}
}

func TestParseTokenRequiresExpiryAndHS256(t *testing.T) {
	secret := []byte(strings.Repeat("x", 32))
	service := NewService(nil, secret, 0, nil)
	valid := signedAuthToken(t, jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "a8c17038-50e1-4efb-849a-dda82cbde8cb",
		"phone": "+628123456789",
		"role":  "staff",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}, secret)
	if user, err := service.ParseToken(valid); err != nil || user.Role != "staff" {
		t.Fatalf("ParseToken() = (%+v, %v), want staff user", user, err)
	}

	missingExpiry := signedAuthToken(t, jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   "a8c17038-50e1-4efb-849a-dda82cbde8cb",
		"phone": "+628123456789",
		"role":  "staff",
	}, secret)
	if _, err := service.ParseToken(missingExpiry); err == nil {
		t.Fatal("expected token without expiry to be rejected")
	}

	weakAlgorithm := signedAuthToken(t, jwt.SigningMethodHS384, jwt.MapClaims{
		"sub":   "a8c17038-50e1-4efb-849a-dda82cbde8cb",
		"phone": "+628123456789",
		"role":  "staff",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}, secret)
	if _, err := service.ParseToken(weakAlgorithm); err == nil {
		t.Fatal("expected non-HS256 token to be rejected")
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

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2] = digits[b>>4]
		result[i*2+1] = digits[b&15]
	}
	return string(result)
}
