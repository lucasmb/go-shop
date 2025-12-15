package main

import (
	"fmt"
	"go-shop/internal/handlers"
	"go-shop/ui"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

type application struct {
	logger         *slog.Logger
	handlers       *handlers.Application
	sessionManager *scs.SessionManager
}

// App routes
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// filesystem object rooted in the 'static' directory of embedded files.
	staticFS, err := fs.Sub(ui.Files, "static")
	if err != nil {
		panic(err)
	}
	// file server that serves files from this embedded filesystem.
	fileServer := http.FileServer(http.FS(staticFS))

	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// Middleware chains
	dynamic := handlers.NewChain(app.handlers.PopulateUser)
	protected := dynamic.Append(app.handlers.RequireAuthentication)
	admin := protected.Append(app.handlers.RequireAdmin)

	// Route definitions
	mux.Handle("GET /{$}", dynamic.ThenFunc(app.handlers.Home))
	mux.Handle("GET /product/{id}", dynamic.ThenFunc(app.handlers.ProductDetail))
	mux.Handle("GET /register", dynamic.ThenFunc(app.handlers.ShowRegister))
	mux.Handle("POST /register", dynamic.ThenFunc(app.handlers.DoRegister))
	mux.Handle("GET /login", dynamic.ThenFunc(app.handlers.ShowLogin))
	mux.Handle("POST /login", dynamic.ThenFunc(app.handlers.DoLogin))
	mux.Handle("POST /logout", protected.ThenFunc(app.handlers.DoLogout))
	mux.Handle("GET /cart", dynamic.ThenFunc(app.handlers.ViewCart))
	mux.Handle("GET /my-orders", protected.ThenFunc(app.handlers.ShowMyOrders))
	mux.Handle("GET /my-orders/{id}", protected.ThenFunc(app.handlers.ShowOrderDetail))
	mux.Handle("GET /checkout", protected.ThenFunc(app.handlers.ShowCheckout))
	mux.Handle("GET /order/success", protected.ThenFunc(app.handlers.OrderSuccess))
	mux.Handle("POST /cart/add/{id}", dynamic.ThenFunc(app.handlers.AddToCart))
	mux.Handle("POST /cart/update/{id}", dynamic.ThenFunc(app.handlers.UpdateCartItem))
	mux.Handle("DELETE /cart/item/{id}", dynamic.ThenFunc(app.handlers.RemoveFromCart))
	mux.Handle("POST /checkout/create-payment-intent", protected.ThenFunc(app.handlers.CreatePaymentIntent))
	mux.Handle("POST /checkout/bank-deposit", protected.ThenFunc(app.handlers.CreateBankDepositOrder))

	mux.Handle("GET /admin", admin.ThenFunc(app.handlers.AdminDashboard)) // Dashboard home
	mux.Handle("GET /admin/orders", admin.ThenFunc(app.handlers.AdminShowOrders))
	mux.Handle("POST /admin/order/update-status/{id}", admin.ThenFunc(app.handlers.AdminUpdateOrder))

	// Product CRUD Routes
	mux.Handle("GET /admin/products", admin.ThenFunc(app.handlers.AdminListProducts))
	mux.Handle("GET /admin/products/new", admin.ThenFunc(app.handlers.AdminNewProductForm))
	mux.Handle("POST /admin/products/new", admin.ThenFunc(app.handlers.AdminCreateProduct))
	mux.Handle("GET /admin/products/edit/{id}", admin.ThenFunc(app.handlers.AdminEditProductForm))
	mux.Handle("POST /admin/products/edit/{id}", admin.ThenFunc(app.handlers.AdminUpdateProduct))
	mux.Handle("DELETE /admin/products/{id}", admin.ThenFunc(app.handlers.AdminDeleteProduct))

	//  API endpoint for chart data
	mux.Handle("GET /admin/api/sales-report", admin.ThenFunc(app.handlers.APISalesReport))
	mux.Handle("GET /admin/api/product-sales-report", admin.ThenFunc(app.handlers.APIProductSalesReport))

	// The final handler is wrapped in the session manager middleware
	return app.sessionManager.LoadAndSave(mux)
}

// We can also move the server startup logic into a method for clarity.
func (app *application) serve() error {
	srv := &http.Server{
		Addr:     ":4000", // This should come from config
		Handler:  app.routes(),
		ErrorLog: slog.NewLogLogger(app.logger.Handler(), slog.LevelError)}

	app.logger.Info(fmt.Sprintf("Starting server on %s", srv.Addr))
	return srv.ListenAndServe()
}
