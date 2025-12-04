package main

import (
	"fmt"
	"go-shop/internal/handlers"
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

	fileServer := http.FileServer(http.Dir("./ui/static/"))
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
	mux.Handle("GET /admin/orders", admin.ThenFunc(app.handlers.AdminShowOrders))
	mux.Handle("POST /admin/order/update-status/{id}", admin.ThenFunc(app.handlers.AdminUpdateOrder))

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
