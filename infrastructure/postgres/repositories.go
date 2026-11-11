package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"warehouse-service/domain/entity"
	"warehouse-service/domain/repository"

	"github.com/lib/pq"
)

type Repositories struct {
	db *sql.DB
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{db: db}
}

var _ repository.AuthRepository = (*Repositories)(nil)
var _ repository.ProductRepository = (*Repositories)(nil)
var _ repository.SalesRepository = (*Repositories)(nil)

func (r *Repositories) FindByEmail(ctx context.Context, email string) (entity.User, string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.name, u.email, u.password_hash, r.name
		FROM users.users u
		JOIN users.user_has_roles uhr ON uhr.user_id = u.id
		JOIN users.roles r ON r.id = uhr.role_id
		WHERE u.email = $1
		ORDER BY r.name`, email)
	if err != nil {
		return entity.User{}, "", err
	}
	defer rows.Close()
	var user entity.User
	var passwordHash string
	found := false
	for rows.Next() {
		var role string
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &role); err != nil {
			return entity.User{}, "", err
		}
		user.Roles = append(user.Roles, role)
		found = true
	}
	if err := rows.Err(); err != nil {
		return entity.User{}, "", err
	}
	if !found {
		return entity.User{}, "", repository.ErrNotFound
	}
	return user, passwordHash, nil
}

const productColumns = `
	p.id, p.sku, p.name, p.cost_price, p.sell_price, p.stock, p.status, p.notes,
	p.created_by, p.created_at, p.updated_at,
	COALESCE(array_agg(ph.url ORDER BY ph.sort_order) FILTER (WHERE ph.url IS NOT NULL), '{}')`

func (r *Repositories) ListProducts(ctx context.Context, search string) ([]entity.Product, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+productColumns+`
		FROM web_db.products p
		LEFT JOIN web_db.product_photos ph ON ph.product_id = p.id
		WHERE ($1 = '' OR p.sku ILIKE '%' || $1 || '%' OR p.name ILIKE '%' || $1 || '%')
		GROUP BY p.id
		ORDER BY p.updated_at DESC, p.id DESC
		LIMIT 1000`, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]entity.Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (r *Repositories) GetProduct(ctx context.Context, id int64) (entity.Product, error) {
	product, err := scanProduct(r.db.QueryRowContext(ctx, `
		SELECT `+productColumns+`
		FROM web_db.products p
		LEFT JOIN web_db.product_photos ph ON ph.product_id = p.id
		WHERE p.id = $1
		GROUP BY p.id`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Product{}, repository.ErrNotFound
	}
	return product, err
}

type rowScanner interface {
	Scan(...any) error
}

func scanProduct(row rowScanner) (entity.Product, error) {
	var product entity.Product
	var photos pq.StringArray
	err := row.Scan(
		&product.ID, &product.SKU, &product.Name, &product.CostPrice, &product.SellPrice,
		&product.Stock, &product.Status, &product.Notes, &product.CreatedBy,
		&product.CreatedAt, &product.UpdatedAt, &photos,
	)
	if err != nil {
		return entity.Product{}, err
	}
	product.Photos = []string(photos)
	if product.Photos == nil {
		product.Photos = []string{}
	}
	return product, nil
}

func (r *Repositories) CreateProduct(ctx context.Context, input entity.ProductInput, userID int64) (entity.Product, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO web_db.products (sku, name, cost_price, sell_price, stock, status, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`, input.SKU, input.Name, input.CostPrice, input.SellPrice, input.Stock, input.Status, input.Notes, userID).Scan(&id)
	if err != nil {
		return entity.Product{}, err
	}
	return r.GetProduct(ctx, id)
}

