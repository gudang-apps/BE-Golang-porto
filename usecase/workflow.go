package usecase

import (
	"context"
	"fmt"
	"strings"

	"warehouse-service/domain/entity"
	"warehouse-service/domain/repository"
)

type Workflow struct {
	repository repository.WorkflowRepository
}

func NewWorkflow(repository repository.WorkflowRepository) *Workflow {
	return &Workflow{repository: repository}
}

func (u *Workflow) ListScans(ctx context.Context, userID int64, isOwner bool) ([]entity.InventoryScan, error) {
	return u.repository.ListScans(ctx, userID, isOwner)
}

func (u *Workflow) CreateScan(ctx context.Context, input entity.InventoryScanInput, userID int64) (entity.InventoryScan, error) {
	input.Barcode = strings.TrimSpace(input.Barcode)
	if input.Barcode == "" || len(input.Barcode) > 160 {
		return entity.InventoryScan{}, fmt.Errorf("%w: barcode is required", ErrInvalidInput)
	}
	if input.Quantity == 0 {
		input.Quantity = 1
	}
	if input.Quantity < 1 || input.Quantity > 10000 {
		return entity.InventoryScan{}, fmt.Errorf("%w: quantity must be between 1 and 10000", ErrInvalidInput)
	}
	return u.repository.CreateScan(ctx, input, userID)
}

func (u *Workflow) ListSKURequests(ctx context.Context, userID int64, isOwner bool) ([]entity.SKURequest, error) {
	return u.repository.ListSKURequests(ctx, userID, isOwner)
}

func (u *Workflow) CreateSKURequest(ctx context.Context, input entity.SKURequestInput, userID int64) (entity.SKURequest, error) {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Name = strings.TrimSpace(input.Name)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.SKU == "" || len(input.SKU) > 100 || input.Name == "" || len(input.Name) > 200 ||
		input.CostPrice < 0 || input.SellPrice < 0 || input.Stock < 0 || len(input.Notes) > 4000 {
		return entity.SKURequest{}, fmt.Errorf("%w: invalid SKU request data", ErrInvalidInput)
	}
	if !strings.HasPrefix(input.Photo, "data:image/jpeg;base64,") &&
		!strings.HasPrefix(input.Photo, "data:image/png;base64,") &&
		!strings.HasPrefix(input.Photo, "data:image/webp;base64,") {
		return entity.SKURequest{}, fmt.Errorf("%w: a JPEG, PNG, or WebP photo is required", ErrInvalidInput)
	}
	if len(input.Photo) > 8*1024*1024 {
		return entity.SKURequest{}, fmt.Errorf("%w: photo is too large", ErrInvalidInput)
	}
	return u.repository.CreateSKURequest(ctx, input, userID)
}

func (u *Workflow) ReviewSKURequest(ctx context.Context, id int64, review entity.SKURequestReview, reviewerID int64) (entity.SKURequest, error) {
	review.Status = strings.ToLower(strings.TrimSpace(review.Status))
	review.ReviewNote = strings.TrimSpace(review.ReviewNote)
	if (review.Status != "approved" && review.Status != "rejected") || len(review.ReviewNote) > 1000 {
		return entity.SKURequest{}, fmt.Errorf("%w: status must be approved or rejected", ErrInvalidInput)
	}
	return u.repository.ReviewSKURequest(ctx, id, review, reviewerID)
}
