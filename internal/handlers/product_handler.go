package handlers

import (
	"database/sql"
	"errors"
	"go-shop/internal/data"
	"net/http"
	"strconv"
)

func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	category := r.URL.Query().Get("category")
	minPrice, _ := strconv.Atoi(r.URL.Query().Get("min_price"))
	maxPrice, _ := strconv.Atoi(r.URL.Query().Get("max_price"))
	sort := r.URL.Query().Get("sort")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	filters := data.SearchFilters{
		Query:    query,
		Category: category,
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		Sort:     sort,
		Page:     page,
		PageSize: 15,
	}

	products, totalRecords, err := app.Products.Search(filters)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	// --- ADD THIS LOGGING ---
	app.Logger.Info(
		"Product search complete",
		"filter_query", filters.Query,
		"found_products", len(products),
		"total_records_in_db", totalRecords,
	)
	// --- END LOGGING ---

	// --- FETCH CATEGORIES ---
	categories, err := app.Categories.GetAllWithCounts()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	dataTemplate := app.newTemplateData(r)
	dataTemplate.Products = products
	dataTemplate.Categories = categories
	dataTemplate.CurrentPage = page
	dataTemplate.TotalPages = (totalRecords + filters.PageSize - 1) / filters.PageSize
	dataTemplate.Filters = &filters // Pass filters back to re-populate the form

	// --- HTMX Request Handling ---
	// If the request is from HTMX, only render the product list partial.
	if r.Header.Get("HX-Request") == "true" {
		app.renderPartial(w, r, http.StatusOK, "product_list.partial.html", "product_list.partial.html", dataTemplate)
		return
	}

	// For a full page load, render the entire home page.
	app.render(w, r, http.StatusOK, "home.page.html", dataTemplate)
}

// ProductDetail Handler
func (app *Application) ProductDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	product, err := app.Products.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// Pre-calculate the initial stock for the template.
	var initialStock int
	if product.VariantInfo.SKUs != nil && len(product.VariantInfo.SKUs) > 0 {
		// Default to the stock of the first SKU
		initialStock = product.VariantInfo.SKUs[0].Stock
	} else {
		// Fallback to the base product stock
		initialStock = product.Stock
	}

	data := app.newTemplateData(r)
	data.Product = product
	data.InitialStock = initialStock
	app.render(w, r, http.StatusOK, "product_detail.page.html", data)
}
