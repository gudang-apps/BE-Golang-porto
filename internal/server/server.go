package server

import (
	"database/sql"
	"log"
	"strconv"
	"time"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/config"
	"warehouse-service/internal/ocr"

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
	ocr  *ocr.Reader
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
		ocr:  ocr.New(cfg.OCRBinary, cfg.OCRLang),
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
	authGroup.Post("/register", limiter.New(limiter.Config{Max: 5, Expiration: time.Minute}), s.register)
	authGroup.Post("/login", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), s.login)
	authGroup.Post("/otp/verify", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), s.verifyLogin)

	api := s.App.Group("/api/v1", auth.Require(s.auth))
	api.Post("/auth/invitations", auth.RequireAdmin(), s.invite)

	api.Get("/products", s.listProducts)
	api.Post("/products", s.createProduct)
	api.Post("/products/scan", s.scanSKU)
	api.Get("/products/sku/:sku", s.getProductBySKU)
	api.Get("/products/:id", s.getProduct)
	api.Put("/products/:id", s.updateProduct)
	api.Delete("/products/:id", s.deleteProduct)

	api.Get("/warehouses", s.listWarehouses)
	api.Post("/warehouses", s.createWarehouse)
	api.Get("/warehouses/:id", s.getWarehouse)
	api.Put("/warehouses/:id", s.updateWarehouse)
	api.Delete("/warehouses/:id", s.deleteWarehouse)

	api.Get("/suppliers", s.listSuppliers)
	api.Post("/suppliers", s.createSupplier)
	api.Get("/suppliers/:id", s.getSupplier)
	api.Put("/suppliers/:id", s.updateSupplier)
	api.Delete("/suppliers/:id", s.deleteSupplier)

	api.Get("/stock", s.listStock)
	api.Get("/stock/low", s.listLowStock)
	api.Get("/stock-movements", s.listMovements)
	api.Post("/stock-movements/inbound", s.createInbound)
	api.Post("/stock-movements/outbound", s.createOutbound)
	api.Post("/stock-movements/transfer", s.createTransfer)
	api.Post("/stock-movements/adjustment", s.createAdjustment)
}

func (s *Server) Start() error {
	return s.App.Listen(":" + strconv.Itoa(s.cfg.Port))
}

func (s *Server) Shutdown() error {
	return s.App.Shutdown()
}
