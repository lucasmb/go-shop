package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	d "go-shop/internal/data"
	"net/http"
	"strconv"
	"strings"
)

// ProductForm holds data and validation errors for the product form.
type ProductForm struct {
	ID           int64
	Name         string
	Description  string
	CategoryID   sql.NullInt64
	ImageURL     string // This can be removed, but is harmless for now.
	ImagesJSON   string
	Price        float64
	Stock        int
	VariantsJSON string
	IsEdit       bool
	Errors       map[string]string
}

type VariantGroupForm struct {
	Name    string
	Options string // A comma-separated string
}

func (app *Application) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	app.render(w, r, http.StatusOK, "admin_dashboard.page.html", data)
}

func (app *Application) AdminShowOrders(w http.ResponseWriter, r *http.Request) {
	//fetch all orders. this would have pagination.
	orders, err := app.Orders.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Orders = orders
	app.render(w, r, http.StatusOK, "admin_orders.page.html", data)
}

func (app *Application) AdminListProducts(w http.ResponseWriter, r *http.Request) {
	products, _, err := app.Products.Search(d.SearchFilters{Page: 1, PageSize: 1000, Sort: "date_desc"})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	data := app.newTemplateData(r)
	data.Products = products
	app.render(w, r, http.StatusOK, "admin_products.page.html", data)
}

// AdminProductCreate handles both showing and processing the create form.
func (app *Application) AdminProductCreate(w http.ResponseWriter, r *http.Request) {
	// We'll need the category list for both GET and POST (on error).
	categories, err := app.Categories.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// --- HANDLE SHOWING AN EMPTY FORM ---
		// We pass a nil product because we are creating a new one.
		data := app.prepareProductFormData(r, nil, categories, nil)
		app.render(w, r, http.StatusOK, "admin_product_form.page.html", data)

	case http.MethodPost:
		// --- HANDLE PROCESSING THE FORM ---
		err := r.ParseForm()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		// Use the same parser, but pass 0 for ID and isEdit=false.
		form := app.parseAndValidateProductForm(r, 0, false)

		if len(form.Errors) > 0 {
			// Validation failed. Re-render the form with errors.
			w.Header().Set("HX-Reswap", "outerHTML")
			// Pass a nil product and the failed form data.
			data := app.prepareProductFormData(r, nil, categories, form)
			app.render(w, r, http.StatusUnprocessableEntity, "admin_product_form.page.html", data)
			return
		}

		// Validation succeeded. Create the product.
		product := app.productFromForm(form)
		_, err = app.Products.Insert(product)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		app.SessionManager.Put(r.Context(), "flash", "Product created successfully!")
		w.Header().Set("HX-Redirect", "/admin/products")
	}
}

func (app *Application) AdminProductEdit(w http.ResponseWriter, r *http.Request) {

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// We'll need the category list for both GET and PUT (on error).
	categories, err := app.Categories.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// --- HANDLE SHOWING THE FORM ---
		product, err := app.Products.Get(id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		// Prepare the data to display. This is the SINGLE SOURCE OF TRUTH.
		data := app.prepareProductFormData(r, product, categories, nil)
		app.render(w, r, http.StatusOK, "admin_product_form.page.html", data)

	case http.MethodPut:
		// --- HANDLE PROCESSING THE FORM ---
		err := r.ParseForm()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		form := app.parseAndValidateProductForm(r, id, true) // Pass id and isEdit=true

		if len(form.Errors) > 0 {
			// Validation failed. Re-render the form with errors.
			w.Header().Set("HX-Reswap", "outerHTML")
			product, _ := app.Products.Get(id) // Get original product for context
			data := app.prepareProductFormData(r, product, categories, form)
			app.render(w, r, http.StatusUnprocessableEntity, "admin_product_form.page.html", data)
			return
		}

		// Validation succeeded. Update the product.
		product := app.productFromForm(form)
		err = app.Products.Update(product)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		app.SessionManager.Put(r.Context(), "flash", "Product updated successfully!")
		w.Header().Set("HX-Redirect", "/admin/products")
	}
}

