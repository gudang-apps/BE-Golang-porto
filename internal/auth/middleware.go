package auth

import (
	"strings"

	"warehouse-service/domain/entity"
	"warehouse-service/infrastructure/security"

	"github.com/gofiber/fiber/v2"
)

type TokenParser interface {
	Parse(string) (entity.User, error)
}

func Require(service TokenParser) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.Fields(c.Get(fiber.HeaderAuthorization))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			return fiber.NewError(fiber.StatusUnauthorized, "bearer token is required")
		}
		user, err := service.Parse(header[1])
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, security.ErrInvalidToken.Error())
		}
		c.Locals("user", user)
		return c.Next()
	}
}

func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(entity.User)
		if !ok || !user.HasRole("owner") {
			return fiber.NewError(fiber.StatusForbidden, "owner role is required")
		}
		return c.Next()
	}
}

func CurrentUser(c *fiber.Ctx) (entity.User, bool) {
	user, ok := c.Locals("user").(entity.User)
	return user, ok
}
