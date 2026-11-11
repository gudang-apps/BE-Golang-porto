package entity

import "time"

type Product struct {
	ID        int64     `json:"id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	CostPrice int64     `json:"cost_price"`
	SellPrice int64     `json:"sell_price"`
	Stock     int       `json:"stock"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes"`
	CreatedBy int64     `json:"created_by"`
	Photos    []string  `json:"photos"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProductInput struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	CostPrice int64  `json:"cost_price"`
	SellPrice int64  `json:"sell_price"`
	Stock     int    `json:"stock"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
}
