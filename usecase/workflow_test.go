package usecase

import (
	"context"
	"testing"

	"warehouse-service/domain/entity"
)

func TestWorkflowRejectsInvalidInputsBeforeRepositoryCalls(t *testing.T) {
	workflow := NewWorkflow(nil, nil)
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

type testImageUploader struct {
	url  string
	data string
}

func (u *testImageUploader) UploadImage(_ context.Context, data string) (string, error) {
	u.data = data
	return u.url, nil
}

type testWorkflowRepository struct {
	input entity.SKURequestInput
}

func (r *testWorkflowRepository) ListScans(context.Context, int64, bool) ([]entity.InventoryScan, error) {
	return nil, nil
}

func (r *testWorkflowRepository) CreateScan(context.Context, entity.InventoryScanInput, int64) (entity.InventoryScan, error) {
	return entity.InventoryScan{}, nil
}

func (r *testWorkflowRepository) ListSKURequests(context.Context, int64, bool) ([]entity.SKURequest, error) {
	return nil, nil
}

func (r *testWorkflowRepository) CreateSKURequest(_ context.Context, input entity.SKURequestInput, _ int64) (entity.SKURequest, error) {
	r.input = input
	return entity.SKURequest{Photo: input.Photo}, nil
}

func (r *testWorkflowRepository) ReviewSKURequest(context.Context, int64, entity.SKURequestReview, int64) (entity.SKURequest, error) {
	return entity.SKURequest{}, nil
}

func TestCreateSKURequestUploadsPhotoAndPersistsSecureURL(t *testing.T) {
	const photo = "data:image/png;base64,iVBORw0KGgo="
	repo := &testWorkflowRepository{}
	uploader := &testImageUploader{url: "https://res.cloudinary.com/test/image/upload/photo.png"}
	workflow := NewWorkflow(repo, uploader)

	request, err := workflow.CreateSKURequest(context.Background(), entity.SKURequestInput{
		SKU: "SKU-1", Name: "Item", Photo: photo,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if uploader.data != photo {
		t.Fatal("expected source image to be uploaded")
	}
	if repo.input.Photo != uploader.url || request.Photo != uploader.url {
		t.Fatalf("photo URL not persisted: repository=%q response=%q", repo.input.Photo, request.Photo)
	}
}

func TestCreateSKURequestRejectsInvalidImageBeforeUpload(t *testing.T) {
	uploader := &testImageUploader{url: "https://res.cloudinary.com/test/image/upload/photo.png"}
	workflow := NewWorkflow(&testWorkflowRepository{}, uploader)

	_, err := workflow.CreateSKURequest(context.Background(), entity.SKURequestInput{
		SKU: "SKU-1", Name: "Item", Photo: "data:image/png;base64,YmFk",
	}, 1)
	if err == nil || uploader.data != "" {
		t.Fatal("expected invalid image data to be rejected before upload")
	}
}
