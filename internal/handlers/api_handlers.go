package handlers

import (
	"encoding/json"
	"net/http"
)

// This handler serves JSON data specifically for the daily sales chart.
func (app *Application) APISalesReport(w http.ResponseWriter, r *http.Request) {
	report, err := app.Orders.GetDailySales()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	js, err := json.Marshal(report)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// APIProductSalesReport serves JSON data for the top-selling products chart.
func (app *Application) APIProductSalesReport(w http.ResponseWriter, r *http.Request) {
	report, err := app.Orders.GetProductSales()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	js, err := json.Marshal(report)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
