package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type Service struct {
	db     *sql.DB
	secret []byte
}

func NewService(db *sql.DB, secret []byte) *Service {
	return &Service{db: db, secret: secret}
}

type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) || password == "" {
		return "", User{}, ErrInvalidCredentials
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.name, u.email, u.password_hash, r.name
		FROM users.users u
		JOIN users.user_has_roles uhr ON uhr.user_id = u.id
		JOIN users.roles r ON r.id = uhr.role_id
		WHERE lower(u.email) = $1
		ORDER BY r.name`, email)
	if err != nil {
		return "", User{}, fmt.Errorf("lookup account: %w", err)
	}
	defer rows.Close()

	var user User
	var passwordHash string
	found := false
	for rows.Next() {
		var role string
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &role); err != nil {
			return "", User{}, fmt.Errorf("read account: %w", err)
		}
		user.Roles = append(user.Roles, role)
		found = true
	}
	if err := rows.Err(); err != nil {
		return "", User{}, fmt.Errorf("read account roles: %w", err)
	}
	if !found || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token, err := s.issueToken(user)
	if err != nil {
		return "", User{}, err
	}
	return token, user, nil
}

func (s *Service) issueToken(user User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": user.ID, "email": user.Email, "name": user.Name, "roles": user.Roles,
		"iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix(),
		"jti": uuid.NewString(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return token, nil
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
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	roles := parseRoles(claims["roles"])
	if id == "" || !validEmail(email) || name == "" || len(roles) == 0 {
		return User{}, ErrInvalidCredentials
	}
	return User{ID: id, Name: name, Email: email, Roles: roles}, nil
}

func parseRoles(value any) []string {
	rawRoles, ok := value.([]any)
	if !ok {
		return nil
	}
	roles := make([]string, 0, len(rawRoles))
	for _, value := range rawRoles {
		role, ok := value.(string)
		if !ok || (role != "owner" && role != "staff") {
			return nil
		}
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
