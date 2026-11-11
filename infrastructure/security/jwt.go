package security

import (
	"errors"
	"time"

	"warehouse-service/domain/entity"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid or expired access token")

type JWT struct {
	secret []byte
}

func NewJWT(secret []byte) *JWT {
	return &JWT{secret: secret}
}

func (j *JWT) Issue(user entity.User) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": user.ID, "email": user.Email, "name": user.Name, "roles": user.Roles,
		"iat": now.Unix(), "exp": now.Add(8 * time.Hour).Unix(), "jti": uuid.NewString(),
	}).SignedString(j.secret)
}

func (j *JWT) Parse(tokenString string) (entity.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return j.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return entity.User{}, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return entity.User{}, ErrInvalidToken
	}
	id, _ := claims.GetSubject()
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	rawRoles, ok := claims["roles"].([]any)
	if !ok || id == "" || email == "" || name == "" || len(rawRoles) == 0 {
		return entity.User{}, ErrInvalidToken
	}
	user := entity.User{ID: id, Name: name, Email: email}
	for _, rawRole := range rawRoles {
		role, ok := rawRole.(string)
		if !ok || (role != "owner" && role != "staff") {
			return entity.User{}, ErrInvalidToken
		}
		user.Roles = append(user.Roles, role)
	}
	return user, nil
}