func (app *Application) parseAndValidateProductForm(r *http.Request, id int64, isEdit bool) *ProductForm {
	price, _ := strconv.ParseFloat(r.PostForm.Get("price"), 64)
	stock, _ := strconv.Atoi(r.PostForm.Get("stock"))
	catID, _ := strconv.ParseInt(r.PostForm.Get("category_id"), 10, 64)

	// --- PARSE DYNAMIC VARIANT GROUPS ---
	var variantGroups []d.VariantDisplayGroup
	for i := 0; ; i++ {
		groupNameKey := fmt.Sprintf("group_%d_name", i)
		if !r.PostForm.Has(groupNameKey) {
			break // No more groups
		}
		groupName := r.PostForm.Get(groupNameKey)
		if groupName == "" {
			continue
		}

		optionsStr := r.PostForm.Get(fmt.Sprintf("group_%d_options", i))
		var options []string
		for _, opt := range strings.Split(optionsStr, ",") {
			trimmedOpt := strings.TrimSpace(opt)
			if trimmedOpt != "" {
				options = append(options, trimmedOpt)
			}
		}
		variantGroups = append(variantGroups, d.VariantDisplayGroup{Name: groupName, Options: options})
	}

	var variantInfo d.ProductVariantInfo
	if len(variantGroups) > 0 {
		// In a real app, you would generate SKUs here based on the combinations
		variantInfo = d.ProductVariantInfo{VariantGroups: variantGroups, SKUs: []d.SKU{}}
	}

	variantsJsonBytes, _ := json.Marshal(variantInfo)
	variantsJsonString := string(variantsJsonBytes)
	if variantsJsonString == "{}" || variantsJsonString == "{\"variant_groups\":null,\"skus\":null}" {
		variantsJsonString = ""
	}
	// --- END PARSE DYNAMIC VARIANT GROUPS ---

	form := &ProductForm{
		ID:           id,
		IsEdit:       isEdit,
		Name:         r.PostForm.Get("name"),
		Description:  r.PostForm.Get("description"),
		CategoryID:   sql.NullInt64{Int64: catID, Valid: catID > 0},
		ImagesJSON:   r.PostForm.Get("images_json"),
		Price:        price,
		Stock:        stock,
		VariantsJSON: variantsJsonString,
		Errors:       make(map[string]string),
	}

	// Validation
	if form.Name == "" {
		form.Errors["Name"] = "Product name cannot be empty."
	}
	if form.Price <= 0 {
		form.Errors["Price"] = "Price must be a positive number."
	}

	return form
}

func (app *Application) prepareProductFormData(r *http.Request, product *d.Product, categories []*d.Category, form *ProductForm) *TemplateData {
	if form == nil {
		if product != nil {
			// If no form is provided (e.g., on GET request), create one from the product data.
			var variantInfo d.ProductVariantInfo
			if product.VariantsJSON.Valid && product.VariantsJSON.String != "" {
				json.Unmarshal([]byte(product.VariantsJSON.String), &variantInfo)
			}

			form = &ProductForm{
				ID:           product.ID,
				Name:         product.Name,
				Description:  product.Description,
				CategoryID:   product.CategoryID,
				ImagesJSON:   product.ImagesJSON.String,
				Price:        float64(product.Price) / 100.0,
				Stock:        product.Stock,
				VariantsJSON: product.VariantsJSON.String,
				IsEdit:       true,
			}
		} else {
			// This is the GET /new path, create a blank form
			form = &ProductForm{IsEdit: false, Errors: make(map[string]string)}
		}
	}

	data := app.newTemplateData(r)
	data.Form = form
	data.AllCategories = categories

	// Pre-process variants for display
	var variantInfo d.ProductVariantInfo
	if form.VariantsJSON != "" {
		json.Unmarshal([]byte(form.VariantsJSON), &variantInfo)
	}
	var formGroups []VariantGroupForm
	if variantInfo.VariantGroups != nil {
		for _, group := range variantInfo.VariantGroups {
			formGroups = append(formGroups, VariantGroupForm{
				Name:    group.Name,
				Options: strings.Join(group.Options, ", "),
			})
		}
	}
	data.VariantGroups = formGroups
	data.VariantInfo = variantInfo

	return data
}

