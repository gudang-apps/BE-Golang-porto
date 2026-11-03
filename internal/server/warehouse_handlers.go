package server

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"warehouse-service/internal/auth"
	"warehouse-service/internal/ocr"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type product struct {
	ID        string  `json:"id"`
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Unit      string  `json:"unit"`
	MinStock  float64 `json:"min_stock"`
	Active    bool    `json:"active"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func (s *Server) getProductBySKU(c *fiber.Ctx) error {
	sku := strings.TrimSpace(c.Params("sku"))
	if sku == "" {
		return fiber.NewError(fiber.StatusBadRequest, "sku is required")
	}
	var item product
	err := s.db.QueryRowContext(c.UserContext(), `
		SELECT id, sku, name, category, unit, min_stock, active, created_at, updated_at
		FROM warehouse.products WHERE lower(sku)=lower($1) AND active`, sku).
		Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit, &item.MinStock, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return notFoundOrServerError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": item})
}

type productInput struct {
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Unit     string  `json:"unit"`
	MinStock float64 `json:"min_stock"`
}

func (s *Server) getWarehouse(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var warehouseCode, name, location string
	var created, updated time.Time
	err = s.db.QueryRowContext(c.UserContext(), `
		SELECT code, name, location, created_at, updated_at
		FROM warehouse.warehouses WHERE id=$1 AND active`, id).
		Scan(&warehouseCode, &name, &location, &created, &updated)
	if err != nil {
		return notFoundOrServerError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"id": id, "code": warehouseCode, "name": name, "location": location,
		"created_at": created, "updated_at": updated,
	}})
}

type warehouseInput struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

func (s *Server) getSupplier(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var name, contact, phone, email, address string
	var created, updated time.Time
	err = s.db.QueryRowContext(c.UserContext(), `
		SELECT name, contact_name, phone, email, address, created_at, updated_at
		FROM warehouse.suppliers WHERE id=$1 AND active`, id).
		Scan(&name, &contact, &phone, &email, &address, &created, &updated)
	if err != nil {
		return notFoundOrServerError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"id": id, "name": name, "contact_name": contact, "phone": phone,
		"email": email, "address": address, "created_at": created, "updated_at": updated,
	}})
}

type supplierInput struct {
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Address     string `json:"address"`
}

type movementInput struct {
	ProductID     string  `json:"product_id"`
	Quantity      float64 `json:"quantity"`
	Counted       float64 `json:"counted_quantity"`
	FromWarehouse string  `json:"from_warehouse_id"`
	ToWarehouse   string  `json:"to_warehouse_id"`
	SupplierID    string  `json:"supplier_id"`
	Reference     string  `json:"reference"`
	Note          string  `json:"note"`
}

func (s *Server) listProducts(c *fiber.Ctx) error {
	search := "%" + strings.TrimSpace(c.Query("search")) + "%"
	rows, err := s.db.QueryContext(c.UserContext(), `
		SELECT id, sku, name, category, unit, min_stock, active, created_at, updated_at
		FROM warehouse.products WHERE active AND ($1 = '%%' OR sku ILIKE $1 OR name ILIKE $1)
		ORDER BY name, sku LIMIT 500`, search)
	if err != nil {
		return serverError(err)
	}
	defer rows.Close()
	items := make([]product, 0)
	for rows.Next() {
		var item product
		if err := rows.Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit, &item.MinStock, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return serverError(err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return serverError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": items})
}

func (s *Server) createProduct(c *fiber.Ctx) error {
	var input productInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateProduct(input); err != nil {
		return err
	}
	var item product
	err := s.db.QueryRowContext(c.UserContext(), `
		INSERT INTO warehouse.products (sku, name, category, unit, min_stock)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, sku, name, category, unit, min_stock, active, created_at, updated_at`,
		strings.TrimSpace(input.SKU), strings.TrimSpace(input.Name), strings.TrimSpace(input.Category),
		strings.TrimSpace(input.Unit), input.MinStock).
		Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit, &item.MinStock, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return conflictOrServerError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": item})
}

func (s *Server) getProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var item product
	err = s.db.QueryRowContext(c.UserContext(), `
		SELECT id, sku, name, category, unit, min_stock, active, created_at, updated_at
		FROM warehouse.products WHERE id = $1 AND active`, id).
		Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit, &item.MinStock, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return notFoundOrServerError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": item})
}

func (s *Server) updateProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var input productInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateProduct(input); err != nil {
		return err
	}
	var item product
	err = s.db.QueryRowContext(c.UserContext(), `
		UPDATE warehouse.products SET sku=$2, name=$3, category=$4, unit=$5, min_stock=$6, updated_at=now()
		WHERE id=$1 AND active
		RETURNING id, sku, name, category, unit, min_stock, active, created_at, updated_at`,
		id, strings.TrimSpace(input.SKU), strings.TrimSpace(input.Name), strings.TrimSpace(input.Category), strings.TrimSpace(input.Unit), input.MinStock).
		Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit, &item.MinStock, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return conflictOrNotFound(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": item})
}

func (s *Server) deleteProduct(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(c.UserContext(), `
		UPDATE warehouse.products p SET active=false, updated_at=now()
		WHERE p.id=$1 AND p.active
			AND NOT EXISTS (SELECT 1 FROM warehouse.stock_balances b WHERE b.product_id=p.id AND b.quantity > 0)`, id)
	if err != nil {
		return serverError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fiber.NewError(fiber.StatusConflict, "product not found or still has stock")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listWarehouses(c *fiber.Ctx) error {
	rows, err := s.db.QueryContext(c.UserContext(), `
		SELECT id, code, name, location, active, created_at, updated_at
		FROM warehouse.warehouses WHERE active ORDER BY name LIMIT 500`)
	if err != nil {
		return serverError(err)
	}
	defer rows.Close()
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, code, name, location, created, updated string
		var active bool
		if err := rows.Scan(&id, &code, &name, &location, &active, &created, &updated); err != nil {
			return serverError(err)
		}
		items = append(items, fiber.Map{"id": id, "code": code, "name": name, "location": location, "active": active, "created_at": created, "updated_at": updated})
	}
	if err := rows.Err(); err != nil {
		return serverError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": items})
}

func (s *Server) createWarehouse(c *fiber.Ctx) error {
	var input warehouseInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateWarehouse(input); err != nil {
		return err
	}
	var id string
	err := s.db.QueryRowContext(c.UserContext(), `
		INSERT INTO warehouse.warehouses (code, name, location) VALUES ($1,$2,$3)
		RETURNING id`, strings.TrimSpace(input.Code), strings.TrimSpace(input.Name), strings.TrimSpace(input.Location)).Scan(&id)
	if err != nil {
		return conflictOrServerError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": fiber.Map{"id": id, "code": strings.TrimSpace(input.Code), "name": strings.TrimSpace(input.Name), "location": strings.TrimSpace(input.Location)}})
}

func (s *Server) updateWarehouse(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var input warehouseInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateWarehouse(input); err != nil {
		return err
	}
	result, err := s.db.ExecContext(c.UserContext(), `
		UPDATE warehouse.warehouses SET code=$2, name=$3, location=$4, updated_at=now()
		WHERE id=$1 AND active`, id, strings.TrimSpace(input.Code), strings.TrimSpace(input.Name), strings.TrimSpace(input.Location))
	if err != nil {
		return conflictOrServerError(err)
	}
	return affectedResponse(c, result)
}

func (s *Server) deleteWarehouse(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(c.UserContext(), `
		UPDATE warehouse.warehouses w SET active=false, updated_at=now()
		WHERE w.id=$1 AND w.active
			AND NOT EXISTS (SELECT 1 FROM warehouse.stock_balances b WHERE b.warehouse_id=w.id AND b.quantity > 0)`, id)
	if err != nil {
		return serverError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fiber.NewError(fiber.StatusConflict, "warehouse not found or still has stock")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) listSuppliers(c *fiber.Ctx) error {
	search := "%" + strings.TrimSpace(c.Query("search")) + "%"
	rows, err := s.db.QueryContext(c.UserContext(), `
		SELECT id, name, contact_name, phone, email, address, created_at, updated_at
		FROM warehouse.suppliers WHERE active AND ($1='%%' OR name ILIKE $1)
		ORDER BY name LIMIT 500`, search)
	if err != nil {
		return serverError(err)
	}
	defer rows.Close()
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, name, contact, phone, email, address, created, updated string
		if err := rows.Scan(&id, &name, &contact, &phone, &email, &address, &created, &updated); err != nil {
			return serverError(err)
		}
		items = append(items, fiber.Map{"id": id, "name": name, "contact_name": contact, "phone": phone, "email": email, "address": address, "created_at": created, "updated_at": updated})
	}
	if err := rows.Err(); err != nil {
		return serverError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": items})
}

func (s *Server) createSupplier(c *fiber.Ctx) error {
	var input supplierInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateSupplier(input); err != nil {
		return err
	}
	var id string
	err := s.db.QueryRowContext(c.UserContext(), `
		INSERT INTO warehouse.suppliers (name, contact_name, phone, email, address)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		strings.TrimSpace(input.Name), strings.TrimSpace(input.ContactName), strings.TrimSpace(input.Phone), strings.TrimSpace(input.Email), strings.TrimSpace(input.Address)).Scan(&id)
	if err != nil {
		return serverError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": fiber.Map{"id": id, "name": strings.TrimSpace(input.Name)}})
}

