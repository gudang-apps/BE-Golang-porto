package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"warehouse-service/domain/entity"
	"warehouse-service/domain/repository"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrInvalidInput = errors.New("invalid request data")

type TokenIssuer interface {
	Issue(entity.User) (string, error)
}

type Auth struct {
	repository repository.AuthRepository
	tokens     TokenIssuer
}

func NewAuth(repository repository.AuthRepository, tokens TokenIssuer) *Auth {
	return &Auth{repository: repository, tokens: tokens}
}

func (u *Auth) Login(ctx context.Context, email, password string) (string, entity.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || password == "" {
		return "", entity.User{}, ErrInvalidCredentials
	}
	user, passwordHash, err := u.repository.FindByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		return "", entity.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", entity.User{}, fmt.Errorf("find login account: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil || len(user.Roles) == 0 {
		return "", entity.User{}, ErrInvalidCredentials
	}
	token, err := u.tokens.Issue(user)
	if err != nil {
		return "", entity.User{}, fmt.Errorf("issue access token: %w", err)
	}
	return token, user, nil
}

type Catalog struct {
	products repository.ProductRepository
}

func NewCatalog(products repository.ProductRepository) *Catalog {
	return &Catalog{products: products}
}

func (u *Catalog) ListProducts(ctx context.Context, search string) ([]entity.Product, error) {
	return u.products.ListProducts(ctx, strings.TrimSpace(search))
}

func (u *Catalog) GetProduct(ctx context.Context, id int64) (entity.Product, error) {
	return u.products.GetProduct(ctx, id)
}

func (u *Catalog) CreateProduct(ctx context.Context, input entity.ProductInput, userID int64) (entity.Product, error) {
	if err := validateProduct(input); err != nil {
		return entity.Product{}, err
	}
	return u.products.CreateProduct(ctx, normalizeProduct(input), userID)
}

func (u *Catalog) UpdateProduct(ctx context.Context, id int64, input entity.ProductInput) (entity.Product, error) {
	if err := validateProduct(input); err != nil {
		return entity.Product{}, err
	}
	return u.products.UpdateProduct(ctx, id, normalizeProduct(input))
}

func (u *Catalog) DeleteProduct(ctx context.Context, id int64) error {
	return u.products.DeleteProduct(ctx, id)
}

func normalizeProduct(input entity.ProductInput) entity.ProductInput {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Name = strings.TrimSpace(input.Name)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Status == "" {
		input.Status = "Ready"
	}
	return input
}

func validateProduct(input entity.ProductInput) error {
	input = normalizeProduct(input)
	if input.SKU == "" || len(input.SKU) > 100 || input.Name == "" || len(input.Name) > 200 ||
		input.CostPrice < 0 || input.SellPrice < 0 || input.Stock < 0 || len(input.Notes) > 4000 {
		return fmt.Errorf("%w: invalid product data", ErrInvalidInput)
	}
	switch input.Status {
	case "Ready", "Sold", "Rusak", "Retur":
		return nil
	default:
		return fmt.Errorf("%w: invalid product status", ErrInvalidInput)
	}
}

type Sales struct {
	repository repository.SalesRepository
}

func NewSales(repository repository.SalesRepository) *Sales {
	return &Sales{repository: repository}
}

func (u *Sales) List(ctx context.Context, filter entity.SalesFilter, limit int) ([]entity.Sale, error) {
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	return u.repository.ListSales(ctx, filter, limit)
}

func (u *Sales) Create(ctx context.Context, input entity.SaleInput, userID int64) (entity.Sale, error) {
	if input.ProductID < 1 || input.Qty < 1 || input.Qty > 100000 || input.FinalPrice < 0 || input.ExtraCost < 0 {
		return entity.Sale{}, fmt.Errorf("%w: invalid sale data", ErrInvalidInput)
	}
	switch input.Platform {
	case "Instagram", "WhatsApp", "Shopee", "Tokopedia", "Offline", "Lainnya":
	default:
		return entity.Sale{}, fmt.Errorf("%w: invalid sales platform", ErrInvalidInput)
	}
	return u.repository.CreateSale(ctx, input, userID)
}

func (u *Sales) Summary(ctx context.Context, filter entity.SalesFilter) (entity.SalesSummary, error) {
	return u.repository.SalesSummary(ctx, filter)
}
