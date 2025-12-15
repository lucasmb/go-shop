package handlers

import (
	"database/sql"
	"encoding/json"
	d "go-shop/internal/data"
	"net/http"
	"strconv"
)

// ProductForm holds data and validation errors for the product form.
type ProductForm struct {
	ID           int64
	Name         string
	Description  string
	CategoryID   sql.NullInt64
	ImageURL     string
	Price        float64
	Stock        int
	VariantsJSON string
	IsEdit       bool
	Errors       map[string]string
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

func (app *Application) AdminNewProductForm(w http.ResponseWriter, r *http.Request) {
	categories, err := app.Categories.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	data := app.newTemplateData(r)
	data.Form = ProductForm{Errors: make(map[string]string)}
	data.AllCategories = categories

	app.render(w, r, http.StatusOK, "admin_product_form.page.html", data)
}

func (app *Application) AdminCreateProduct(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	price, _ := strconv.ParseFloat(r.PostForm.Get("price"), 64)
	stock, _ := strconv.Atoi(r.PostForm.Get("stock"))
	catID, _ := strconv.ParseInt(r.PostForm.Get("category_id"), 10, 64)

	form := ProductForm{
		Name:         r.PostForm.Get("name"),
		Description:  r.PostForm.Get("description"),
		CategoryID:   sql.NullInt64{Int64: catID, Valid: catID > 0},
		ImageURL:     r.PostForm.Get("image_url"),
		Price:        price,
		Stock:        stock,
		VariantsJSON: r.PostForm.Get("variants_json"),
		Errors:       make(map[string]string),
	}

	// Validation
	if form.Name == "" {
		form.Errors["Name"] = "Product name cannot be empty."
	}
	if form.Price <= 0 {
		form.Errors["Price"] = "Price must be a positive number."
	}
	if form.VariantsJSON != "" {
		var js json.RawMessage
		if json.Unmarshal([]byte(form.VariantsJSON), &js) != nil {
			form.Errors["VariantsJSON"] = "Invalid JSON format."
		}
	}

	if len(form.Errors) > 0 {
		// 1. Set the HX-Reswap header to tell HTMX to perform the swap despite the error code.
		//    The value "outerHTML" should match the hx-swap on the form.
		w.Header().Set("HX-Reswap", "outerHTML")

		categories, err := app.Categories.GetAll()
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		data := app.newTemplateData(r)
		data.Form = form
		data.AllCategories = categories
		// Use the standard 'render' function, not 'renderPartial'.
		// Send a 422 status code so HTMX knows it was a validation error.
		app.render(w, r, http.StatusUnprocessableEntity, "admin_product_form.page.html", data)
		return
	}

	product := &d.Product{
		Name:         form.Name,
		Description:  form.Description,
		CategoryID:   form.CategoryID,
		ImageURL:     form.ImageURL,
		Price:        int64(form.Price * 100),
		Stock:        form.Stock,
		VariantsJSON: sql.NullString{String: form.VariantsJSON, Valid: form.VariantsJSON != ""},
	}

	_, err = app.Products.Insert(product)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/admin/products")
}

func (app *Application) AdminEditProductForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	product, err := app.Products.Get(id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	categories, err := app.Categories.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	form := ProductForm{
		ID:           product.ID,
		Name:         product.Name,
		Description:  product.Description,
		CategoryID:   product.CategoryID,
		ImageURL:     product.ImageURL,
		Price:        float64(product.Price) / 100.0,
		Stock:        product.Stock,
		VariantsJSON: product.VariantsJSON.String,
		IsEdit:       true,
		Errors:       make(map[string]string),
	}

	data := app.newTemplateData(r)
	data.Form = form
	data.AllCategories = categories
	app.render(w, r, http.StatusOK, "admin_product_form.page.html", data)

	if len(form.Errors) > 0 {
		// For now, to stop the errors, we'll just redirect back.
		// This is NOT ideal UX, but it stops the HTMX error handling complexity.
		app.SessionManager.Put(r.Context(), "flash", "There were errors with your submission.")
		w.Header().Set("HX-Redirect", r.Header.Get("HX-Current-URL"))
		return
	}
}

func (app *Application) AdminUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	price, _ := strconv.ParseFloat(r.PostForm.Get("price"), 64)
	stock, _ := strconv.Atoi(r.PostForm.Get("stock"))
	catID, _ := strconv.ParseInt(r.PostForm.Get("category_id"), 10, 64)

	form := ProductForm{
		ID:           id,
		Name:         r.PostForm.Get("name"),
		Description:  r.PostForm.Get("description"),
		CategoryID:   sql.NullInt64{Int64: catID, Valid: catID > 0},
		ImageURL:     r.PostForm.Get("image_url"),
		Price:        price,
		Stock:        stock,
		VariantsJSON: r.PostForm.Get("variants_json"),
		IsEdit:       true,
		Errors:       make(map[string]string),
	}

	// Validation (same as create)
	if form.Name == "" {
		form.Errors["Name"] = "Product name cannot be empty."
	}
	if form.Price <= 0 {
		form.Errors["Price"] = "Price must be a positive number."
	}
	if form.VariantsJSON != "" {
		var js json.RawMessage
		if json.Unmarshal([]byte(form.VariantsJSON), &js) != nil {
			form.Errors["VariantsJSON"] = "Invalid JSON format."
		}
	}

	if len(form.Errors) > 0 {
		data := app.newTemplateData(r)
		data.Form = form
		app.render(w, r, http.StatusUnprocessableEntity, "admin_product_form.page.html", data)
		return
	}

	product := &d.Product{
		ID:           form.ID,
		Name:         form.Name,
		Description:  form.Description,
		CategoryID:   form.CategoryID,
		ImageURL:     form.ImageURL,
		Price:        int64(form.Price * 100),
		Stock:        form.Stock,
		VariantsJSON: sql.NullString{String: form.VariantsJSON, Valid: form.VariantsJSON != ""},
	}

	err = app.Products.Update(product)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.SessionManager.Put(r.Context(), "flash", "Product updated successfully!")
	w.Header().Set("HX-Redirect", "/admin/products")
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
