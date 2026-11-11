package auth

import (
	"database/sql"
	"net/mail"
	"strings"

	"warehouse-service/domain/entity"
	"warehouse-service/infrastructure/security"
)

type Service struct {
	tokens *security.JWT
}

func NewService(_ *sql.DB, secret []byte) *Service {
	return &Service{tokens: security.NewJWT(secret)}
}

func (s *Service) Parse(token string) (entity.User, error) {
	return s.tokens.Parse(token)
}

func (s *Service) ParseToken(token string) (entity.User, error) {
	return s.Parse(token)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}
