package server

import (
	"errors"
	"log"

	"warehouse-service/internal/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

type registerRequest struct {
	InvitationID string `json:"invitation_id"`
	Phone        string `json:"phone"`
	OTP          string `json:"otp"`
	Password     string `json:"password"`
}

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type verifyOTPRequest struct {
	ChallengeID string `json:"challenge_id"`
	Phone       string `json:"phone"`
	OTP         string `json:"otp"`
}

func (s *Server) invite(c *fiber.Ctx) error {
	var request struct {
		Phone string `json:"phone"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, ok := auth.CurrentUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	invitationID, err := s.auth.InviteUser(c.UserContext(), request.Phone, user.ID)
	if err != nil {
		return authError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"invitation_id": invitationID, "expires_in_seconds": int(s.cfg.OTPTTL.Seconds())},
	})
}

func (s *Server) register(c *fiber.Ctx) error {
	var request registerRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := s.auth.Register(c.UserContext(), request.InvitationID, request.Phone, request.OTP, request.Password); err != nil {
		return authError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": fiber.Map{"registered": true}})
}

func (s *Server) login(c *fiber.Ctx) error {
	var request loginRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	challengeID, err := s.auth.StartLogin(c.UserContext(), request.Phone, request.Password)
	if err != nil {
		return authError(err)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"challenge_id": challengeID, "expires_in_seconds": int(s.cfg.OTPTTL.Seconds())},
	})
}

func (s *Server) verifyLogin(c *fiber.Ctx) error {
	var request verifyOTPRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	token, user, err := s.auth.VerifyLogin(c.UserContext(), request.ChallengeID, request.Phone, request.OTP)
	if err != nil {
		return authError(err)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   900,
			"user":         fiber.Map{"id": user.ID, "phone": user.Phone, "role": user.Role},
		},
	})
}

func authError(err error) error {
	switch {
	case errors.Is(err, auth.ErrAccountExists):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrInvalidInput):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrSenderUnavailable):
		return fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidChallenge), errors.Is(err, auth.ErrInvalidInvite):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case isUniqueViolation(err):
		return fiber.NewError(fiber.StatusConflict, "account already exists")
	default:
		log.Printf("authentication request failed: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "authentication request failed")
	}
}

func isUniqueViolation(err error) bool {
	var postgresError *pq.Error
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
