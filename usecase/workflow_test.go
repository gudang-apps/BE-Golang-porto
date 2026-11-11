package usecase

import (
	"context"
	"testing"

	"warehouse-service/domain/entity"
)

func TestWorkflowRejectsInvalidInputsBeforeRepositoryCalls(t *testing.T) {
	workflow := NewWorkflow(nil)
	if _, err := workflow.CreateScan(context.Background(), entity.InventoryScanInput{}, 1); err == nil {
		t.Fatal("expected empty barcode to be rejected")
	}
	if _, err := workflow.CreateSKURequest(context.Background(), entity.SKURequestInput{
		SKU: "SKU-1", Name: "Item",
	}, 1); err == nil {
		t.Fatal("expected a missing SKU photo to be rejected")
	}
	if _, err := workflow.ReviewSKURequest(context.Background(), 1, entity.SKURequestReview{
		Status: "pending",
	}, 1); err == nil {
		t.Fatal("expected an unsupported review status to be rejected")
	}
}
