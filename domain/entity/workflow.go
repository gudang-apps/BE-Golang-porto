package entity

import "time"

type InventoryScan struct {
	ID          int64     `json:"id"`
	Barcode     string    `json:"barcode"`
	ProductName string    `json:"product_name"`
	Quantity    int       `json:"quantity"`
	UserID      int64     `json:"user_id"`
	UserName    string    `json:"user_name"`
	ScannedAt   time.Time `json:"scanned_at"`
}

type InventoryScanInput struct {
	Barcode  string `json:"barcode"`
	Quantity int    `json:"quantity"`
}

type SKURequest struct {
	ID          int64      `json:"id"`
	SKU         string     `json:"sku"`
	Name        string     `json:"name"`
	CostPrice   int64      `json:"cost_price"`
	SellPrice   int64      `json:"sell_price"`
	Stock       int        `json:"stock"`
	Notes       string     `json:"notes"`
	Photo       string     `json:"photo"`
	Status      string     `json:"status"`
	SubmittedBy int64      `json:"submitted_by"`
	SubmittedAs string     `json:"submitted_by_name"`
	ReviewedBy  *int64     `json:"reviewed_by,omitempty"`
	ReviewNote  string     `json:"review_note"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
}

type SKURequestInput struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	CostPrice int64  `json:"cost_price"`
	SellPrice int64  `json:"sell_price"`
	Stock     int    `json:"stock"`
	Notes     string `json:"notes"`
	Photo     string `json:"photo"`
}

type SKURequestReview struct {
	Status     string `json:"status"`
	ReviewNote string `json:"review_note"`
}
