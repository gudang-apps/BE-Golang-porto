package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Require(service *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.Fields(c.Get(fiber.HeaderAuthorization))
		if len(header) != 2 || !strings.EqualFold(header[0], "Bearer") {
			return fiber.NewError(fiber.StatusUnauthorized, "bearer token is required")
		}
		user, err := service.ParseToken(header[1])
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
		}
		c.Locals("user", user)
		return c.Next()
	}
}

func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(User)
		if !ok || user.Role != "admin" {
			return fiber.NewError(fiber.StatusForbidden, "admin role is required")
		}
		return c.Next()
	}
}

func CurrentUser(c *fiber.Ctx) (User, bool) {
	user, ok := c.Locals("user").(User)
	return user, ok
}
