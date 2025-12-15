package data

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ProductModel struct {
	DB     *sql.DB
	Logger *slog.Logger
}

type SearchFilters struct {
	Query    string
	Category string
	MinPrice int
	MaxPrice int
	Sort     string
	Page     int
	PageSize int
}

// URLWith returns the current URL with modified filter values.
// It accepts key-value pairs to update, e.g., "page", "2" or "category", "Apparel".
func (f SearchFilters) URLWith(mods ...string) string {
	v := url.Values{}
	// Start with the existing filters
	if f.Query != "" {
		v.Set("query", f.Query)
	}
	if f.Category != "" {
		v.Set("category", f.Category)
	}
	if f.MinPrice > 0 {
		v.Set("min_price", strconv.Itoa(f.MinPrice))
	}
	if f.MaxPrice > 0 {
		v.Set("max_price", strconv.Itoa(f.MaxPrice))
	}
	if f.Sort != "" {
		v.Set("sort", f.Sort)
	}
	v.Set("page", strconv.Itoa(f.Page))

	// Apply modifications
	// mods are passed as "key1", "value1", "key2", "value2", ...
	for i := 0; i < len(mods); i += 2 {
		key := mods[i]
		value := mods[i+1]
		if value == "" {
			v.Del(key) // If value is empty, remove the parameter
		} else {
			v.Set(key, value)
		}
	}

	// Always reset to page 1 when a major filter (not 'page' itself) is changed.
	resetPage := true
	for i := 0; i < len(mods); i += 2 {
		if mods[i] == "page" {
			resetPage = false
			break
		}
	}
	if resetPage {
		v.Set("page", "1")
	}

	return "/?" + v.Encode()
}

// unmarshalVariants is a helper to safely unmarshal the JSON
func unmarshalVariants(p *Product) error {
	if p.VariantsJSON.Valid && p.VariantsJSON.String != "" {
		err := json.Unmarshal([]byte(p.VariantsJSON.String), &p.VariantInfo)
		if err != nil {
			return fmt.Errorf("error unmarshaling variants for product %d: %w", p.ID, err)
		}
	}
	return nil
}

// Search retrieves products based on a variety of filters.
func (m *ProductModel) Search(filters SearchFilters) ([]*Product, int, error) {
	baseQuery := `
        SELECT p.id, p.name, p.description, p.category_id, p.image_url, p.price, p.stock, p.variants_json, p.created_at, COALESCE(c.name, '') as category_name
        FROM products p
        LEFT JOIN categories c ON p.category_id = c.id
    `
	countQuery := `SELECT COUNT(*) FROM products p LEFT JOIN categories c ON p.category_id = c.id`

	var whereClauses []string
	var args []interface{}

	if filters.Query != "" {
		// Now we can search inside the JSON text!
		whereClauses = append(whereClauses, `(p.name LIKE ? OR p.description LIKE ? OR c.name LIKE ? OR p.variants_json LIKE ?)`)
		searchTerm := "%" + filters.Query + "%"
		args = append(args, searchTerm, searchTerm, searchTerm, searchTerm)
	}
	if filters.Category != "" {
		whereClauses = append(whereClauses, "c.name = ?")
		args = append(args, filters.Category)
	}
	if filters.MinPrice > 0 {
		whereClauses = append(whereClauses, "p.price >= ?")
		args = append(args, filters.MinPrice*100)
	}
	if filters.MaxPrice > 0 {
		whereClauses = append(whereClauses, "p.price <= ?")
		args = append(args, filters.MaxPrice*100)
	}

	if len(whereClauses) > 0 {
		wherePart := " WHERE " + strings.Join(whereClauses, " AND ")
		baseQuery += wherePart
		countQuery += wherePart
	}

	// Sorting
	switch filters.Sort {
	case "price_asc":
		baseQuery += " ORDER BY p.price ASC"
	case "price_desc":
		baseQuery += " ORDER BY p.price DESC"
	case "date_desc":
		baseQuery += " ORDER BY p.created_at DESC"
	default:
		// Default to newest products first
		baseQuery += " ORDER BY p.created_at DESC"
	}

	baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", filters.PageSize, (filters.Page-1)*filters.PageSize)

	m.Logger.Debug("Executing product search", "count_sql", countQuery, "search_sql", baseQuery, "args", args)

	var totalRecords int
	err := m.DB.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		m.Logger.Error("Failed to execute count query", "error", err)
		return nil, 0, err
	}

	if totalRecords == 0 {
		return []*Product{}, 0, nil
	}

	rows, err := m.DB.Query(baseQuery, args...)
	if err != nil {
		m.Logger.Error("Failed to execute search query", "error", err)
		return nil, 0, err
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p := &Product{}
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CategoryID, &p.ImageURL, &p.Price, &p.Stock, &p.VariantsJSON, &p.CreatedAt, &p.CategoryName)
		if err != nil {
			return nil, 0, err
		}
		// Unmarshal the JSON for each product
		if err := unmarshalVariants(p); err != nil {
			m.Logger.Error("skipping product due to invalid variant JSON", "product_id", p.ID, "error", err)
			continue
		}
		p.TotalStock = p.Stock
		if p.VariantInfo.SKUs != nil {
			p.TotalStock = 0
			// Sum the stock of all available SKUs
			for _, sku := range p.VariantInfo.SKUs {
				p.TotalStock += sku.Stock
			}
		}

		products = append(products, p)

	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return products, totalRecords, nil
}

// Get fetches a single product along with its variants.
func (m *ProductModel) Get(id int64) (*Product, error) {
	stmt := `
        SELECT p.id, p.name, p.description, p.category_id, p.image_url, p.price, p.stock, p.variants_json, p.created_at, COALESCE(c.name, '') as category_name
        FROM products p
        LEFT JOIN categories c ON p.category_id = c.id
        WHERE p.id = ?`

	row := m.DB.QueryRow(stmt, id)
	p := &Product{}
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CategoryID, &p.ImageURL, &p.Price, &p.Stock, &p.VariantsJSON, &p.CreatedAt, &p.CategoryName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	// Unmarshal the JSON into the usable struct field.
	if err := unmarshalVariants(p); err != nil {
		return nil, err
	}

	return p, nil
}

// --- CRUD Methods ---

func (m *ProductModel) Insert(p *Product) (int64, error) {
	stmt := `INSERT INTO products (name, description, category_id, image_url, price, stock, variants_json, created_at, updated_at)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := m.DB.Exec(stmt, p.Name, p.Description, p.CategoryID, p.ImageURL, p.Price, p.Stock, p.VariantsJSON, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (m *ProductModel) Update(p *Product) error {
	stmt := `UPDATE products SET name=?, description=?, category_id=?, image_url=?, price=?, stock=?, variants_json=?, updated_at=? WHERE id=?`
	_, err := m.DB.Exec(stmt, p.Name, p.Description, p.CategoryID, p.ImageURL, p.Price, p.Stock, p.VariantsJSON, time.Now(), p.ID)
	return err
}

func (m *ProductModel) Delete(id int64) error {
	stmt := `DELETE FROM products WHERE id = ?`
	_, err := m.DB.Exec(stmt, id)
	return err
}
