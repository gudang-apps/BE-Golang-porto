package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"warehouse-service/domain/entity"
	"warehouse-service/domain/repository"
)

func (r *Repositories) ListScans(ctx context.Context, userID int64, isOwner bool) ([]entity.InventoryScan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.barcode, COALESCE(p.name, ''), s.quantity, s.user_id, u.name, s.scanned_at
		FROM web_db.inventory_scans s
		JOIN users.users u ON u.id = s.user_id
		LEFT JOIN web_db.products p ON p.sku = s.barcode
		WHERE ($2 OR s.user_id = $1)
		ORDER BY s.scanned_at DESC, s.id DESC
		LIMIT 200`, userID, isOwner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scans := make([]entity.InventoryScan, 0)
	for rows.Next() {
		var scan entity.InventoryScan
		if err := rows.Scan(&scan.ID, &scan.Barcode, &scan.ProductName, &scan.Quantity, &scan.UserID, &scan.UserName, &scan.ScannedAt); err != nil {
			return nil, err
		}
		scans = append(scans, scan)
	}
	return scans, rows.Err()
}

func (r *Repositories) CreateScan(ctx context.Context, input entity.InventoryScanInput, userID int64) (entity.InventoryScan, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO web_db.inventory_scans (barcode, quantity, user_id)
		VALUES ($1, $2, $3)
		RETURNING id`, input.Barcode, input.Quantity, userID).Scan(&id)
	if err != nil {
		return entity.InventoryScan{}, err
	}
	var scan entity.InventoryScan
	err = r.db.QueryRowContext(ctx, `
		SELECT s.id, s.barcode, COALESCE(p.name, ''), s.quantity, s.user_id, u.name, s.scanned_at
		FROM web_db.inventory_scans s
		JOIN users.users u ON u.id = s.user_id
		LEFT JOIN web_db.products p ON p.sku = s.barcode
		WHERE s.id = $1`, id).Scan(&scan.ID, &scan.Barcode, &scan.ProductName, &scan.Quantity, &scan.UserID, &scan.UserName, &scan.ScannedAt)
	return scan, err
}

const skuRequestColumns = `
	r.id, r.sku, r.name, r.cost_price, r.sell_price, r.stock, r.notes, r.photo,
	r.status, r.submitted_by, submitter.name, r.reviewed_by, r.review_note,
	r.submitted_at, r.reviewed_at`

func (r *Repositories) ListSKURequests(ctx context.Context, userID int64, isOwner bool) ([]entity.SKURequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+skuRequestColumns+`
		FROM web_db.sku_requests r
		JOIN users.users submitter ON submitter.id = r.submitted_by
		WHERE ($2 OR r.submitted_by = $1)
		ORDER BY CASE WHEN r.status = 'pending' THEN 0 ELSE 1 END, r.submitted_at DESC, r.id DESC
		LIMIT 500`, userID, isOwner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]entity.SKURequest, 0)
	for rows.Next() {
		request, err := scanSKURequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *Repositories) CreateSKURequest(ctx context.Context, input entity.SKURequestInput, userID int64) (entity.SKURequest, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO web_db.sku_requests (sku, name, cost_price, sell_price, stock, notes, photo, submitted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`, input.SKU, input.Name, input.CostPrice, input.SellPrice, input.Stock, input.Notes, input.Photo, userID).Scan(&id)
	if err != nil {
		return entity.SKURequest{}, err
	}
	return r.getSKURequest(ctx, r.db, id)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r *Repositories) getSKURequest(ctx context.Context, queryer queryRower, id int64) (entity.SKURequest, error) {
	request, err := scanSKURequest(queryer.QueryRowContext(ctx, `
		SELECT `+skuRequestColumns+`
		FROM web_db.sku_requests r
		JOIN users.users submitter ON submitter.id = r.submitted_by
		WHERE r.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SKURequest{}, repository.ErrNotFound
	}
	return request, err
}

type nullableInt64Scanner interface {
	Scan(...any) error
}

func scanSKURequest(row nullableInt64Scanner) (entity.SKURequest, error) {
	var request entity.SKURequest
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	err := row.Scan(
		&request.ID, &request.SKU, &request.Name, &request.CostPrice, &request.SellPrice,
		&request.Stock, &request.Notes, &request.Photo, &request.Status, &request.SubmittedBy,
		&request.SubmittedAs, &reviewedBy, &request.ReviewNote, &request.SubmittedAt, &reviewedAt,
	)
	if err != nil {
		return entity.SKURequest{}, err
	}
	if reviewedBy.Valid {
		request.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		request.ReviewedAt = &reviewedAt.Time
	}
	return request, nil
}

func (r *Repositories) ReviewSKURequest(ctx context.Context, id int64, review entity.SKURequestReview, reviewerID int64) (entity.SKURequest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.SKURequest{}, err
	}
	defer tx.Rollback()

	var request entity.SKURequest
	err = tx.QueryRowContext(ctx, `
		SELECT id, sku, name, cost_price, sell_price, stock, notes, photo, status
		FROM web_db.sku_requests
		WHERE id = $1
		FOR UPDATE`, id).Scan(
		&request.ID, &request.SKU, &request.Name, &request.CostPrice, &request.SellPrice,
		&request.Stock, &request.Notes, &request.Photo, &request.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SKURequest{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.SKURequest{}, err
	}
	if request.Status != "pending" {
		return entity.SKURequest{}, repository.ErrAlreadyReviewed
	}
	if review.Status == "approved" {
		var productID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO web_db.products (sku, name, cost_price, sell_price, stock, notes, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`, request.SKU, request.Name, request.CostPrice, request.SellPrice, request.Stock, request.Notes, reviewerID).Scan(&productID)
		if err != nil {
			return entity.SKURequest{}, fmt.Errorf("create approved product: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO web_db.product_photos (product_id, url, sort_order)
			VALUES ($1, $2, 0)`, productID, request.Photo); err != nil {
			return entity.SKURequest{}, fmt.Errorf("save approved product photo: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE web_db.sku_requests
		SET status = $2, reviewed_by = $3, review_note = $4, reviewed_at = now()
		WHERE id = $1`, id, review.Status, reviewerID, review.ReviewNote); err != nil {
		return entity.SKURequest{}, err
	}
	request, err = r.getSKURequest(ctx, tx, id)
	if err != nil {
		return entity.SKURequest{}, err
	}
	if err := tx.Commit(); err != nil {
		return entity.SKURequest{}, err
	}
	return request, nil
}
