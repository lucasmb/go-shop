package handlers

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-shop/internal/data"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// --- Cart Handlers ---
func (app *Application) ViewCart(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "cart.page.html", data)
}

func (app *Application) AddToCart(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	product, err := app.Products.Get(productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	var finalStock int = product.Stock
	var finalPrice int64 = product.Price
	var variantDescription string
	var selectedSKU *data.SKU

	hasVariants := product.VariantInfo.SKUs != nil && len(product.VariantInfo.SKUs) > 0

	if hasVariants {
		userSelections := make(map[string]string)
		for _, group := range product.VariantInfo.VariantGroups {
			value := r.PostForm.Get("variant_" + group.Name)
			if value != "" {
				userSelections[group.Name] = value
			}
		}

		// Find the SKU that matches user selections
		for _, sku := range product.VariantInfo.SKUs {
			match := true
			if len(sku.Attributes) != len(userSelections) {
				continue
			}
			for key, value := range userSelections {
				if sku.Attributes[key] != value {
					match = false
					break
				}
			}
			if match {
				selectedSKU = &sku
				break
			}
		}

		if selectedSKU != nil {
			finalStock = selectedSKU.Stock
			finalPrice = product.Price + selectedSKU.PriceModifier

			// Build description string
			var descriptions []string
			// Sort keys
			keys := make([]string, 0, len(selectedSKU.Attributes))
			for k := range selectedSKU.Attributes {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				descriptions = append(descriptions, fmt.Sprintf("%s: %s", k, selectedSKU.Attributes[k]))
			}
			variantDescription = strings.Join(descriptions, ", ")

		} else {
			finalStock = 0 // Invalid combination is out of stock
		}
	}

	// GET CURRENT QUANTITY IN CART
	cart := app.getCartFromSession(r)
	tempKey := fmt.Sprintf("%d-%s", product.ID, variantDescription)
	hash := sha1.Sum([]byte(tempKey))
	compositeID := hex.EncodeToString(hash[:])

	quantityInCart := 0
	if item, ok := cart.Items[compositeID]; ok {
		quantityInCart = item.Quantity
	}

	// THE STOCK CHECK
	if (quantityInCart + 1) > finalStock {
		// Create a map for the error details.
		toastData := map[string]string{
			"message": "Not enough stock for this item.",
			"type":    "error",
		}
		// Marshal it into a valid JSON string.
		detailJSON, err := json.Marshal(toastData)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		// Set the HX-Trigger header with the valid JSON.
		w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": %s}`, string(detailJSON)))

		// Return an appropriate status code without a body.
		// 422 Unprocessable Entity is a good choice for a validation failure.
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if (quantityInCart + 1) > finalStock {
		toastData := map[string]string{"message": "Not enough stock for this item.", "type": "error"}
		detailJSON, _ := json.Marshal(toastData)

		w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": %s}`, string(detailJSON)))
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	//addItem to Cart
	cart.AddItem(product, variantDescription, finalPrice, 1)
	app.SessionManager.Put(r.Context(), "cart", cart)

	// Respond with success OOB swaps
	// Prepare data and the event payload
	data := app.newTemplateData(r)
	toastData := map[string]string{
		"message": fmt.Sprintf("%s added to cart!", product.Name),
		"type":    "success",
	}
	// Marshal this into a JSON string
	detailJSON, err := json.Marshal(toastData)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	// Set headers
	// Set the HX-Trigger header with the JSON string
	w.Header().Set("Content-Type", "text/html")
	// Use %s and pass the JSON string directly
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": %s}`, string(detailJSON)))

	// Render ONLY the cart icon partial directly to the response writer
	err = app.TemplateCache["cart_icon.partial.html"].ExecuteTemplate(w, "cart_icon.partial.html", data)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	compositeID := r.PathValue("id")
	if compositeID == "" {
		http.NotFound(w, r)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	newQuantity, err := strconv.Atoi(r.PostForm.Get("quantity"))
	if err != nil {
		http.Error(w, "Invalid quantity", http.StatusBadRequest)
		return
	}

	cart := app.getCartFromSession(r)

	// Find the item in the cart
	item, ok := cart.Items[compositeID]
	if !ok {
		http.NotFound(w, r)
		return
	}

	// --- STOCK CHECK LOGIC ---
	// Fetch the full product data to get its variant info
	product, err := app.Products.Get(item.Product.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	finalStock := product.Stock
	if product.VariantInfo.SKUs != nil {
		// Find the SKU corresponding to this cart item's description
		for _, sku := range product.VariantInfo.SKUs {
			// Rebuild the description string for the SKU to compare
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
				break
			}
		}
	}

	if newQuantity > finalStock {
		toastData := map[string]string{
			"message": fmt.Sprintf("Only %d available for %s.", finalStock, item.Product.Name),
			"type":    "error",
		}
		detailJSON, _ := json.Marshal(toastData)

		w.Header().Set("HX-Trigger-After-Swap", fmt.Sprintf(`{"showToast": %s}`, string(detailJSON)))

		// Set the status code explicitly here. This is our ONE WriteHeader call.
		w.WriteHeader(http.StatusUnprocessableEntity)

		// Re-render the cart table with the OLD quantity.
		// We pass the cart as it was BEFORE the failed update.
		data := app.newTemplateData(r)
		// Note: newTemplateData gets the cart from the session, which hasn't been changed yet. This is correct.

		// The renderPartial call will write the body but NOT another header.
		app.render(w, r, http.StatusOK, "cart_update.partial.html", data)
		return
	}

	// If stock is OK, update the cart
	cart.UpdateItemByCompositeID(compositeID, newQuantity)
	app.SessionManager.Put(r.Context(), "cart", cart)

	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "cart_update.partial.html", data)
}

func (app *Application) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	// The ID from the URL is the string compositeID
	compositeID := r.PathValue("id")
	if compositeID == "" {
		http.NotFound(w, r)
		return
	}

	cart := app.getCartFromSession(r)
	cart.RemoveItemByCompositeID(compositeID)
	app.SessionManager.Put(r.Context(), "cart", cart)

	data := app.newTemplateData(r)
	// Render the combined partial that updates both the table and the icon
	app.render(w, r, http.StatusOK, "cart_update.partial.html", data)
}
