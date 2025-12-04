package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	d "go-shop/internal/data"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
)

func (app *Application) ShowCheckout(w http.ResponseWriter, r *http.Request) {
	cart := app.getCartFromSession(r)
	if cart.ItemCount == 0 {
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "checkout.page.html", data)
}

func (app *Application) CreateBankDepositOrder(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	idempotencyKey := r.PostForm.Get("idempotency_key")
	// ... idempotency check from before ...

	cart := app.getCartFromSession(r)
	user := r.Context().Value(contextKeyUser).(*d.User)

	// --- ADD FINAL STOCK CHECK ---
	ok, message, err := app.verifyStock(cart)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if !ok {
		app.SessionManager.Put(r.Context(), "flash", message)
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}
	// --- END STOCK CHECK ---

	orderItems := []d.OrderItem{}
	for _, item := range cart.Items {
		orderItems = append(orderItems, d.OrderItem{
			ProductID: item.Product.ID,
			Quantity:  item.Quantity,
			Price:     item.FinalPrice, // Use the final price from the cart
			VariantDescription: sql.NullString{ // Use the description from the cart
				String: item.VariantDescription,
				Valid:  item.VariantDescription != "",
			},
		})
	}

	order := &d.Order{
		UserID:         user.ID,
		IdempotencyKey: sql.NullString{String: idempotencyKey, Valid: true},
		Status:         "pending",
		PaymentMethod:  "Bank Deposit",
		Total:          cart.Total,
		Items:          orderItems,
	}

	_, err = app.Orders.Insert(order)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.SessionManager.Remove(r.Context(), "cart")
	http.Redirect(w, r, "/order/success", http.StatusSeeOther)
}

func (app *Application) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Provider       string `json:"provider"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	// decode payload and perform idempotency check ...
	cart := app.getCartFromSession(r)
	user := r.Context().Value(contextKeyUser).(*d.User)
	baseURL := "http://localhost:4000"

	// --- FINAL STOCK CHECK ---
	ok, message, err := app.verifyStock(cart)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if !ok {
		app.renderJSON(w, http.StatusConflict, map[string]string{"error": message})
		return
	}
	// --- END STOCK CHECK ---

	orderItems := []d.OrderItem{}
	for _, item := range cart.Items {
		orderItems = append(orderItems, d.OrderItem{
			ProductID: item.Product.ID,
			Quantity:  item.Quantity,
			Price:     item.FinalPrice,
			VariantDescription: sql.NullString{
				String: item.VariantDescription,
				Valid:  item.VariantDescription != "",
			},
		})
	}

	order := &d.Order{
		UserID:         user.ID,
		IdempotencyKey: sql.NullString{String: payload.IdempotencyKey, Valid: true},
		Status:         "pending",
		PaymentMethod:  payload.Provider,
		Total:          cart.Total,
		Items:          orderItems,
	}
	orderID, err := app.Orders.Insert(order)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if payload.Provider == "stripe" {
		lineItems := []*stripe.CheckoutSessionLineItemParams{}
		for _, item := range cart.Items {
			lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:    stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{Name: stripe.String(item.Product.Name)},
					UnitAmount:  stripe.Int64(item.Product.Price),
				},
				Quantity: stripe.Int64(int64(item.Quantity)),
			})
		}
		params := &stripe.CheckoutSessionParams{
			PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
			LineItems:          lineItems,
			Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
			SuccessURL:         stripe.String(baseURL + "/order/success"),
			CancelURL:          stripe.String(baseURL + "/cart"),
			ClientReferenceID:  stripe.String(strconv.FormatInt(orderID, 10)),
		}
		s, err := session.New(params)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		app.renderJSON(w, http.StatusOK, map[string]string{"id": s.ID})
	} else if payload.Provider == "mercadopago" {
		// MercadoPago Logic here
	} else {
		http.Error(w, "Invalid provider", http.StatusBadRequest)
	}
}

func (app *Application) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	app.SessionManager.Remove(r.Context(), "cart")
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "success.page.html", data)
}

// checks if all items in the cart are in stock.
// It returns true if all items are available. If not, it returns false and a
// error message.
func (app *Application) verifyStock(cart *d.Cart) (bool, string, error) {
	for _, item := range cart.Items {
		product, err := app.Products.Get(item.Product.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, fmt.Sprintf("Product '%s' is no longer available.", item.Product.Name), nil
			}
			return false, "", err
		}

		var finalStock int = product.Stock
		hasVariants := product.VariantInfo.SKUs != nil && len(product.VariantInfo.SKUs) > 0

		if hasVariants {
			if item.VariantDescription == "" {
				return false, fmt.Sprintf("Variants for '%s' were not selected.", item.Product.Name), nil
			}

			foundSKU := false
			for _, sku := range product.VariantInfo.SKUs {
				// Rebuild the SKU's description string in a sorted order to compare.
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

				if skuDesc == item.VariantDescription {
					finalStock = sku.Stock
					foundSKU = true
					break
				}
			}
			if !foundSKU {
				return false, fmt.Sprintf("The selected options for '%s' are no longer available.", item.Product.Name), nil
			}
		}

		if item.Quantity > finalStock {
			message := fmt.Sprintf("Not enough stock for %s. Only %d left, but you have %d in your cart.", item.Product.Name, finalStock, item.Quantity)
			return false, message, nil
		}
	}

	return true, "", nil
}

// renderJSON sends a JSON response.
func (app *Application) renderJSON(w http.ResponseWriter, status int, data any) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
}