func (s *Server) updateSupplier(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	var input supplierInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := validateSupplier(input); err != nil {
		return err
	}
	result, err := s.db.ExecContext(c.UserContext(), `
		UPDATE warehouse.suppliers SET name=$2, contact_name=$3, phone=$4, email=$5, address=$6, updated_at=now()
		WHERE id=$1 AND active`, id, strings.TrimSpace(input.Name), strings.TrimSpace(input.ContactName),
		strings.TrimSpace(input.Phone), strings.TrimSpace(input.Email), strings.TrimSpace(input.Address))
	if err != nil {
		return serverError(err)
	}
	return affectedResponse(c, result)
}

func (s *Server) deleteSupplier(c *fiber.Ctx) error {
	id, err := parseID(c.Params("id"))
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(c.UserContext(), `UPDATE warehouse.suppliers SET active=false, updated_at=now() WHERE id=$1 AND active`, id)
	if err != nil {
		return serverError(err)
	}
	return affectedResponse(c, result)
}

func (s *Server) listStock(c *fiber.Ctx) error {
	return s.stockQuery(c, false)
}

func (s *Server) listLowStock(c *fiber.Ctx) error {
	return s.stockQuery(c, true)
}

func (s *Server) stockQuery(c *fiber.Ctx, lowOnly bool) error {
	productID := strings.TrimSpace(c.Query("product_id"))
	warehouseID := strings.TrimSpace(c.Query("warehouse_id"))
	if productID != "" {
		if _, err := parseID(productID); err != nil {
			return err
		}
	}
	if warehouseID != "" {
		if _, err := parseID(warehouseID); err != nil {
			return err
		}
	}
	rows, err := s.db.QueryContext(c.UserContext(), `
		SELECT p.id, p.sku, p.name, w.id, w.code, w.name,
			COALESCE(b.quantity, 0), p.min_stock,
			(COALESCE(b.quantity, 0) < p.min_stock) AS is_low
		FROM warehouse.products p CROSS JOIN warehouse.warehouses w
		LEFT JOIN warehouse.stock_balances b ON b.product_id=p.id AND b.warehouse_id=w.id
		WHERE p.active AND w.active
			AND ($1='' OR p.id::text=$1)
			AND ($2='' OR w.id::text=$2)
			AND (NOT $3 OR COALESCE(b.quantity, 0) < p.min_stock)
		ORDER BY p.name, w.name LIMIT 1000`, productID, warehouseID, lowOnly)
	if err != nil {
		return serverError(err)
	}
	defer rows.Close()
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var pID, sku, pName, wID, wCode, wName string
		var quantity, minStock float64
		var low bool
		if err := rows.Scan(&pID, &sku, &pName, &wID, &wCode, &wName, &quantity, &minStock, &low); err != nil {
			return serverError(err)
		}
		items = append(items, fiber.Map{
			"product":   fiber.Map{"id": pID, "sku": sku, "name": pName},
			"warehouse": fiber.Map{"id": wID, "code": wCode, "name": wName},
			"quantity":  quantity, "min_stock": minStock, "low_stock": low,
		})
	}
	if err := rows.Err(); err != nil {
		return serverError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": items})
}

