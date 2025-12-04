package data

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

type OrderModel struct {
	DB     *sql.DB
	Logger *slog.Logger
}

// Insert creates a new order and atomically decrements stock for all items.
func (m *OrderModel) Insert(order *Order) (int64, error) {
	tx, err := m.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // Rollback on any error

	// --- STOCK DECREMENT AND VERIFICATION LOGIC ---
	for _, item := range order.Items {
		var currentStock int
		var productID int64 = item.ProductID
		var variantsJSON sql.NullString

		// Step A: Lock the product row for update and get current stock.
		// `FOR UPDATE` is not standard in SQLite but the transaction provides locking.
		// We fetch the latest data to be sure.
		err := tx.QueryRow(`SELECT stock, variants_json FROM products WHERE id = ?`, productID).Scan(&currentStock, &variantsJSON)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, fmt.Errorf("product with ID %d not found", productID)
			}
			return 0, err
		}

		if variantsJSON.Valid && variantsJSON.String != "" {
			// Product has variants, we need to update the JSON
			var variantInfo ProductVariantInfo
			if err := json.Unmarshal([]byte(variantsJSON.String), &variantInfo); err != nil {
				return 0, fmt.Errorf("could not parse variants for product %d", productID)
			}

			skuFound := false
			for i, sku := range variantInfo.SKUs {
				// Rebuild description string to find the matching SKU
				var descriptions []string
				keys := make([]string, 0, len(sku.Attributes))
				for k := range sku.Attributes {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					descriptions = append(descriptions, fmt.Sprintf("%s: %s", k, sku.Attributes[k]))
				}
				skuDesc := strings.Join(descriptions, ", ")

				if skuDesc == item.VariantDescription.String {
					// Found the SKU. Check stock.
					if sku.Stock < item.Quantity {
						return 0, fmt.Errorf("not enough stock for %s (%s)", item.ProductName, item.VariantDescription.String)
					}
					// Decrement stock
					variantInfo.SKUs[i].Stock -= item.Quantity
					skuFound = true
					break
				}
			}
			if !skuFound {
				return 0, fmt.Errorf("variant %s for product %d not found", item.VariantDescription.String, productID)
			}

			// Marshal the updated variant info back to JSON
			newVariantsJSON, err := json.Marshal(variantInfo)
			if err != nil {
				return 0, err
			}

			// Update the product row with the new JSON
			_, err = tx.Exec(`UPDATE products SET variants_json = ?, updated_at = ? WHERE id = ?`, string(newVariantsJSON), time.Now(), productID)
			if err != nil {
				return 0, err
			}

		} else {
			// Product has no variants, update the base stock
			if currentStock < item.Quantity {
				return 0, fmt.Errorf("not enough stock for %s", item.ProductName)
			}
			_, err = tx.Exec(`UPDATE products SET stock = stock - ?, updated_at = ? WHERE id = ?`, item.Quantity, time.Now(), productID)
			if err != nil {
				return 0, err
			}
		}
	}
	// --- END STOCK DECREMENT LOGIC ---

	// Insert into orders table
	stmt := `INSERT INTO orders (user_id, idempotency_key, status, payment_method, total, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := tx.Exec(stmt, order.UserID, order.IdempotencyKey, order.Status, order.PaymentMethod, order.Total, time.Now())
	if err != nil {
		return 0, err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Insert into order_items
	itemStmt, err := tx.Prepare(`INSERT INTO order_items (order_id, product_id, quantity, price, variant_description) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer itemStmt.Close()

	for _, item := range order.Items {
		_, err := itemStmt.Exec(orderID, item.ProductID, item.Quantity, item.Price, item.VariantDescription)
		if err != nil {
			return 0, err
		}
	}

	return orderID, tx.Commit()
}

// GetForUser fetches all orders for a specific user.
func (m *OrderModel) GetForUser(userID int64) ([]*Order, error) {
	stmt := `SELECT id, status, payment_method, tracking_number, total, created_at FROM orders WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := m.DB.Query(stmt, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		err := rows.Scan(&o.ID, &o.Status, &o.PaymentMethod, &o.TrackingNumber, &o.Total, &o.CreatedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetForUserAndID fetches a single order for a specific user to prevent users from viewing each other's orders.
func (m *OrderModel) GetForUserAndID(orderID int64, userID int64) (*Order, error) {
	// Fetch the main order details, ensuring it belongs to the user.
	stmt := `SELECT id, status, payment_method, tracking_number, total, created_at FROM orders WHERE id = ? AND user_id = ?`
	row := m.DB.QueryRow(stmt, orderID, userID)
	o := &Order{}
	err := row.Scan(&o.ID, &o.Status, &o.PaymentMethod, &o.TrackingNumber, &o.Total, &o.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows // handle not found or not authorized
		}
		return nil, err
	}

	// Fetch the associated order itemsx
	itemStmt := `
        SELECT oi.id, oi.product_id, oi.quantity, oi.price, oi.variant_description, p.name
        FROM order_items oi
        JOIN products p ON oi.product_id = p.id
        WHERE oi.order_id = ?`

	rows, err := m.DB.Query(itemStmt, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		i := OrderItem{}

		err := rows.Scan(&i.ID, &i.ProductID, &i.Quantity, &i.Price, &i.VariantDescription, &i.ProductName)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	o.Items = items
	return o, nil
}

// GetAllPending fetches all pending orders along with the user's email for the admin view.
func (m *OrderModel) GetAllPending() ([]*Order, error) {
	stmt := `
        SELECT o.id, o.user_id, o.status, o.payment_method, o.total, o.created_at, u.email
        FROM orders o
        JOIN users u ON o.user_id = u.id
        WHERE o.status = 'pending'
        ORDER BY o.created_at DESC`
	rows, err := m.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.PaymentMethod, &o.Total, &o.CreatedAt, &o.UserEmail)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetByIdempotencyKey finds an order by its unique key.
func (m *OrderModel) GetByIdempotencyKey(key string) (*Order, error) {
	var order Order
	stmt := `SELECT id, user_id, status, total FROM orders WHERE idempotency_key = ?`
	err := m.DB.QueryRow(stmt, key).Scan(&order.ID, &order.UserID, &order.Status, &order.Total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows // Special error for "not found"
		}
		return nil, err
	}
	return &order, nil
}

// UpdateStatusAndTracking updates an order's status and tracking number.
func (m *OrderModel) UpdateStatusAndTracking(id int64, status, trackingNumber string) error {
	stmt := `UPDATE orders SET status = ?, tracking_number = ? WHERE id = ?`
	_, err := m.DB.Exec(stmt, status, trackingNumber, id)
	return err
}
