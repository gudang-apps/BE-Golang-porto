package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidChallenge   = errors.New("invalid or expired verification code")
	ErrInvalidInvite      = errors.New("invalid or expired invitation")
	ErrAccountExists      = errors.New("account already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrSenderUnavailable  = errors.New("OTP sender is not configured")
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

type Sender interface {
	Send(context.Context, string, string, string, string) error
}

type ConsoleSender struct{}

func (ConsoleSender) Send(_ context.Context, phone, code, purpose, reference string) error {
	masked := maskPhone(phone)
	log.Printf("[development-only OTP] purpose=%s phone=%s code=%s reference=%s", purpose, masked, code, reference)
	return nil
}

type Service struct {
	db     *sql.DB
	secret []byte
	ttl    time.Duration
	sender Sender
}

func NewService(db *sql.DB, secret []byte, ttl time.Duration, sender Sender) *Service {
	return &Service{db: db, secret: secret, ttl: ttl, sender: sender}
}

type User struct {
	ID    string
	Phone string
	Role  string
}

func (s *Service) InviteUser(ctx context.Context, phone, inviterID string) (string, error) {
	if s.sender == nil {
		return "", ErrSenderUnavailable
	}
	phone = normalizePhone(phone)
	if !phonePattern.MatchString(phone) {
		return "", ErrInvalidInput
	}
	code, err := generateOTP()
	if err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}
	invitationID := uuid.NewString()
	challengeID := uuid.NewString()
	now := time.Now().UTC()
	expires := now.Add(s.ttl)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM warehouse.users WHERE phone = $1)`, phone).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return "", ErrAccountExists
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE warehouse.otp_challenges SET consumed_at = now()
		WHERE invitation_id IN (
			SELECT id FROM warehouse.user_invitations
			WHERE phone=$1 AND accepted_at IS NULL AND expires_at > now()
		) AND consumed_at IS NULL`, phone); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE warehouse.user_invitations SET expires_at = now()
		WHERE phone=$1 AND accepted_at IS NULL AND expires_at > now()`, phone); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO warehouse.user_invitations (id, phone, role, invited_by, expires_at)
		VALUES ($1, $2, 'staff', $3, $4)`, invitationID, phone, inviterID, expires); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO warehouse.otp_challenges (id, phone, purpose, code_hash, invitation_id, expires_at)
		VALUES ($1, $2, 'invite', $3, $4, $5)`, challengeID, phone, s.hashCode(challengeID, code), invitationID, expires); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	if err := s.sender.Send(ctx, phone, code, "invite", invitationID); err != nil {
		if _, cleanupErr := s.db.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET consumed_at = now() WHERE id = $1`, challengeID); cleanupErr != nil {
			log.Printf("failed to invalidate unsent invitation OTP: %v", cleanupErr)
		}
		if _, cleanupErr := s.db.ExecContext(ctx, `UPDATE warehouse.user_invitations SET expires_at = now() WHERE id = $1`, invitationID); cleanupErr != nil {
			log.Printf("failed to invalidate unsent invitation: %v", cleanupErr)
		}
		return "", fmt.Errorf("send invitation OTP: %w", err)
	}
	return invitationID, nil
}

func (s *Service) Register(ctx context.Context, invitationID, phone, code, password string) error {
	phone = normalizePhone(phone)
	if !phonePattern.MatchString(phone) || len(password) < 10 || len(password) > 72 {
		return ErrInvalidInput
	}
	if _, err := uuid.Parse(invitationID); err != nil {
		return ErrInvalidInvite
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var challengeID, storedHash string
	var expires time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT c.id, encode(c.code_hash, 'hex'), c.expires_at
		FROM warehouse.user_invitations i
		JOIN warehouse.otp_challenges c ON c.invitation_id = i.id
		WHERE i.id = $1 AND i.phone = $2 AND i.accepted_at IS NULL
			AND c.purpose = 'invite' AND c.consumed_at IS NULL
		ORDER BY c.created_at DESC LIMIT 1
		FOR UPDATE OF i, c`, invitationID, phone).Scan(&challengeID, &storedHash, &expires)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidInvite
		}
		return err
	}
	if time.Now().After(expires) {
		return ErrInvalidInvite
	}
	if !s.matchesCode(challengeID, code, storedHash) {
		if _, err := tx.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET attempts = attempts + 1, consumed_at = CASE WHEN attempts + 1 >= 5 THEN now() ELSE consumed_at END WHERE id = $1`, challengeID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return ErrInvalidChallenge
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO warehouse.users (phone, password_hash, role)
		VALUES ($1, $2, 'staff')`, phone, string(passwordHash)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE warehouse.user_invitations SET accepted_at = now() WHERE id = $1`, invitationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET consumed_at = now() WHERE id = $1`, challengeID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) StartLogin(ctx context.Context, phone, password string) (string, error) {
	if s.sender == nil {
		return "", ErrSenderUnavailable
	}
	phone = normalizePhone(phone)
	if !phonePattern.MatchString(phone) || password == "" {
		return "", ErrInvalidCredentials
	}
	var userID, storedHash string
	var active bool
	err := s.db.QueryRowContext(ctx, `
		SELECT id, password_hash, active FROM warehouse.users WHERE phone = $1`, phone).
		Scan(&userID, &storedHash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("lookup account: %w", err)
	}
	if !active || bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) != nil {
		return "", ErrInvalidCredentials
	}

	code, err := generateOTP()
	if err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}
	challengeID := uuid.NewString()
	expires := time.Now().UTC().Add(s.ttl)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE warehouse.otp_challenges SET consumed_at = now()
		WHERE phone=$1 AND purpose='login' AND consumed_at IS NULL`, phone); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO warehouse.otp_challenges (id, phone, purpose, code_hash, user_id, expires_at)
		VALUES ($1, $2, 'login', $3, $4, $5)`,
		challengeID, phone, s.hashCode(challengeID, code), userID, expires); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	if err := s.sender.Send(ctx, phone, code, "login", challengeID); err != nil {
		if _, cleanupErr := s.db.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET consumed_at = now() WHERE id = $1`, challengeID); cleanupErr != nil {
			log.Printf("failed to invalidate unsent login OTP: %v", cleanupErr)
		}
		return "", fmt.Errorf("send login OTP: %w", err)
	}
	return challengeID, nil
}

