package handlers

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"go-shop/internal/data"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/scs/v2"
)

type Application struct {
	Logger         *slog.Logger
	Products       *data.ProductModel
	Users          *data.UserModel
	Orders         *data.OrderModel
	Categories     *data.CategoryModel
	TemplateCache  map[string]*template.Template
	SessionManager *scs.SessionManager
	StripePubKey   string
}

type TemplateData struct {
	Cart           *data.Cart
	Products       []*data.Product
	Product        *data.Product
	InitialStock   int
	Order          *data.Order
	IdempotencyKey string
	CurrentPage    int
	TotalPages     int
	Filters        *data.SearchFilters
	Categories     []*data.CategoryFilter
	AllCategories  []*data.Category
	User           *data.User
	Orders         []*data.Order
	Form           any
	StripePubKey   string
	Flash          string
	Type           string
}

// helper to populate data common to all templates.
func (app *Application) newTemplateData(r *http.Request) *TemplateData {
	data2 := &TemplateData{
		Flash:        app.SessionManager.PopString(r.Context(), "flash"),
		Cart:         app.getCartFromSession(r),
		StripePubKey: app.StripePubKey,
	}

	if user, ok := r.Context().Value(contextKeyUser).(*data.User); ok {
		data2.User = user
	}
	return data2
}

// helper for rendering templates.
func (app *Application) render(w http.ResponseWriter, r *http.Request, status int, page string, tplData *TemplateData) {
	ts, ok := app.TemplateCache[page]
	if !ok {
		app.serverError(w, r, fmt.Errorf("the template %s does not exist", page))
		return
	}

	buf := new(bytes.Buffer)
	// Determine which layout to use.
	layout := "base"
	if strings.HasPrefix(page, "admin_") {
		layout = "admin"
	}
	err := ts.ExecuteTemplate(buf, layout, tplData)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.WriteHeader(status)
	buf.WriteTo(w)
}

// renderPartial renders only a specific block of a template, for HTMX swaps.
func (app *Application) renderPartial(w http.ResponseWriter, r *http.Request, status int, page, block string, tplData *TemplateData) {
	ts, ok := app.TemplateCache[page]
	if !ok {
		app.serverError(w, r, fmt.Errorf("the template %s does not exist", page))
		return
	}

	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, block, tplData)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.WriteHeader(status)
	buf.WriteTo(w)
}

// serverError logs the detailed error and sends a generic 500 response.
func (app *Application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.Logger.Error(err.Error(), "method", method, "uri", uri)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// getCartFromSession retrieves the cart, creating one if it doesn't exist.
func (app *Application) getCartFromSession(r *http.Request) *data.Cart {
	cart, ok := app.SessionManager.Get(r.Context(), "cart").(*data.Cart)
	if !ok || cart == nil {
		return data.NewCart()
	}
	return cart
}

// --- Page Handlers ---
func (app *Application) OrderCancel(w http.ResponseWriter, r *http.Request) {
	data := &TemplateData{
		Cart: app.getCartFromSession(r),
	}
	app.render(w, r, http.StatusOK, "cancel.page.html", data)
}

// ShowMyOrders displays the current user's order history.
func (app *Application) ShowMyOrders(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(contextKeyUser).(*data.User)

	orders, err := app.Orders.GetForUser(user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Orders = orders
	app.render(w, r, http.StatusOK, "my_orders.page.html", data)
}

func (app *Application) AdminUpdateOrder(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	status := r.PostForm.Get("status")
	trackingNumber := r.PostForm.Get("tracking_number")

	err = app.Orders.UpdateStatusAndTracking(id, status, trackingNumber)
	if err != nil {
		// Handle error, maybe return an error toast
		app.serverError(w, r, err)
		return
	}

	// Fetch the updated order to render the new row
	updatedOrder, err := app.Orders.GetByID(id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Order = updatedOrder
	// Render just the table row partial
	app.renderPartial(w, r, http.StatusOK, "admin_order_row.partial.html", "admin_order_row.partial.html", data)
}

func (app *Application) ShowOrderDetail(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	user := r.Context().Value(contextKeyUser).(*data.User)

	order, err := app.Orders.GetForUserAndID(orderID, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r) // User is trying to access an order that isn't theirs or doesn't exist
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	data := app.newTemplateData(r)
	data.Order = order
	app.render(w, r, http.StatusOK, "order_detail.page.html", data)
}
