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

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
)

func (app *Application) ShowCheckout(w http.ResponseWriter, r *http.Request) {
	cart := app.getCartFromSession(r)
	if cart.ItemCount == 0 {
		http.Redirect(w, r, "/cart", http.StatusSeeOther)
		return
	}

	// Check if a key already exists in the session for this checkout.
	idempotencyKey := app.SessionManager.GetString(r.Context(), "idempotencyKey")
	if idempotencyKey == "" {
		// If not, generate a new one and save it.
		idempotencyKey = uuid.New().String()
		app.SessionManager.Put(r.Context(), "idempotencyKey", idempotencyKey)
	}
	//
	data := app.newTemplateData(r)
	data.IdempotencyKey = idempotencyKey
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
	existingOrder, err := app.Orders.GetByIdempotencyKey(idempotencyKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, r, err)
		return
	}

	var orderID int64
	cart := app.getCartFromSession(r)
	user := r.Context().Value(contextKeyUser).(*d.User)

	if existingOrder != nil {
		// The order already exists. Do NOT create a new one.
		app.Logger.Info("Duplicate payment intent request. Reusing existing order.", "key", idempotencyKey, "order_id", existingOrder.ID)
		orderID = existingOrder.ID

		// Optional: You could verify if cart total matches existing order total.
		// If not, it's a more complex scenario (cart was changed). For now, we assume it's a simple retry.

	} else {
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

		newOrderID, err := app.Orders.Insert(order)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		orderID = newOrderID

		app.Logger.Info("New order.", "order_id", orderID)

		app.SessionManager.Remove(r.Context(), "cart")
		app.SessionManager.Remove(r.Context(), "idempotencyKey")
		http.Redirect(w, r, "/order/success", http.StatusSeeOther)
	}
}

func (app *Application) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Provider       string `json:"provider"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if payload.IdempotencyKey == "" {
		app.renderJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency key is missing."})
		return
	}

	// Check if an order with this key ALREADY EXISTS.
	existingOrder, err := app.Orders.GetByIdempotencyKey(payload.IdempotencyKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, r, err)
		return
	}

	var orderID int64
	cart := app.getCartFromSession(r)

	if existingOrder != nil {
		// The order already exists. Do NOT create a new one.
		app.Logger.Info("Duplicate payment intent request. Reusing existing order.", "key", payload.IdempotencyKey, "order_id", existingOrder.ID)
		orderID = existingOrder.ID

		// Optional: You could verify if cart total matches existing order total.
		// If not, it's a more complex scenario (cart was changed). For now, we assume it's a simple retry.

	} else {
		// The order does not exist. This is the first attempt.
		// Proceed with stock check and order creation.
		ok, message, err := app.verifyStock(cart)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if !ok {
			app.renderJSON(w, http.StatusConflict, map[string]string{"error": message})
			return
		}

		user := r.Context().Value(contextKeyUser).(*d.User)
		orderItems := []d.OrderItem{}
		for _, item := range cart.Items {
			orderItems = append(orderItems, d.OrderItem{
				ProductID: item.Product.ID,
				Quantity:  item.Quantity,
				Price:     item.FinalPrice, // The final price including variant modifiers
				VariantDescription: sql.NullString{
					String: item.VariantDescription, // The human-readable string like "Size: Large, Color: Blue"
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

		newOrderID, err := app.Orders.Insert(order)
		if err != nil {
			// This could be a race condition, but our DB constraint will catch it.
			app.serverError(w, r, err)
			return
		}
		orderID = newOrderID
	}
	// --- END FIX ---

	// By this point, we have a valid orderID, either new or existing.
	// We can now proceed to create the payment provider session.
	baseURL := "http://localhost:4000"

	if payload.Provider == "stripe" {
		lineItems := []*stripe.CheckoutSessionLineItemParams{}
		// Build line items from the cart...
		for _, item := range cart.Items {
			lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					// ...
					UnitAmount: stripe.Int64(item.FinalPrice),
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
			// Associate the Stripe session with OUR order ID
			ClientReferenceID: stripe.String(strconv.FormatInt(orderID, 10)),
		}

		s, err := session.New(params)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		app.renderJSON(w, http.StatusOK, map[string]string{"id": s.ID})
		return
	}
	// ... other providers ...

	http.Error(w, "Invalid provider", http.StatusBadRequest)
}

func (app *Application) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	app.SessionManager.Remove(r.Context(), "cart")
	app.SessionManager.Remove(r.Context(), "idempotencyKey") // CLEAR THE KEY
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