func (s *Service) VerifyLogin(ctx context.Context, challengeID, phone, code string) (string, User, error) {
	phone = normalizePhone(phone)
	if _, err := uuid.Parse(challengeID); err != nil || !phonePattern.MatchString(phone) {
		return "", User{}, ErrInvalidChallenge
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", User{}, err
	}
	defer tx.Rollback()

	var storedHash, userID, role string
	var expires time.Time
	var active bool
	err = tx.QueryRowContext(ctx, `
		SELECT encode(c.code_hash, 'hex'), c.user_id, c.expires_at, u.role, u.active
		FROM warehouse.otp_challenges c JOIN warehouse.users u ON u.id = c.user_id
		WHERE c.id = $1 AND c.phone = $2 AND c.purpose = 'login'
			AND c.consumed_at IS NULL
		FOR UPDATE OF c`, challengeID, phone).Scan(&storedHash, &userID, &expires, &role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return "", User{}, ErrInvalidChallenge
	}
	if err != nil {
		return "", User{}, err
	}
	if !active || time.Now().After(expires) {
		return "", User{}, ErrInvalidChallenge
	}
	if !s.matchesCode(challengeID, code, storedHash) {
		if _, err := tx.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET attempts = attempts + 1, consumed_at = CASE WHEN attempts + 1 >= 5 THEN now() ELSE consumed_at END WHERE id = $1`, challengeID); err != nil {
			return "", User{}, err
		}
		if err := tx.Commit(); err != nil {
			return "", User{}, err
		}
		return "", User{}, ErrInvalidChallenge
	}
	if _, err := tx.ExecContext(ctx, `UPDATE warehouse.otp_challenges SET consumed_at = now() WHERE id = $1`, challengeID); err != nil {
		return "", User{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", User{}, err
	}
	user := User{ID: userID, Phone: phone, Role: role}
	token, err := s.issueToken(user)
	return token, user, err
}

func (s *Service) issueToken(user User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": user.ID, "phone": user.Phone, "role": user.Role,
		"iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix(),
		"jti": uuid.NewString(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *Service) ParseToken(tokenString string) (User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidCredentials
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return User{}, ErrInvalidCredentials
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	id, _ := claims.GetSubject()
	phone, _ := claims["phone"].(string)
	role, _ := claims["role"].(string)
	if id == "" || phone == "" || (role != "admin" && role != "staff") {
		return User{}, ErrInvalidCredentials
	}
	return User{ID: id, Phone: phone, Role: role}, nil
}

func (s *Service) hashCode(challengeID, code string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(challengeID + ":" + code))
	return mac.Sum(nil)
}

func (s *Service) matchesCode(challengeID, code, storedHex string) bool {
	stored, err := hex.DecodeString(storedHex)
	if err != nil {
		return false
	}
	return hmac.Equal(s.hashCode(challengeID, strings.TrimSpace(code)), stored)
}

func generateOTP() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func normalizePhone(phone string) string {
	return strings.TrimSpace(strings.ReplaceAll(phone, " ", ""))
}

func maskPhone(phone string) string {
	if len(phone) < 6 {
		return "***"
	}
	return phone[:4] + strings.Repeat("*", len(phone)-7) + phone[len(phone)-3:]
}