func (s *Server) scanSKU(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "multipart field 'image' is required")
	}
	if file.Size > 8*1024*1024 {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "image must not exceed 8 MB")
	}
	source, err := file.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "cannot read uploaded image")
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, 8*1024*1024+1))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "cannot read uploaded image")
	}
	if len(data) > 8*1024*1024 {
		return fiber.NewError(fiber.StatusRequestEntityTooLarge, "image must not exceed 8 MB")
	}
	result, err := s.ocr.Read(c.UserContext(), data)
	switch {
	case errors.Is(err, ocr.ErrUnavailable):
		log.Printf("OCR engine unavailable: %v", err)
		return fiber.NewError(fiber.StatusServiceUnavailable, "OCR is unavailable; install Tesseract and language data")
	case errors.Is(err, ocr.ErrUnsupportedImage):
		return fiber.NewError(fiber.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, ocr.ErrEmptyResult):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	case err != nil:
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}

func (s *Server) listMovements(c *fiber.Ctx) error {
	rows, err := s.db.QueryContext(c.UserContext(), `
		SELECT m.id, m.movement_type, m.product_id, p.sku, p.name, m.quantity,
			m.from_warehouse_id, fw.name, m.to_warehouse_id, tw.name,
			m.supplier_id, COALESCE(s.name, ''), m.reference, m.note, m.created_by, m.created_at
		FROM warehouse.stock_movements m
		JOIN warehouse.products p ON p.id=m.product_id
		LEFT JOIN warehouse.warehouses fw ON fw.id=m.from_warehouse_id
		LEFT JOIN warehouse.warehouses tw ON tw.id=m.to_warehouse_id
		LEFT JOIN warehouse.suppliers s ON s.id=m.supplier_id
		ORDER BY m.created_at DESC LIMIT 500`)
	if err != nil {
		return serverError(err)
	}
	defer rows.Close()
	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id, kind, productID, sku, name, createdBy, createdAt string
		var quantity float64
		var fromID, fromName, toID, toName, supplierID, supplierName sql.NullString
		var reference, note string
		if err := rows.Scan(&id, &kind, &productID, &sku, &name, &quantity, &fromID, &fromName, &toID, &toName, &supplierID, &supplierName, &reference, &note, &createdBy, &createdAt); err != nil {
			return serverError(err)
		}
		items = append(items, fiber.Map{
			"id": id, "type": kind, "product": fiber.Map{"id": productID, "sku": sku, "name": name},
			"quantity": quantity, "from_warehouse_id": nullable(fromID), "from_warehouse": nullable(fromName),
			"to_warehouse_id": nullable(toID), "to_warehouse": nullable(toName),
			"supplier_id": nullable(supplierID), "supplier": nullable(supplierName),
			"reference": reference, "note": note, "created_by": createdBy, "created_at": createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return serverError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": items})
}

func (s *Server) createInbound(c *fiber.Ctx) error {
	var input movementInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	return s.applyMovement(c, "inbound", input)
}

func (s *Server) createOutbound(c *fiber.Ctx) error {
	var input movementInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	return s.applyMovement(c, "outbound", input)
}

func (s *Server) createTransfer(c *fiber.Ctx) error {
	var input movementInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	return s.applyMovement(c, "transfer", input)
}

func (s *Server) createAdjustment(c *fiber.Ctx) error {
	var input movementInput
	if err := c.BodyParser(&input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	return s.applyMovement(c, "adjustment", input)
}

func (s *Server) applyMovement(c *fiber.Ctx, kind string, input movementInput) error {
	user, ok := auth.CurrentUser(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}
	productID, err := parseID(input.ProductID)
	if err != nil {
		return err
	}
	if len(input.Reference) > 100 {
		return fiber.NewError(fiber.StatusBadRequest, "reference is too long")
	}
	var fromID, toID, supplierID any
	switch kind {
	case "inbound":
		toID, err = parseID(input.ToWarehouse)
		if err == nil && input.SupplierID != "" {
			supplierID, err = parseID(input.SupplierID)
		}
		if input.Quantity <= 0 {
			err = fmt.Errorf("quantity must be greater than zero")
		}
	case "outbound":
		fromID, err = parseID(input.FromWarehouse)
		if input.Quantity <= 0 {
			err = fmt.Errorf("quantity must be greater than zero")
		}
	case "transfer":
		fromID, err = parseID(input.FromWarehouse)
		if err == nil {
			toID, err = parseID(input.ToWarehouse)
		}
		if err == nil && fromID == toID {
			err = fmt.Errorf("source and destination warehouses must differ")
		}
		if input.Quantity <= 0 {
			err = fmt.Errorf("quantity must be greater than zero")
		}
	case "adjustment":
		fromID, err = parseID(input.FromWarehouse)
		if input.Counted < 0 || !finite(input.Counted) {
			err = fmt.Errorf("counted_quantity must be zero or greater")
		}
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if kind != "adjustment" && !finite(input.Quantity) {
		return fiber.NewError(fiber.StatusBadRequest, "quantity must be finite")
	}
	if kind != "adjustment" && !threeDecimalPlaces(input.Quantity) {
		return fiber.NewError(fiber.StatusBadRequest, "quantity must have at most 3 decimal places")
	}
	if kind == "adjustment" && !threeDecimalPlaces(input.Counted) {
		return fiber.NewError(fiber.StatusBadRequest, "counted_quantity must have at most 3 decimal places")
	}

	tx, err := s.db.BeginTx(c.UserContext(), nil)
	if err != nil {
		return serverError(err)
	}
	defer tx.Rollback()
	var active bool
	if err := tx.QueryRowContext(c.UserContext(), `SELECT active FROM warehouse.products WHERE id=$1 FOR UPDATE`, productID).Scan(&active); err != nil {
		return notFoundOrServerError(err)
	}
	if !active {
		return fiber.NewError(fiber.StatusNotFound, "product not found")
	}
	warehouseIDs := make([]string, 0, 2)
	if id, ok := fromID.(string); ok {
		warehouseIDs = append(warehouseIDs, id)
	}
	if id, ok := toID.(string); ok {
		warehouseIDs = append(warehouseIDs, id)
	}
	sort.Strings(warehouseIDs)
	for _, warehouseID := range warehouseIDs {
		if err := ensureWarehouse(c, tx, warehouseID); err != nil {
			return err
		}
	}
	if supplierID != nil {
		var supplierActive bool
		if err := tx.QueryRowContext(c.UserContext(), `SELECT active FROM warehouse.suppliers WHERE id=$1 FOR UPDATE`, supplierID).Scan(&supplierActive); err != nil {
			return notFoundOrServerError(err)
		}
		if !supplierActive {
			return fiber.NewError(fiber.StatusNotFound, "supplier not found")
		}
	}

	quantity := input.Quantity
	if kind == "adjustment" {
		if _, err := tx.ExecContext(c.UserContext(), `
			INSERT INTO warehouse.stock_balances (product_id, warehouse_id)
			VALUES ($1,$2) ON CONFLICT DO NOTHING`, productID, fromID); err != nil {
			return serverError(err)
		}
		var current float64
		if err := tx.QueryRowContext(c.UserContext(), `
			SELECT quantity FROM warehouse.stock_balances WHERE product_id=$1 AND warehouse_id=$2 FOR UPDATE`,
			productID, fromID).Scan(&current); err != nil {
			return serverError(err)
		}
		quantity = input.Counted - current
		if quantity == 0 {
			return fiber.NewError(fiber.StatusConflict, "counted quantity matches current stock")
		}
		if _, err := tx.ExecContext(c.UserContext(), `
			UPDATE warehouse.stock_balances SET quantity=$3, updated_at=now()
			WHERE product_id=$1 AND warehouse_id=$2`, productID, fromID, input.Counted); err != nil {
			return serverError(err)
		}
	} else {
		if fromID != nil {
			if _, err := tx.ExecContext(c.UserContext(), `INSERT INTO warehouse.stock_balances (product_id, warehouse_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, productID, fromID); err != nil {
				return serverError(err)
			}
			result, err := tx.ExecContext(c.UserContext(), `
				UPDATE warehouse.stock_balances SET quantity=quantity-$3, updated_at=now()
				WHERE product_id=$1 AND warehouse_id=$2 AND quantity >= $3`,
				productID, fromID, input.Quantity)
			if err != nil {
				return serverError(err)
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				return fiber.NewError(fiber.StatusConflict, "insufficient stock")
			}
		}
		if toID != nil {
			if _, err := tx.ExecContext(c.UserContext(), `
				INSERT INTO warehouse.stock_balances (product_id, warehouse_id, quantity)
				VALUES ($1,$2,$3) ON CONFLICT (product_id, warehouse_id)
				DO UPDATE SET quantity=warehouse.stock_balances.quantity + EXCLUDED.quantity, updated_at=now()`,
				productID, toID, input.Quantity); err != nil {
				return serverError(err)
			}
		}
	}

	var movementID string
	err = tx.QueryRowContext(c.UserContext(), `
		INSERT INTO warehouse.stock_movements
			(movement_type, product_id, quantity, from_warehouse_id, to_warehouse_id, supplier_id, reference, note, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		kind, productID, quantity, fromID, toID, supplierID,
		strings.TrimSpace(input.Reference), strings.TrimSpace(input.Note), user.ID).Scan(&movementID)
	if err != nil {
		return serverError(err)
	}
	if err := tx.Commit(); err != nil {
		return serverError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": fiber.Map{"id": movementID, "type": kind, "quantity": quantity}})
}

func ensureWarehouse(c *fiber.Ctx, tx *sql.Tx, id any) error {
	var active bool
	if err := tx.QueryRowContext(c.UserContext(), `SELECT active FROM warehouse.warehouses WHERE id=$1 FOR UPDATE`, id).Scan(&active); err != nil {
		return notFoundOrServerError(err)
	}
	if !active {
		return fiber.NewError(fiber.StatusNotFound, "warehouse not found")
	}
	return nil
}

func validateProduct(input productInput) error {
	if strings.TrimSpace(input.SKU) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Unit) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "sku, name, and unit are required")
	}
	if len(input.SKU) > 100 || len(input.Name) > 200 || len(input.Category) > 100 || len(input.Unit) > 40 {
		return fiber.NewError(fiber.StatusBadRequest, "one or more product fields are too long")
	}
	if input.MinStock < 0 || !finite(input.MinStock) || !threeDecimalPlaces(input.MinStock) {
		return fiber.NewError(fiber.StatusBadRequest, "min_stock must be zero or greater with at most 3 decimal places")
	}
	return nil
}

func validateWarehouse(input warehouseInput) error {
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "code and name are required")
	}
	if len(input.Code) > 40 || len(input.Name) > 150 || len(input.Location) > 2000 {
		return fiber.NewError(fiber.StatusBadRequest, "one or more warehouse fields are too long")
	}
	return nil
}

func validateSupplier(input supplierInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	if len(input.Name) > 200 || len(input.ContactName) > 150 || len(input.Phone) > 40 || len(input.Email) > 254 || len(input.Address) > 2000 {
		return fiber.NewError(fiber.StatusBadRequest, "one or more supplier fields are too long")
	}
	return nil
}

func parseID(value string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", fiber.NewError(fiber.StatusBadRequest, "invalid UUID")
	}
	return id.String(), nil
}

func serverError(err error) error {
	log.Printf("database request failed: %v", err)
	return fiber.NewError(fiber.StatusInternalServerError, "database request failed")
}

func notFoundOrServerError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "resource not found")
	}
	return serverError(err)
}

func conflictOrServerError(err error) error {
	var postgresError *pq.Error
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return fiber.NewError(fiber.StatusConflict, "resource already exists")
	}
	return serverError(err)
}

func conflictOrNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fiber.NewError(fiber.StatusNotFound, "resource not found")
	}
	return conflictOrServerError(err)
}

func affectedResponse(c *fiber.Ctx, result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return serverError(err)
	}
	if affected == 0 {
		return fiber.NewError(fiber.StatusNotFound, "resource not found")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func nullable(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func threeDecimalPlaces(value float64) bool {
	return math.Abs(value*1000-math.Round(value*1000)) < 1e-8
}
