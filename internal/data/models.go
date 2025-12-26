package data

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Category struct {
	ID   int64
	Name string
}

type CategoryFilter struct {
	Name         string
	ProductCount int
}

// This represents one option within a variant group, e.g., "Large"
type VariantOption struct {
	Value         string `json:"value"`
	PriceModifier int64  `json:"priceModifier"`
	Stock         int    `json:"stock"`
	Image         string `json:"image,omitempty"`
}

// Represents a specific purchasable combination
type SKU struct {
	Attributes    map[string]string `json:"attributes"`
	PriceModifier int64             `json:"priceModifier"`
	Stock         int               `json:"stock"`
	Image         string            `json:"image,omitempty"`
}

// Represents the UI definition for a variant type
type VariantDisplayGroup struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

// struct for the JSON blob
type ProductVariantInfo struct {
	VariantGroups []VariantDisplayGroup `json:"variant_groups"`
	SKUs          []SKU                 `json:"skus"`
}

type Product struct {
	ID           int64
	Name         string
	Description  string
	CategoryID   sql.NullInt64
	CategoryName string
	ImagesJSON   sql.NullString `json:"-"`
	ImageURLs    []string       `json:"-"`
	Price        int64
	Stock        int
	VariantsJSON sql.NullString
	VariantInfo  ProductVariantInfo `json:"-"`
	TotalStock   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// A small struct to hold the selected option and its group context
type SelectedOption struct {
	GroupName string
	Option    VariantOption
}

// CartItem represents an item in the shopping cart.
type CartItem struct {
	CompositeID        string
	Product            *Product
	VariantDescription string
	Quantity           int
	FinalPrice         int64
}

// Cart represents the entire shopping cart.
type Cart struct {
	Items     map[string]*CartItem
	Total     int64
	ItemCount int
}

type User struct {
	ID             int64
	Name           string
	Email          string
	HashedPassword string
	IsAdmin        bool
	CreatedAt      time.Time
}

type Order struct {
	ID             int64
	UserID         int64
	IdempotencyKey sql.NullString
	Status         string
	PaymentMethod  string
	TrackingNumber sql.NullString
	Total          int64
	CreatedAt      time.Time
	Items          []OrderItem
	UserEmail      string
}

type OrderItem struct {
	ID                 int64
	OrderID            int64
	ProductID          int64
	VariantID          sql.NullInt64
	Quantity           int
	Price              int64
	VariantDescription sql.NullString
	ProductName        string
}

// NewCart creates an empty cart.
func NewCart() *Cart {
	return &Cart{
		Items: make(map[string]*CartItem),
	}
}

// Checks if the provided password matches the hashed one.
func (u *User) PasswordMatches(plainTextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(plainTextPassword))
	if err != nil {
		switch err {
		case bcrypt.ErrMismatchedHashAndPassword:
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}
func GenerateCompositeID(productID int64, options []SelectedOption) string {
	// Create a stable key by sorting variant names and then values
	sort.Slice(options, func(i, j int) bool {
		return options[i].GroupName < options[j].GroupName
	})

	idParts := []string{strconv.FormatInt(productID, 10)}
	for _, selOpt := range options {
		idParts = append(idParts, selOpt.GroupName, selOpt.Option.Value)
	}

	key := strings.Join(idParts, "-")
	hash := sha1.Sum([]byte(key))
	return hex.EncodeToString(hash[:])

}

// AddItem adds a product with selected variant options to the cart.
func (c *Cart) AddItem(product *Product, variantDescription string, finalPrice int64, quantity int) {
	// Create a stable key for the composite ID
	key := fmt.Sprintf("%d-%s", product.ID, variantDescription)
	hash := sha1.Sum([]byte(key))
	compositeID := hex.EncodeToString(hash[:])

	if item, exists := c.Items[compositeID]; exists {
		item.Quantity += quantity
	} else {
		c.Items[compositeID] = &CartItem{
			CompositeID:        compositeID,
			Product:            product,
			VariantDescription: variantDescription,
			Quantity:           quantity,
			FinalPrice:         finalPrice,
		}
	}
	c.recalculate()
}

// UpdateItemByCompositeID changes the quantity of an item in the cart.
func (c *Cart) UpdateItemByCompositeID(compositeID string, quantity int) {
	if item, exists := c.Items[compositeID]; exists {
		if quantity > 0 {
			item.Quantity = quantity
		} else {
			delete(c.Items, compositeID)
		}
	}
	c.recalculate()
}

// RemoveItemByCompositeID removes an item from the cart by its unique hash.
func (c *Cart) RemoveItemByCompositeID(compositeID string) {
	delete(c.Items, compositeID)
	c.recalculate()
}

// recalculate updates the total price and item count.
func (c *Cart) recalculate() {
	var total int64
	var count int
	for _, item := range c.Items {
		total += item.FinalPrice * int64(item.Quantity)
		count += item.Quantity
	}
	c.Total = total
	c.ItemCount = count
}

// FeaturedImageURL returns the primary image for the product.
// It prioritizes the first image in the ImageURLs slice, with a safe fallback.
func (p *Product) FeaturedImageURL() string {
	if len(p.ImageURLs) > 0 {
		return p.ImageURLs[0]
	}
	return "https://placehold.co/600x600/ccc/FFFFFF/png?text=No+Image"
}
