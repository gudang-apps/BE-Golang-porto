package entity

import "time"

type Sale struct {
	ID         int64     `json:"id"`
	ProductID  int64     `json:"product_id"`
	ProductSKU string    `json:"product_sku"`
	Product    string    `json:"product_name"`
	UserID     int64     `json:"user_id"`
	UserName   string    `json:"user_name"`
	Platform   string    `json:"platform"`
	Qty        int       `json:"qty"`
	FinalPrice int64     `json:"final_price"`
	ExtraCost  int64     `json:"extra_cost"`
	TotalModal int64     `json:"total_modal"`
	Omzet      int64     `json:"omzet"`
	Profit     int64     `json:"profit"`
	SoldAt     time.Time `json:"sold_at"`
}

type SaleInput struct {
	ProductID  int64  `json:"product_id"`
	Platform   string `json:"platform"`
	Qty        int    `json:"qty"`
	FinalPrice int64  `json:"final_price"`
	ExtraCost  int64  `json:"extra_cost"`
}

type SalesFilter struct {
	Year     int
	From     time.Time
	To       time.Time
	Platform string
}

type SalesSummary struct {
	Count     int64            `json:"count"`
	Omzet     int64            `json:"omzet"`
	Profit    int64            `json:"profit"`
	TotalCost int64            `json:"total_cost"`
	ByMonth   []MonthlySales   `json:"by_month"`
	ByChannel []ChannelSales   `json:"by_channel"`
}

type MonthlySales struct {
	Month int   `json:"month"`
	Omzet int64 `json:"omzet"`
	Profit int64 `json:"profit"`
	Count int64 `json:"count"`
}

type ChannelSales struct {
	Platform string `json:"platform"`
	Omzet    int64  `json:"omzet"`
	Profit   int64  `json:"profit"`
	Count    int64  `json:"count"`
}
