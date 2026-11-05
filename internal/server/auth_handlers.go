package server

import (
	"errors"
	"log"

	"warehouse-service/internal/auth"

	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(c *fiber.Ctx) error {
	var request loginRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	token, user, err := s.auth.Login(c.UserContext(), request.Email, request.Password)
	if err != nil {
		return authError(err)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   900,
			"user":         user,
		},
	})
}

func authError(err error) error {
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	}
	log.Printf("authentication request failed: %v", err)
	return fiber.NewError(fiber.StatusInternalServerError, "authentication request failed")
}
