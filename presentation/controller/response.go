package controller

import (
	"database/sql"
	"errors"
	"log"

	"warehouse-service/domain/repository"
	"warehouse-service/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

func writeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidCredentials):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, usecase.ErrInvalidInput):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrAlreadyReviewed):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, sql.ErrNoRows):
		return fiber.NewError(fiber.StatusNotFound, "record not found")
	default:
		var postgresError *pq.Error
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			return fiber.NewError(fiber.StatusConflict, "SKU already exists")
		}
		if _, ok := err.(*fiber.Error); ok {
			return err
		}
		log.Printf("request failed: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "request failed")
	}
}
