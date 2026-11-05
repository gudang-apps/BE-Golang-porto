package server

import (
	"database/sql"
	"log"
	"strconv"
	"time"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/config"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Server struct {
	App  *fiber.App
	db   *sql.DB
	cfg  config.Config
	auth *auth.Service
}

func New(cfg config.Config, db *sql.DB, authService *auth.Service) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             10 * 1024 * 1024,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if fiberErr, ok := err.(*fiber.Error); ok {
				code = fiberErr.Code
			}
			if code >= 500 {
				log.Printf("request failed: %v", err)
				return c.Status(code).JSON(fiber.Map{"success": false, "error": "internal server error"})
			}
			return c.Status(code).JSON(fiber.Map{"success": false, "error": err.Error()})
		},
	})
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigin,
		AllowHeaders: "Accept, Authorization, Content-Type, Origin",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))
	server := &Server{
		App:  app,
		db:   db,
		cfg:  cfg,
		auth: authService,
	}
	server.registerRoutes()
	return server
}

func (s *Server) registerRoutes() {
	s.App.Get("/healthz", func(c *fiber.Ctx) error {
		if err := s.db.PingContext(c.UserContext()); err != nil {
			return fiber.NewError(fiber.StatusServiceUnavailable, "database unavailable")
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"status": "ok"}})
	})

	authGroup := s.App.Group("/api/v1/auth")
	authGroup.Post("/login", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), s.login)
}

func (s *Server) Start() error {
	return s.App.Listen(":" + strconv.Itoa(s.cfg.Port))
}

func (s *Server) Shutdown() error {
	return s.App.Shutdown()
}
