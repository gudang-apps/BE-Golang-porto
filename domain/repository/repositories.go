package repository

import (
	"context"

	"warehouse-service/domain/entity"
)

type AuthRepository interface {
	FindByEmail(context.Context, string) (entity.User, string, error)
}

type ProductRepository interface {
	ListProducts(context.Context, string) ([]entity.Product, error)
	GetProduct(context.Context, int64) (entity.Product, error)
	CreateProduct(context.Context, entity.ProductInput, int64) (entity.Product, error)
	UpdateProduct(context.Context, int64, entity.ProductInput) (entity.Product, error)
	DeleteProduct(context.Context, int64) error
}

type SalesRepository interface {
	ListSales(context.Context, entity.SalesFilter, int) ([]entity.Sale, error)
	CreateSale(context.Context, entity.SaleInput, int64) (entity.Sale, error)
	SalesSummary(context.Context, entity.SalesFilter) (entity.SalesSummary, error)
}

type WorkflowRepository interface {
	ListScans(context.Context, int64, bool) ([]entity.InventoryScan, error)
	CreateScan(context.Context, entity.InventoryScanInput, int64) (entity.InventoryScan, error)
	ListSKURequests(context.Context, int64, bool) ([]entity.SKURequest, error)
	CreateSKURequest(context.Context, entity.SKURequestInput, int64) (entity.SKURequest, error)
	ReviewSKURequest(context.Context, int64, entity.SKURequestReview, int64) (entity.SKURequest, error)
}
