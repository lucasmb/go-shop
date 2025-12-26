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

func (app *Application) APIUploadImage(w http.ResponseWriter, r *http.Request) {
	// Set a max memory limit for the upload. This is crucial for security.
	const maxUploadSize = 10 * 1024 * 1024 // 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// The `ParseMultipartForm` method populates r.MultipartForm.
	// We need to call it before we can access the file.
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		// This error is often triggered if the file is too large.
		app.Logger.Error("failed to parse multipart form", "error", err)
		app.renderJSON(w, http.StatusBadRequest, map[string]string{"error": "The uploaded file is too big (max 10MB)."})
		return
	}

	// Use r.FormFile to get the file from the form. The key "image"
	// must match the key used in the FormData append in JavaScript.
	file, header, err := r.FormFile("image")
	if err != nil {
		app.Logger.Error("failed to get file from form", "error", err)
		app.renderJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid file upload. Could not find 'image' field."})
		return
	}
	defer file.Close()

	// The rest of the logic is correct: call the storage interface.
	url, err := app.Store.Upload(header)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Return the public URL in a JSON response
	app.renderJSON(w, http.StatusOK, map[string]string{"url": url})
}
