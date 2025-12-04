package handlers

import (
	"net/http"
	"strconv"
)

func (app *Application) AdminShowOrders(w http.ResponseWriter, r *http.Request) {
	pendingOrders, err := app.Orders.GetAllPending()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Orders = pendingOrders
	app.render(w, r, http.StatusOK, "admin_orders.page.html", data)
}

func (app *Application) AdminUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
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

	err = app.Orders.UpdateStatusAndTracking(id, status, "")
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// successful HTMX POST from the admin page should redirect back
	w.Header().Set("HX-Redirect", "/admin/orders")
}
