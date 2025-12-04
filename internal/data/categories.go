package data

import (
	"database/sql"
	"log/slog"
)

type CategoryModel struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// GetAllWithCounts fetches all categories and the number of products in each.
func (m *CategoryModel) GetAllWithCounts() ([]*CategoryFilter, error) {
	stmt := `
        SELECT c.name, COUNT(p.id) as product_count
        FROM categories c
        LEFT JOIN products p ON c.id = p.category_id
        GROUP BY c.id
        ORDER BY c.name ASC
    `
	rows, err := m.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*CategoryFilter
	for rows.Next() {
		cf := &CategoryFilter{}
		err := rows.Scan(&cf.Name, &cf.ProductCount)
		if err != nil {
			return nil, err
		}
		categories = append(categories, cf)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}