func (r *Repositories) UpdateProduct(ctx context.Context, id int64, input entity.ProductInput) (entity.Product, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE web_db.products
		SET sku=$2, name=$3, cost_price=$4, sell_price=$5, stock=$6, status=$7, notes=$8, updated_at=now()
		WHERE id=$1`, id, input.SKU, input.Name, input.CostPrice, input.SellPrice, input.Stock, input.Status, input.Notes)
	if err != nil {
		return entity.Product{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return entity.Product{}, err
	}
	if affected == 0 {
		return entity.Product{}, repository.ErrNotFound
	}
	return r.GetProduct(ctx, id)
}

func (r *Repositories) DeleteProduct(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM web_db.products WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func salesFilterSQL(filter entity.SalesFilter) (string, []any) {
	return `($1::int = 0 OR extract(year FROM s.sold_at)::int = $1)
		AND ($2::timestamptz IS NULL OR s.sold_at >= $2)
		AND ($3::timestamptz IS NULL OR s.sold_at < $3)
		AND ($4::text = '' OR s.platform = $4)`,
		[]any{filter.Year, nullableTime(filter.From), nullableTime(filter.To), filter.Platform}
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func (r *Repositories) ListSales(ctx context.Context, filter entity.SalesFilter, limit int) ([]entity.Sale, error) {
	where, args := salesFilterSQL(filter)
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.product_id, p.sku, p.name, s.user_id, u.name, s.platform, s.qty,
			s.final_price, s.extra_cost, s.total_modal, s.omzet, s.profit, s.sold_at
		FROM web_db.sales s
		JOIN web_db.products p ON p.id=s.product_id
		JOIN users.users u ON u.id=s.user_id
		WHERE `+where+`
		ORDER BY s.sold_at DESC, s.id DESC
		LIMIT $5`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sales := make([]entity.Sale, 0)
	for rows.Next() {
		var sale entity.Sale
		if err := rows.Scan(
			&sale.ID, &sale.ProductID, &sale.ProductSKU, &sale.Product, &sale.UserID,
			&sale.UserName, &sale.Platform, &sale.Qty, &sale.FinalPrice, &sale.ExtraCost,
			&sale.TotalModal, &sale.Omzet, &sale.Profit, &sale.SoldAt,
		); err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	return sales, rows.Err()
}

func (r *Repositories) CreateSale(ctx context.Context, input entity.SaleInput, userID int64) (entity.Sale, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Sale{}, err
	}
	defer tx.Rollback()

	var stock, costPrice int64
	err = tx.QueryRowContext(ctx, `
		SELECT stock, cost_price FROM web_db.products WHERE id=$1 FOR UPDATE`, input.ProductID).
		Scan(&stock, &costPrice)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Sale{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Sale{}, err
	}
	if int64(input.Qty) > stock {
		return entity.Sale{}, errors.New("insufficient stock")
	}
	totalModal := costPrice * int64(input.Qty)
	omzet := input.FinalPrice * int64(input.Qty)
	profit := omzet - totalModal - input.ExtraCost
	var saleID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO web_db.sales
			(product_id, user_id, platform, qty, final_price, extra_cost, total_modal, omzet, profit)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`, input.ProductID, userID, input.Platform, input.Qty, input.FinalPrice,
		input.ExtraCost, totalModal, omzet, profit).Scan(&saleID)
	if err != nil {
		return entity.Sale{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE web_db.products
		SET stock=stock-$2, status=CASE WHEN stock-$2=0 THEN 'Sold' ELSE 'Ready' END, updated_at=now()
		WHERE id=$1`, input.ProductID, input.Qty); err != nil {
		return entity.Sale{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO web_db.stock_movements (product_id, sale_id, user_id, type, qty_change, note)
		VALUES ($1,$2,$3,'sale',$4,'')`, input.ProductID, saleID, userID, -input.Qty); err != nil {
		return entity.Sale{}, err
	}
	if err := tx.Commit(); err != nil {
		return entity.Sale{}, err
	}
	sales, err := r.ListSales(ctx, entity.SalesFilter{}, 1)
	if err != nil {
		return entity.Sale{}, err
	}
	for _, sale := range sales {
		if sale.ID == saleID {
			return sale, nil
		}
	}
	return entity.Sale{}, fmt.Errorf("sale %d created but not returned", saleID)
}

func (r *Repositories) SalesSummary(ctx context.Context, filter entity.SalesFilter) (entity.SalesSummary, error) {
	where, args := salesFilterSQL(filter)
	var summary entity.SalesSummary
	err := r.db.QueryRowContext(ctx, `
		SELECT count(*), coalesce(sum(s.omzet),0), coalesce(sum(s.profit),0), coalesce(sum(s.total_modal),0)
		FROM web_db.sales s WHERE `+where, args...).Scan(&summary.Count, &summary.Omzet, &summary.Profit, &summary.TotalCost)
	if err != nil {
		return entity.SalesSummary{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT extract(month FROM s.sold_at)::int, coalesce(sum(s.omzet),0),
			coalesce(sum(s.profit),0), count(*)
		FROM web_db.sales s WHERE `+where+`
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return entity.SalesSummary{}, err
	}
	for rows.Next() {
		var month entity.MonthlySales
		if err := rows.Scan(&month.Month, &month.Omzet, &month.Profit, &month.Count); err != nil {
			rows.Close()
			return entity.SalesSummary{}, err
		}
		summary.ByMonth = append(summary.ByMonth, month)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return entity.SalesSummary{}, err
	}
	rows.Close()

	rows, err = r.db.QueryContext(ctx, `
		SELECT s.platform, coalesce(sum(s.omzet),0), coalesce(sum(s.profit),0), count(*)
		FROM web_db.sales s WHERE `+where+`
		GROUP BY s.platform ORDER BY sum(s.omzet) DESC`, args...)
	if err != nil {
		return entity.SalesSummary{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var channel entity.ChannelSales
		if err := rows.Scan(&channel.Platform, &channel.Omzet, &channel.Profit, &channel.Count); err != nil {
			return entity.SalesSummary{}, err
		}
		summary.ByChannel = append(summary.ByChannel, channel)
	}
	if err := rows.Err(); err != nil {
		return entity.SalesSummary{}, err
	}
	if summary.ByMonth == nil {
		summary.ByMonth = []entity.MonthlySales{}
	}
	if summary.ByChannel == nil {
		summary.ByChannel = []entity.ChannelSales{}
	}
	return summary, nil
}

func ParseSalesFilter(year, from, to, platform string) (entity.SalesFilter, error) {
	filter := entity.SalesFilter{Platform: strings.TrimSpace(platform)}
	if year != "" {
		if _, err := fmt.Sscanf(year, "%d", &filter.Year); err != nil || filter.Year < 2000 || filter.Year > 2200 {
			return entity.SalesFilter{}, errors.New("year must be between 2000 and 2200")
		}
	}
	var err error
	if from != "" {
		filter.From, err = time.Parse("2006-01-02", from)
		if err != nil {
			return entity.SalesFilter{}, errors.New("from must use YYYY-MM-DD")
		}
	}
	if to != "" {
		end, parseErr := time.Parse("2006-01-02", to)
		if parseErr != nil {
			return entity.SalesFilter{}, errors.New("to must use YYYY-MM-DD")
		}
		filter.To = end.AddDate(0, 0, 1)
	}
	if !filter.From.IsZero() && !filter.To.IsZero() && !filter.From.Before(filter.To) {
		return entity.SalesFilter{}, errors.New("from must be on or before to")
	}
	if filter.Platform != "" {
		switch filter.Platform {
		case "Instagram", "WhatsApp", "Shopee", "Tokopedia", "Offline", "Lainnya":
		default:
			return entity.SalesFilter{}, errors.New("invalid sales platform")
		}
	}
	return filter, nil
}