// productFromForm converts a validated ProductForm to a *data.Product for saving.
func (app *Application) productFromForm(form *ProductForm) *d.Product {
	return &d.Product{
		ID:           form.ID,
		Name:         form.Name,
		Description:  form.Description,
		CategoryID:   form.CategoryID,
		ImagesJSON:   sql.NullString{String: form.ImagesJSON, Valid: form.ImagesJSON != ""},
		Price:        int64(form.Price * 100),
		Stock:        form.Stock,
		VariantsJSON: sql.NullString{String: form.VariantsJSON, Valid: form.VariantsJSON != ""},
	}
}

func (app *Application) AdminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = app.Products.Delete(id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	// This handler is called via HTMX, so we return an empty response
	// The frontend will remove the row from the table.
	w.WriteHeader(http.StatusOK)
}

func (app *Application) AdminAddVariantGroup(w http.ResponseWriter, r *http.Request) {
	// The 'nextIndex' will be sent by the button that was clicked.
	index, _ := strconv.Atoi(r.URL.Query().Get("nextIndex"))

	data := map[string]interface{}{
		"GroupIndex": index,
		"Group":      VariantGroupForm{Name: "", Options: ""}, // Pass the simple form struct
	}
	w.Header().Set("Content-Type", "text/html")
	err := app.renderBlock(w, "admin_variant_group_form.partial.html", data)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// AdminGenerateSKUs handles the HTMX request to generate SKU combinations.
func (app *Application) AdminGenerateSKUs(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 1. Parse the dynamically named variant groups from the form submission.
	var variantGroups []d.VariantDisplayGroup
	for i := 0; ; i++ {
		groupNameKey := fmt.Sprintf("group_%d_name", i)
		if !r.PostForm.Has(groupNameKey) {
			break // No more groups
		}
		groupName := r.PostForm.Get(groupNameKey)
		if groupName == "" {
			continue // Skip empty group names
		}

		optionsStr := r.PostForm.Get(fmt.Sprintf("group_%d_options", i))
		var options []string
		for _, opt := range strings.Split(optionsStr, ",") {
			trimmedOpt := strings.TrimSpace(opt)
			if trimmedOpt != "" {
				options = append(options, trimmedOpt)
			}
		}

		if len(options) > 0 {
			variantGroups = append(variantGroups, d.VariantDisplayGroup{Name: groupName, Options: options})
		}
	}

	// 2. Calculate the Cartesian product to get all combinations.
	var combinations []map[string]string
	if len(variantGroups) > 0 {
		// Start with an empty combination
		combos := []map[string]string{{}}
		for _, group := range variantGroups {
			newCombos := []map[string]string{}
			for _, combo := range combos {
				for _, option := range group.Options {
					newCombo := make(map[string]string)
					// Copy existing combo
					for k, v := range combo {
						newCombo[k] = v
					}
					// Add the new option
					newCombo[group.Name] = option
					newCombos = append(newCombos, newCombo)
				}
			}
			combos = newCombos
		}
		combinations = combos
	}

	// 3. Create a list of SKU structs from the combinations.
	// We'll also try to preserve data from any existing SKUs submitted in the form.
	var skus []d.SKU
	for _, combo := range combinations {
		skus = append(skus, d.SKU{Attributes: combo, PriceModifier: 0, Stock: 0, Image: ""})
	}

	// 4. Render the SKU table partial and return it.
	data := app.newTemplateData(r)
	data.SKUs = skus
	app.render(w, r, http.StatusOK, "admin_sku_table.partial.html", data)
}

// Helper to render a block without a layout
// This is the definitive helper for rendering HTMX fragments.
// It does NOT write a status header, leaving that to the caller.
func (app *Application) renderBlock(w http.ResponseWriter, name string, data any) error {
	ts, ok := app.TemplateCache[name]
	if !ok {
		return fmt.Errorf("the template %s does not exist", name)
	}

	// Execute the template directly to the writer.
	err := ts.ExecuteTemplate(w, name, data)
	if err != nil {
		return err
	}
	return nil
}
