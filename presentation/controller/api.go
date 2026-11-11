package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"warehouse-service/domain/entity"
	"warehouse-service/infrastructure/security"
	"warehouse-service/internal/auth"
	"warehouse-service/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
)

type API struct {
	auth     *usecase.Auth
	catalog  *usecase.Catalog
	sales    *usecase.Sales
	workflow *usecase.Workflow
	tokens   *security.JWT
}

func New(authUsecase *usecase.Auth, catalog *usecase.Catalog, salesUsecase *usecase.Sales, workflow *usecase.Workflow, tokens *security.JWT) *API {
	return &API{auth: authUsecase, catalog: catalog, sales: salesUsecase, workflow: workflow, tokens: tokens}
}

func (a *API) Register(app *fiber.App) {
	app.Post("/api/v1/auth/login", a.login)
	protected := app.Group("/api/v1", auth.Require(a.tokens))
	protected.Get("/auth/me", a.me)
	protected.Get("/products", a.listProducts)
	protected.Get("/products/:id", a.getProduct)
	protected.Get("/sales", a.listSales)
	protected.Get("/sales/summary", a.salesSummary)
	protected.Get("/scans", a.listScans)
	protected.Post("/scans", a.createScan)
	protected.Get("/sku-requests", a.listSKURequests)
	protected.Post("/sku-requests", a.createSKURequest)

	owner := protected.Group("", auth.RequireAdmin())
	owner.Post("/products", a.createProduct)
	owner.Put("/products/:id", a.updateProduct)
	owner.Delete("/products/:id", a.deleteProduct)
	owner.Post("/sales", a.createSale)
	owner.Patch("/sku-requests/:id/review", a.reviewSKURequest)
}

func (a *API) login(c *fiber.Ctx) error {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	token, user, err := a.auth.Login(c.UserContext(), request.Email, request.Password)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   28800,
			"user":         user,
		},
	})
}

func (a *API) me(c *fiber.Ctx) error {
	user, ok := auth.CurrentUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	return c.JSON(fiber.Map{"success": true, "data": user})
}

func (a *API) listProducts(c *fiber.Ctx) error {
	products, err := a.catalog.ListProducts(c.UserContext(), c.Query("search"))
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": products})
}

func (a *API) getProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	product, err := a.catalog.GetProduct(c.UserContext(), id)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": product})
}

func (a *API) createProduct(c *fiber.Ctx) error {
	var input entity.ProductInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, _ := auth.CurrentUser(c)
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	product, err := a.catalog.CreateProduct(c.UserContext(), input, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": product})
}

func (a *API) updateProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var input entity.ProductInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	product, err := a.catalog.UpdateProduct(c.UserContext(), id, input)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": product})
}

func (a *API) deleteProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := a.catalog.DeleteProduct(c.UserContext(), id); err != nil {
		if isForeignKeyViolation(err) {
			return fiber.NewError(fiber.StatusConflict, "product has sales or stock history")
		}
		return writeError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *API) listSales(c *fiber.Ctx) error {
	filter, err := parseSalesFilter(c)
	if err != nil {
		return err
	}
	sales, err := a.sales.List(c.UserContext(), filter, 5000)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sales})
}

func (a *API) salesSummary(c *fiber.Ctx) error {
	filter, err := parseSalesFilter(c)
	if err != nil {
		return err
	}
	summary, err := a.sales.Summary(c.UserContext(), filter)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": summary})
}

func (a *API) createSale(c *fiber.Ctx) error {
	var input entity.SaleInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, _ := auth.CurrentUser(c)
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	sale, err := a.sales.Create(c.UserContext(), input, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": sale})
}

func (a *API) listScans(c *fiber.Ctx) error {
	user, ok := auth.CurrentUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	scans, err := a.workflow.ListScans(c.UserContext(), userID, user.HasRole("owner"))
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": scans})
}

func (a *API) createScan(c *fiber.Ctx) error {
	var input entity.InventoryScanInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, _ := auth.CurrentUser(c)
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	scan, err := a.workflow.CreateScan(c.UserContext(), input, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": scan})
}

func (a *API) listSKURequests(c *fiber.Ctx) error {
	user, ok := auth.CurrentUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	requests, err := a.workflow.ListSKURequests(c.UserContext(), userID, user.HasRole("owner"))
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": requests})
}

func (a *API) createSKURequest(c *fiber.Ctx) error {
	var input entity.SKURequestInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, _ := auth.CurrentUser(c)
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	request, err := a.workflow.CreateSKURequest(c.UserContext(), input, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": request})
}

func (a *API) reviewSKURequest(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var review entity.SKURequestReview
	if err := c.BodyParser(&review); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	user, _ := auth.CurrentUser(c)
	userID, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid account identity")
	}
	request, err := a.workflow.ReviewSKURequest(c.UserContext(), id, review, userID)
	if err != nil {
		return writeError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": request})
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "id must be a positive integer")
	}
	return id, nil
}

func parseSalesFilter(c *fiber.Ctx) (entity.SalesFilter, error) {
	filter := entity.SalesFilter{Platform: strings.TrimSpace(c.Query("platform"))}
	if value := c.Query("year"); value != "" {
		year, err := strconv.Atoi(value)
		if err != nil || year < 2000 || year > 2200 {
			return entity.SalesFilter{}, fiber.NewError(fiber.StatusBadRequest, "year must be between 2000 and 2200")
		}
		filter.Year = year
	}
	var err error
	if value := c.Query("from"); value != "" {
		filter.From, err = time.Parse("2006-01-02", value)
		if err != nil {
			return entity.SalesFilter{}, fiber.NewError(fiber.StatusBadRequest, "from must use YYYY-MM-DD")
		}
	}
	if value := c.Query("to"); value != "" {
		filter.To, err = time.Parse("2006-01-02", value)
		if err != nil {
			return entity.SalesFilter{}, fiber.NewError(fiber.StatusBadRequest, "to must use YYYY-MM-DD")
		}
		filter.To = filter.To.AddDate(0, 0, 1)
	}
	if !filter.From.IsZero() && !filter.To.IsZero() && !filter.From.Before(filter.To) {
		return entity.SalesFilter{}, fiber.NewError(fiber.StatusBadRequest, "from must be on or before to")
	}
	if filter.Platform != "" {
		switch filter.Platform {
		case "Instagram", "WhatsApp", "Shopee", "Tokopedia", "Offline", "Lainnya":
		default:
			return entity.SalesFilter{}, fiber.NewError(fiber.StatusBadRequest, "invalid sales platform")
		}
	}
	return filter, nil
}

func isForeignKeyViolation(err error) bool {
	var postgresError *pq.Error
	return errors.As(err, &postgresError) && postgresError.Code == "23503"
}
